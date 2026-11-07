package zoneloot

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

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
		routes.RegisterRoute(http.MethodPost, "zone-loot/:zone/edit", z.edit, nil),
		routes.RegisterRoute(http.MethodGet, "zone-loot/:zone/items", z.items, nil),
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
	version, _ := strconv.Atoi(c.QueryParam("version"))
	out, err := Load(z.conn(c), c.Param("zone"), version)
	if err != nil {
		return bad(c, err)
	}
	return c.JSON(http.StatusOK, out)
}

type editRequest struct {
	Version int    `json:"version"`
	Edits   []Edit `json:"edits"`
	// preview: read the zone as the edits would leave it, changing nothing
	Preview bool `json:"preview"`
	// snapshot the zone first (the page asks for one before its first save)
	Snapshot bool   `json:"snapshot"`
	Note     string `json:"note"`
}

func (z *Controller) edit(c echo.Context) error {
	zone := c.Param("zone")
	if !zoneName.MatchString(zone) {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "not a zone short name"})
	}
	var req editRequest
	if err := c.Bind(&req); err != nil {
		return bad(c, err)
	}
	if len(req.Edits) == 0 {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "no changes"})
	}
	db := z.conn(c)
	var snap *Snapshot
	if req.Snapshot && !req.Preview {
		note := req.Note
		if note == "" {
			note = "before editing loot"
		}
		var err error
		if snap, err = z.snapshots().TakeAuto(db, zone, note); err != nil {
			return bad(c, fmt.Errorf("could not snapshot the zone first, so nothing was changed: %v", err))
		}
		snap.Tables = nil
	}
	res, err := Apply(db, zone, req.Version, req.Edits, req.Preview)
	if err != nil {
		return bad(c, err)
	}
	return c.JSON(http.StatusOK, echo.Map{"created": res.Created, "zone": res.Zone, "snapshot": snap})
}

// items finds items to add to a drop group, by name or id
func (z *Controller) items(c echo.Context) error {
	q := strings.TrimSpace(c.QueryParam("q"))
	out := []Item{}
	if q == "" {
		return c.JSON(http.StatusOK, out)
	}
	where, args := "Name LIKE ?", []any{"%" + q + "%"}
	if id, err := strconv.Atoi(q); err == nil {
		where, args = "id = ? OR Name LIKE ?", []any{id, "%" + q + "%"}
	}
	err := z.conn(c).Raw(`SELECT id, Name AS name, icon, itemtype AS item_type, slots, reqlevel AS req_level, reclevel AS rec_level, ac, hp, mana, damage, delay, classes,
		magic, nodrop = 0 AS no_drop, (lore LIKE '*%' OR loregroup <> 0) AS lore FROM items WHERE `+where+` ORDER BY Name LIKE ? DESC, LENGTH(Name), id LIMIT 25`,
		append(args, q+"%")...).Scan(&out).Error
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
