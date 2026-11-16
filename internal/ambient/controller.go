package ambient

import (
	"net/http"
	"strconv"

	"github.com/EQEmu/spire/internal/database"
	"github.com/EQEmu/spire/internal/http/routes"
	"github.com/EQEmu/spire/internal/models"
	"github.com/labstack/echo/v4"
)

// Controller edits the Starfall ambient lines. Its own permission (Ambient
// Lines): what NPCs say to players.
type Controller struct {
	db *database.Resolver
}

func NewController(db *database.Resolver) *Controller {
	return &Controller{db: db}
}

func (a *Controller) Routes() []*routes.Route {
	return []*routes.Route{
		routes.RegisterRoute(http.MethodGet, "ambient-lines", a.list, nil),
		routes.RegisterRoute(http.MethodPut, "ambient-lines", a.save, nil),
		routes.RegisterRoute(http.MethodDelete, "ambient-lines/:id", a.delete, nil),
	}
}

func bad(c echo.Context, err error) error {
	return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
}

func (a *Controller) list(c echo.Context) error {
	out, err := Read(a.db.Get(&models.Zone{}, c), a.db.Get(&models.CustomAmbientLine{}, c), c.QueryParam("zone"))
	if err != nil {
		return bad(c, err)
	}
	return c.JSON(http.StatusOK, out)
}

func (a *Controller) save(c echo.Context) error {
	var l models.CustomAmbientLine
	if err := c.Bind(&l); err != nil {
		return bad(c, err)
	}
	out, err := Save(a.db.Get(&models.Zone{}, c), a.db.Get(&models.CustomAmbientLine{}, c), l)
	if err != nil {
		return bad(c, err)
	}
	return c.JSON(http.StatusOK, out)
}

func (a *Controller) delete(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "not a line id"})
	}
	if err := Delete(a.db.Get(&models.CustomAmbientLine{}, c), uint32(id)); err != nil {
		return bad(c, err)
	}
	return c.JSON(http.StatusOK, echo.Map{"deleted": id})
}
