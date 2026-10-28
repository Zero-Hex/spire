package clientfiles

import (
	"io"
	"net/http"
	"os/exec"

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
	SuggestedURL string `json:"suggested_url"` // this Spire's patch address, as this browser reaches it
	Dotnet       bool   `json:"dotnet"`
}

func (p *PatcherController) status(c echo.Context) error {
	_, err := exec.LookPath("dotnet")
	return c.JSON(http.StatusOK, patcherStatus{PatcherStatus: p.patcher.Status(), SuggestedURL: patcherURL(c), Dotnet: err == nil})
}

func (p *PatcherController) settings(c echo.Context) error {
	var s PatcherSettings
	if err := c.Bind(&s); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	if err := p.patcher.SaveSettings(s); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	return p.status(c)
}

func (p *PatcherController) build(c echo.Context) error {
	if err := p.patcher.Build(); err != nil {
		return c.JSON(http.StatusConflict, echo.Map{"error": err.Error()})
	}
	return p.status(c)
}

func (p *PatcherController) upload(c echo.Context) error {
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
	if err := p.patcher.Upload(exe); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	return p.status(c)
}

func (p *PatcherController) release(c echo.Context) error {
	if _, err := p.patcher.Release(); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	return p.status(c)
}

// the staged patcher, to try before players get it
func (p *PatcherController) staged(c echo.Context) error {
	st := p.patcher.Status()
	if st.Staged == nil {
		return c.JSON(http.StatusNotFound, echo.Map{"error": "nothing is staged"})
	}
	return c.Attachment(p.patcher.stagedExe(), st.Staged.FileName+".exe")
}
