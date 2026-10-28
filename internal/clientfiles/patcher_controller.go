package clientfiles

import (
	"io"
	"net/http"

	"github.com/EQEmu/spire/internal/http/routes"
	"github.com/labstack/echo/v4"
)

// PatcherController builds and releases the patcher program. It runs a
// script on this server, so it has its own permission (Patcher Build).
type PatcherController struct {
	patcher *Patcher
}

func NewPatcherController(patcher *Patcher) *PatcherController {
	return &PatcherController{patcher: patcher}
}

func (p *PatcherController) Routes() []*routes.Route {
	return []*routes.Route{
		routes.RegisterRoute(http.MethodGet, "patcher-build", p.status, nil),
		routes.RegisterRoute(http.MethodPut, "patcher-build/settings", p.settings, nil),
		routes.RegisterRoute(http.MethodPost, "patcher-build/build", p.build, nil),
		routes.RegisterRoute(http.MethodPost, "patcher-build/upload", p.upload, nil),
		routes.RegisterRoute(http.MethodPost, "patcher-build/release", p.release, nil),
		routes.RegisterRoute(http.MethodGet, "patcher-build/staged.exe", p.staged, nil),
	}
}

type patcherStatus struct {
	PatcherStatus
	SuggestedURLs    map[string]string `json:"suggested_urls"` // each channel's patch address, as this browser reaches Spire
	Dotnet           bool              `json:"dotnet"`
	DotnetPath       string            `json:"dotnet_path"`        // the dotnet builds use
	DotnetInstallDir string            `json:"dotnet_install_dir"` // where to install it
}

func (p *PatcherController) status(c echo.Context) error {
	dotnet := p.patcher.Dotnet()
	return c.JSON(http.StatusOK, patcherStatus{
		PatcherStatus:    p.patcher.Status(),
		SuggestedURLs:    map[string]string{ChannelLive: patcherURL(c, ChannelLive), ChannelBeta: patcherURL(c, ChannelBeta)},
		Dotnet:           dotnet != "",
		DotnetPath:       dotnet,
		DotnetInstallDir: p.patcher.DotnetInstallDir(),
	})
}

// the channel a request is for: ?channel=live or beta
func channelOf(c echo.Context) (string, error) {
	ch := c.QueryParam("channel")
	return ch, checkChannel(ch)
}

func (p *PatcherController) settings(c echo.Context) error {
	ch, err := channelOf(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	var s PatcherSettings
	if err := c.Bind(&s); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	if err := p.patcher.SaveSettings(ch, s); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	return p.status(c)
}

func (p *PatcherController) build(c echo.Context) error {
	ch, err := channelOf(c)
	if err == nil {
		err = p.patcher.Build(ch)
	}
	if err != nil {
		return c.JSON(http.StatusConflict, echo.Map{"error": err.Error()})
	}
	return p.status(c)
}

func (p *PatcherController) upload(c echo.Context) error {
	ch, err := channelOf(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
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
	exe, err := io.ReadAll(io.LimitReader(f, 64<<20))
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	if err := p.patcher.Upload(ch, exe); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	return p.status(c)
}

func (p *PatcherController) release(c echo.Context) error {
	ch, err := channelOf(c)
	if err == nil {
		_, err = p.patcher.Release(ch)
	}
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	return p.status(c)
}

// a channel's staged patcher, to try before players get it
func (p *PatcherController) staged(c echo.Context) error {
	ch, err := channelOf(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	st := p.patcher.Status().Channels[ch]
	if st.Staged == nil {
		return c.JSON(http.StatusNotFound, echo.Map{"error": "nothing is staged"})
	}
	return c.Attachment(p.patcher.stagedExe(ch), st.Staged.FileName+".exe")
}
