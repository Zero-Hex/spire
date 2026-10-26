package questfiles

import (
	"errors"
	"net/http"

	"github.com/EQEmu/spire/internal/http/routes"
	"github.com/labstack/echo/v4"
)

type Controller struct {
	service *Service
	checker *Checker
}

func NewController(service *Service, checker *Checker) *Controller {
	return &Controller{service: service, checker: checker}
}

func (q *Controller) Routes() []*routes.Route {
	return []*routes.Route{
		routes.RegisterRoute(http.MethodGet, "quest-files/list", q.list, nil),
		routes.RegisterRoute(http.MethodGet, "quest-files/file", q.read, nil),
		routes.RegisterRoute(http.MethodPut, "quest-files/file", q.save, nil),
		routes.RegisterRoute(http.MethodPost, "quest-files/check", q.check, nil),
		routes.RegisterRoute(http.MethodGet, "quest-files/backups", q.backups, nil),
		routes.RegisterRoute(http.MethodGet, "quest-files/backup", q.backup, nil),
		// loaded in a frame, which sends no auth header: use ?jwt=
		routes.RegisterRoute(http.MethodGet, "quest-files/window-forge", q.forge, nil),
	}
}

func fail(c echo.Context, err error) error {
	status := http.StatusBadRequest
	if errors.Is(err, ErrNotFound) {
		status = http.StatusNotFound
	} else if errors.Is(err, ErrConflict) {
		status = http.StatusConflict
	}
	return c.JSON(status, echo.Map{"error": err.Error()})
}

func (q *Controller) list(c echo.Context) error {
	l, err := q.service.List(c.QueryParam("dir"))
	if err != nil {
		return fail(c, err)
	}
	return c.JSON(http.StatusOK, l)
}

func (q *Controller) read(c echo.Context) error {
	s, err := q.service.Read(c.QueryParam("path"))
	if err != nil {
		return fail(c, err)
	}
	return c.JSON(http.StatusOK, s)
}

type saveRequest struct {
	Path     string `json:"path"`
	Content  string `json:"content"`
	Modified int64  `json:"modified,string"`
	Retire   string `json:"retire"`
}

func (q *Controller) save(c echo.Context) error {
	var r saveRequest
	if err := c.Bind(&r); err != nil {
		return fail(c, err)
	}
	s, err := q.service.Save(r.Path, r.Content, r.Modified, r.Retire)
	if err != nil {
		return fail(c, err)
	}
	return c.JSON(http.StatusOK, s)
}

type checkRequest struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// check looks for syntax errors and calls the Quest API does not have
func (q *Controller) check(c echo.Context) error {
	var r checkRequest
	if err := c.Bind(&r); err != nil {
		return fail(c, err)
	}
	if len(r.Content) > maxScriptBytes {
		return fail(c, errors.New("the script is too large to check"))
	}
	return c.JSON(http.StatusOK, q.checker.Check(r.Path, r.Content))
}

func (q *Controller) backups(c echo.Context) error {
	names, err := q.service.Backups(c.QueryParam("path"))
	if err != nil {
		return fail(c, err)
	}
	return c.JSON(http.StatusOK, echo.Map{"backups": names})
}

func (q *Controller) backup(c echo.Context) error {
	content, err := q.service.ReadBackup(c.QueryParam("path"), c.QueryParam("name"))
	if err != nil {
		return fail(c, err)
	}
	return c.JSON(http.StatusOK, echo.Map{"content": content})
}

func (q *Controller) forge(c echo.Context) error {
	html, err := q.service.ForgeHTML()
	if err != nil {
		return c.HTML(http.StatusNotFound, "<p style=\"font-family: sans-serif; color: #ccc\">"+err.Error()+"</p>")
	}
	return c.HTMLBlob(http.StatusOK, html)
}
