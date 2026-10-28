package clientfiles

import (
	"archive/zip"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/EQEmu/spire/internal/clientassets"
	"github.com/EQEmu/spire/internal/http/routes"
	"github.com/EQEmu/spire/internal/models"
	"github.com/labstack/echo/v4"
)

// the client patch: the files the RoF2 client needs from Spire, laid out as
// the client folder. Downloaded as one zip, or published for the patcher.

type patchContents struct {
	Files []clientassets.PatchFile `json:"files"`
	Notes []string                 `json:"notes"`
}

// the database files, as listed before they are built
func (f *Controller) databaseEntries(c echo.Context) []clientassets.PatchFile {
	count := func(model models.Modelable, table, what string) string {
		var n int64
		if err := f.db.Get(model, c).Model(model).Count(&n).Error; err != nil {
			return fmt.Sprintf("database: %v cannot be read (%v)", table, err)
		}
		return fmt.Sprintf("database: %d %s (%s)", n, what, table)
	}
	now := time.Now().Unix()
	out := []clientassets.PatchFile{
		{Path: "dbstr_us.txt", Source: count(&models.DbStr{}, "db_str", "strings"), Kind: "database", Updated: now},
		{Path: "spells_us.txt", Source: count(&models.SpellsNew{}, "spells_new", "spells"), Kind: "database", Updated: now},
	}
	if _, err := os.Stat(f.pathmgmt.GetExportClientFilesBinPath()); err == nil {
		out = append(out,
			clientassets.PatchFile{Path: "Resources/BaseData.txt", Source: "database (base_data)", Kind: "database", Updated: now},
			clientassets.PatchFile{Path: "Resources/SkillCaps.txt", Source: "database (skill_caps)", Kind: "database", Updated: now},
		)
	}
	return out
}

func (f *Controller) patchContents(c echo.Context) error {
	// as beta gets it: every file that goes to either channel
	files, notes := f.assets.PatchFiles(true)
	files = append(f.databaseEntries(c), files...)
	if _, err := os.Stat(f.pathmgmt.GetExportClientFilesBinPath()); err != nil {
		notes = append(notes, "Spire did not find the server's export_client_files, so spells_us.txt and dbstr_us.txt come from its own export and SkillCaps.txt and BaseData.txt are left out.")
	}
	if notes == nil {
		notes = []string{}
	}
	return c.JSON(http.StatusOK, patchContents{Files: files, Notes: notes})
}

// everything in a channel's patch, read as it is stored or published, and the
// files deleted from players' clients
func (f *Controller) patchSources(c echo.Context, ch string) ([]publishSource, []string, []string, error) {
	built, notes, err := f.databaseFiles(c)
	if err != nil {
		return nil, nil, notes, err
	}
	var out []publishSource
	for _, b := range built {
		b := b
		out = append(out, publishSource{Path: b.Path, Source: b.Source, read: func() ([]byte, error) { return b.Data, nil }})
	}
	var deletes []string
	assets, assetNotes := f.assets.PatchFiles(ch == ChannelBeta)
	for _, a := range assets {
		if a.Ignore || (a.Beta && ch != ChannelBeta) {
			continue // kept from these players: neither sent nor deleted
		}
		if a.Remove {
			deletes = append(deletes, a.Path)
			continue
		}
		s := publishSource{Path: a.Path, Source: a.Source, Beta: a.Beta, read: a.Read}
		if live, ok := a.LiveVariant(); ok {
			s.live = live
		}
		out = append(out, s)
	}
	return out, deletes, append(notes, assetNotes...), nil
}

// patch streams the zip of a channel's patch (?channel=, live by default)
func (f *Controller) patch(c echo.Context) error {
	ch := c.QueryParam("channel")
	if ch == "" {
		ch = ChannelLive
	}
	if err := checkChannel(ch); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	// built first: a failure here can still be an error page
	sources, deletes, notes, err := f.patchSources(c, ch)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}

	built := time.Now()
	name := "client-patch-" + built.Format("2006-01-02-1504") + ".zip"
	if ch == ChannelBeta {
		name = "client-patch-beta-" + built.Format("2006-01-02-1504") + ".zip"
	}
	res := c.Response()
	res.Header().Set(echo.HeaderContentType, "application/zip")
	res.Header().Set(echo.HeaderContentDisposition, fmt.Sprintf("attachment; filename=%q", name))
	res.WriteHeader(http.StatusOK)

	zw := zip.NewWriter(res)
	var listing []string
	add := func(path, source string, data []byte) {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: path, Method: zip.Deflate, Modified: built})
		if err == nil {
			_, err = w.Write(data)
		}
		if err != nil {
			notes = append(notes, fmt.Sprintf("%v could not be written: %v", path, err))
			return
		}
		listing = append(listing, fmt.Sprintf("  %-44s %s", path, source))
	}
	for _, s := range sources {
		data, err := s.read()
		if err != nil {
			notes = append(notes, fmt.Sprintf("%v left out: %v", s.Path, err))
			continue
		}
		add(s.Path, s.Source, data)
	}

	readme := []string{
		"Client patch (" + ch + ") built by Spire " + built.Format("2006-01-02 15:04 MST"),
		"",
		"Unzip into the RoF2 client folder (the one with eqgame.exe), replacing files.",
		"",
		"Files:",
	}
	readme = append(readme, listing...)
	if len(deletes) > 0 {
		readme = append(readme, "", "Delete these from the client folder if they are there:")
		for _, d := range deletes {
			readme = append(readme, "  "+d)
		}
	}
	if len(notes) > 0 {
		readme = append(readme, "", "Notes:")
		for _, n := range notes {
			readme = append(readme, "  - "+n)
		}
	}
	add("README-client-patch.txt", "this file", []byte(strings.Join(readme, "\r\n")+"\r\n"))
	return zw.Close()
}

// a channel's public address, which its patcher is built with (FILELIST_URL
// and PATCHER_URL): live at <api>/patch/, beta at <api>/patch/beta/
func patcherURL(c echo.Context, ch string) string {
	path := c.Request().URL.Path
	base := "/api/v1/"
	for _, group := range []string{"/client-files/", "/patcher-build"} {
		if i := strings.Index(path, group); i >= 0 {
			base = path[:i+1]
			break
		}
	}
	url := c.Scheme() + "://" + c.Request().Host + base + "patch/"
	if ch == ChannelBeta {
		url += "beta/"
	}
	return url
}

type publishedInfo struct {
	Manifest    *Manifest `json:"manifest"`
	FilelistURL string    `json:"filelist_url"`
}

func (f *Controller) published(c echo.Context) error {
	out := map[string]publishedInfo{}
	for _, ch := range []string{ChannelLive, ChannelBeta} {
		out[ch] = publishedInfo{Manifest: f.publisher.Current(ch), FilelistURL: patcherURL(c, ch)}
	}
	return c.JSON(http.StatusOK, out)
}

// publish publishes what Spire holds to a channel (?channel=live or beta)
func (f *Controller) publish(c echo.Context) error {
	ch := c.QueryParam("channel")
	if err := checkChannel(ch); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	sources, deletes, notes, err := f.patchSources(c, ch)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}
	m, diff, err := f.publisher.Publish(ch, sources, deletes, notes)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, echo.Map{"channel": ch, "manifest": m, "diff": diff, "filelist_url": patcherURL(c, ch)})
}

// promotePreview is what promoting beta would change on live
func (f *Controller) promotePreview(c echo.Context) error {
	diff, left, err := f.publisher.PromotePreview()
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, echo.Map{"diff": diff, "left_out": left, "beta": f.publisher.Current(ChannelBeta)})
}

// promote makes beta's published patch live, without its beta-only files
func (f *Controller) promote(c echo.Context) error {
	m, diff, err := f.publisher.Promote()
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, echo.Map{"channel": ChannelLive, "manifest": m, "diff": diff, "filelist_url": patcherURL(c, ChannelLive)})
}

// PublicController serves the published patches to players' patchers, with
// no login. Live is at <api>/patch/ and beta at <api>/patch/beta/; under each,
// rof/filelist_rof.yml and rof/<file>, and the released patcher the channel's
// patchers update themselves to: <name>-hash.txt and <name>.exe.
type PublicController struct {
	publisher *Publisher
	patcher   *Patcher
}

func NewPublicController(publisher *Publisher, patcher *Patcher) *PublicController {
	return &PublicController{publisher: publisher, patcher: patcher}
}

func (p *PublicController) Routes() []*routes.Route {
	return []*routes.Route{
		routes.RegisterRoute(http.MethodGet, "patch/*", p.serve, nil),
	}
}

func (p *PublicController) serve(c echo.Context) error {
	raw := c.Param("*")
	name, err := url.PathUnescape(raw)
	if err != nil {
		return c.String(http.StatusBadRequest, "bad path")
	}
	// the patcher asks for <url>rof/filelist_rof.yml, then <url>/filelist_rof.yml
	name = strings.TrimLeft(strings.ReplaceAll(name, "\\", "/"), "/")
	ch := ChannelLive
	if strings.HasPrefix(strings.ToLower(name), "beta/") {
		ch = ChannelBeta
		name = strings.TrimLeft(name[len("beta/"):], "/")
	}
	lower := strings.ToLower(name)

	if lower == "rof/filelist_rof.yml" || lower == "filelist_rof.yml" {
		m := p.publisher.Current(ch)
		if m == nil {
			return c.String(http.StatusNotFound, "nothing is published to "+ch+" yet")
		}
		prefix := strings.TrimSuffix(c.Request().URL.Path, raw)
		if !strings.HasSuffix(prefix, "/") {
			prefix += "/"
		}
		if ch == ChannelBeta {
			prefix += "beta/"
		}
		c.Response().Header().Set("Cache-Control", "no-cache")
		return c.Blob(http.StatusOK, "text/yaml; charset=utf-8", Filelist(m, c.Scheme()+"://"+c.Request().Host+prefix+"rof/"))
	}

	// the patcher asks for <url><name>-hash.txt, then <url>/<name>.exe
	if released, exe := p.patcher.Released(ch); released != nil {
		name := strings.ToLower(released.FileName)
		switch lower {
		case name + "-hash.txt":
			c.Response().Header().Set("Cache-Control", "no-cache")
			return c.String(http.StatusOK, released.MD5)
		case name + ".exe":
			c.Response().Header().Set("Cache-Control", "no-cache")
			return c.Attachment(exe, released.FileName+".exe")
		}
	}

	if !strings.HasPrefix(lower, "rof/") {
		return c.String(http.StatusNotFound, "not found")
	}
	file, blob, ok := p.publisher.Open(ch, name[len("rof/"):])
	if !ok {
		return c.String(http.StatusNotFound, "not in the published patch")
	}
	h := c.Response().Header()
	h.Set("Cache-Control", "no-cache")
	h.Set("ETag", `"`+file.MD5+`"`)
	h.Set(echo.HeaderContentType, "application/octet-stream")
	return c.File(blob)
}
