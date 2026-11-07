package zoneloot

import (
	"net/http"
	"path/filepath"

	"github.com/EQEmu/spire/internal/database"
	"github.com/EQEmu/spire/internal/http/routes"
	"github.com/EQEmu/spire/internal/models"
	"github.com/EQEmu/spire/internal/pathmgmt"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// Controller shows a zone's loot and keeps snapshots of the rows it is made
// of. Restoring writes those rows back, so it has its own permission (Zone
// Loot Workbench).
type Controller struct {
	db       *database.Resolver
	pathmgmt *pathmgmt.PathManagement
}

func NewController(db *database.Resolver, pathmgmt *pathmgmt.PathManagement) *Controller {
	return &Controller{db: db, pathmgmt: pathmgmt}
}

func (z *Controller) Routes() []*routes.Route {
	return []*routes.Route{
		routes.RegisterRoute(http.MethodGet, "zone-loot/:zone", z.load, nil),
		routes.RegisterRoute(http.MethodGet, "zone-loot/:zone/snapshots", z.list, nil),
		routes.RegisterRoute(http.MethodPost, "zone-loot/:zone/snapshots", z.take, nil),
		routes.RegisterRoute(http.MethodGet, "zone-loot/:zone/snapshots/:id/diff", z.diff, nil),
		routes.RegisterRoute(http.MethodPost, "zone-loot/:zone/snapshots/:id/restore", z.restore, nil),
		routes.RegisterRoute(http.MethodDelete, "zone-loot/:zone/snapshots/:id", z.delete, nil),
	}
}

// the loot tables are content: the content connection, where one is set
func (z *Controller) conn(c echo.Context) *gorm.DB {
	return z.db.Get(&models.Spawn2{}, c)
}

func (z *Controller) snapshots() *Snapshots {
	dir := ""
	if base := z.pathmgmt.GetEQEmuServerPath(); base != "" {
		dir = filepath.Join(base, "spire-zone-snapshots")
	}
	return NewSnapshots(dir)
}

func bad(c echo.Context, err error) error {
	return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
}

func (z *Controller) load(c echo.Context) error {
	if !zoneName.MatchString(c.Param("zone")) {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "not a zone short name"})
	}
	out, err := Load(z.conn(c), c.Param("zone"))
	if err != nil {
		return bad(c, err)
	}
	return c.JSON(http.StatusOK, out)
}

func (z *Controller) list(c echo.Context) error {
	out, err := z.snapshots().List(c.Param("zone"))
	if err != nil {
		return bad(c, err)
	}
	return c.JSON(http.StatusOK, out)
}

func (z *Controller) take(c echo.Context) error {
	var req struct {
		Note string `json:"note"`
	}
	_ = c.Bind(&req)
	snap, err := z.snapshots().Take(z.conn(c), c.Param("zone"), req.Note)
	if err != nil {
		return bad(c, err)
	}
	snap.Tables = nil
	return c.JSON(http.StatusOK, snap)
}

func (z *Controller) diff(c echo.Context) error {
	snap, err := z.snapshots().Get(c.Param("zone"), c.Param("id"))
	if err != nil {
		return bad(c, err)
	}
	d, err := Diff(z.conn(c), snap)
	if err != nil {
		return bad(c, err)
	}
	return c.JSON(http.StatusOK, d)
}

func (z *Controller) restore(c echo.Context) error {
	before, d, err := z.snapshots().Restore(z.conn(c), c.Param("zone"), c.Param("id"))
	if err != nil {
		return bad(c, err)
	}
	return c.JSON(http.StatusOK, echo.Map{"before": before, "diff": d})
}

func (z *Controller) delete(c echo.Context) error {
	if err := z.snapshots().Delete(c.Param("zone"), c.Param("id")); err != nil {
		return bad(c, err)
	}
	return c.JSON(http.StatusOK, echo.Map{"deleted": c.Param("id")})
}
