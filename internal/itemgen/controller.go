package itemgen

import (
	"net/http"

	"github.com/EQEmu/spire/internal/database"
	"github.com/EQEmu/spire/internal/http/routes"
	"github.com/EQEmu/spire/internal/models"
	"github.com/labstack/echo/v4"
)

// Controller makes items from the item models and Heroes' Forge sets Spire
// holds. It writes items, so it has its own permission (Item Generator).
type Controller struct {
	db        *database.Resolver
	generator *Generator
}

func NewController(db *database.Resolver, generator *Generator) *Controller {
	return &Controller{db: db, generator: generator}
}

func (g *Controller) Routes() []*routes.Route {
	return []*routes.Route{
		routes.RegisterRoute(http.MethodPost, "item-gen/plan", g.plan, nil),
		routes.RegisterRoute(http.MethodPost, "item-gen/create", g.create, nil),
	}
}

func (g *Controller) plan(c echo.Context) error {
	var req PlanRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	plan, err := g.generator.Plan(g.db.Get(&models.Item{}, c), req)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, plan)
}

func (g *Controller) create(c echo.Context) error {
	var req CreateRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	created, err := g.generator.Create(g.db.Get(&models.Item{}, c), req)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, echo.Map{"created": created})
}
