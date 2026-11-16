package zonecards

import (
	"net/http"

	"github.com/EQEmu/spire/internal/database"
	"github.com/EQEmu/spire/internal/http/routes"
	"github.com/EQEmu/spire/internal/models"
	"github.com/labstack/echo/v4"
)

// Controller edits the Starfall zone title cards. Its own permission (Zone
// Cards): what players read first about every zone.
type Controller struct {
	db *database.Resolver
}

func NewController(db *database.Resolver) *Controller {
	return &Controller{db: db}
}

func (z *Controller) Routes() []*routes.Route {
	return []*routes.Route{
		routes.RegisterRoute(http.MethodGet, "zone-cards", z.list, nil),
		routes.RegisterRoute(http.MethodPut, "zone-cards/:zone", z.save, nil),
		routes.RegisterRoute(http.MethodDelete, "zone-cards/:zone", z.delete, nil),
	}
}

func bad(c echo.Context, err error) error {
	return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
}

func (z *Controller) list(c echo.Context) error {
	out, err := Read(z.db.Get(&models.Zone{}, c), z.db.Get(&models.CustomZoneCard{}, c))
	if err != nil {
		return bad(c, err)
	}
	return c.JSON(http.StatusOK, out)
}

func (z *Controller) save(c echo.Context) error {
	var card models.CustomZoneCard
	if err := c.Bind(&card); err != nil {
		return bad(c, err)
	}
	card.ZoneShortName = c.Param("zone")
	out, err := Save(z.db.Get(&models.Zone{}, c), z.db.Get(&models.CustomZoneCard{}, c), card)
	if err != nil {
		return bad(c, err)
	}
	return c.JSON(http.StatusOK, out)
}

func (z *Controller) delete(c echo.Context) error {
	if err := Delete(z.db.Get(&models.CustomZoneCard{}, c), c.Param("zone")); err != nil {
		return bad(c, err)
	}
	return c.JSON(http.StatusOK, echo.Map{"deleted": c.Param("zone")})
}
