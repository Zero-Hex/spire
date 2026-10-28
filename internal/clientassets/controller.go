package clientassets

import (
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/EQEmu/spire/internal/http/routes"
	"github.com/labstack/echo/v4"
)

// PublicController serves the imported icons, previews and their stylesheet.
// It needs no login: browsers load these through <link> and CSS, which send
// no token, the same as the stock asset pack.
type PublicController struct {
	store *Store
}

func NewPublicController(store *Store) *PublicController {
	return &PublicController{store: store}
}

func (p *PublicController) Routes() []*routes.Route {
	return []*routes.Route{
		routes.RegisterRoute(http.MethodGet, "client-assets/version", p.version, nil),
		routes.RegisterRoute(http.MethodGet, "client-assets/file/*", p.file, nil),
	}
}

// only these are served; archives are not public
var publicFile = regexp.MustCompile(`^(client-assets\.css|icons/item_\d+\.png|models/CTN_\d+\.png)$`)

func (p *PublicController) version(c echo.Context) error {
	return c.JSON(http.StatusOK, echo.Map{"version": p.store.Index().Version})
}

func (p *PublicController) file(c echo.Context) error {
	name := c.Param("*")
	if !publicFile.MatchString(name) {
		return c.JSON(http.StatusNotFound, echo.Map{"error": "not found"})
	}
	path := filepath.Join(p.store.Dir(), filepath.FromSlash(name))
	if name == "client-assets.css" {
		// regenerated on every import; empty until the first one
		c.Response().Header().Set("Cache-Control", "no-cache")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return c.Blob(http.StatusOK, "text/css", []byte{})
		}
	}
	return c.File(path)
}

// Controller imports and removes client assets
type Controller struct {
	store *Store
}

func NewController(store *Store) *Controller {
	return &Controller{store: store}
}

func (a *Controller) Routes() []*routes.Route {
	return []*routes.Route{
		routes.RegisterRoute(http.MethodGet, "client-assets", a.info, nil),
		routes.RegisterRoute(http.MethodPost, "client-assets/upload", a.upload, nil),
		routes.RegisterRoute(http.MethodGet, "client-assets/archive/:name", a.archive, nil),
		routes.RegisterRoute(http.MethodPost, "client-assets/model/:id/preview", a.preview, nil),
		routes.RegisterRoute(http.MethodDelete, "client-assets/sheet/:sheet", a.removeSheet, nil),
		routes.RegisterRoute(http.MethodDelete, "client-assets/archive/:name", a.removeArchive, nil),
		routes.RegisterRoute(http.MethodDelete, "client-assets/client-file/*", a.removeClientFile, nil),
	}
}

func (a *Controller) info(c echo.Context) error {
	return c.JSON(http.StatusOK, echo.Map{
		"drop_folder":    a.store.DropDir(),
		"rof2_last_icon": Rof2LastStockIcon,
		"index":          a.store.Index(),
	})
}

type uploadResult struct {
	File   string `json:"file"`
	Result string `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

// upload imports every file of a multipart form (field "files"). A "folder"
// puts them all in that folder of the client folder; "paths", one per file,
// give each its own (a folder upload: uifiles/default/EQUI.xml).
func (a *Controller) upload(c echo.Context) error {
	form, err := c.MultipartForm()
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	folder := ""
	if v := form.Value["folder"]; len(v) > 0 {
		folder = v[0]
	}
	paths := form.Value["paths"]

	var results []uploadResult
	for i, fh := range form.File["files"] {
		dir := folder
		if i < len(paths) && paths[i] != "" {
			dir = path.Dir(strings.ReplaceAll(paths[i], "\\", "/"))
			if dir == "." {
				dir = ""
			}
		}
		r := uploadResult{File: path.Join(dir, fh.Filename)}
		f, err := fh.Open()
		if err != nil {
			r.Error = err.Error()
		} else {
			r.Result, err = a.store.ImportReaderAt(dir, fh.Filename, f)
			_ = f.Close()
			if err != nil {
				r.Error = err.Error()
			}
		}
		results = append(results, r)
	}

	return c.JSON(http.StatusOK, echo.Map{"results": results, "index": a.store.Index()})
}

// archive sends an imported .eqg to the page that renders its previews
func (a *Controller) archive(c echo.Context) error {
	name := filepath.Base(c.Param("name"))
	ar := a.store.Index().Archives[strings.ToLower(name)]
	if ar == nil {
		return c.JSON(http.StatusNotFound, echo.Map{"error": "not imported"})
	}
	return c.File(filepath.Join(a.store.Dir(), "archives", ar.File))
}

// preview stores a model preview (multipart field "file", a PNG)
func (a *Controller) preview(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "bad model id"})
	}
	fh, err := c.FormFile("file")
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	f, err := fh.Open()
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	defer f.Close()

	data, err := io.ReadAll(io.LimitReader(f, 4<<20))
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	if err := a.store.SavePreview(id, data); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, echo.Map{"ok": true})
}

func (a *Controller) removeSheet(c echo.Context) error {
	n, err := strconv.Atoi(c.Param("sheet"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "bad sheet number"})
	}
	if err := a.store.RemoveSheet(n); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, echo.Map{"index": a.store.Index()})
}

func (a *Controller) removeArchive(c echo.Context) error {
	if err := a.store.RemoveArchive(c.Param("name")); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, echo.Map{"index": a.store.Index()})
}

func (a *Controller) removeClientFile(c echo.Context) error {
	name, err := url.PathUnescape(c.Param("*"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	if err := a.store.RemoveClientFile(name); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, echo.Map{"index": a.store.Index()})
}
