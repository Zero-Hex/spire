package zonecards

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/EQEmu/spire/internal/clientassets"
	"github.com/EQEmu/spire/internal/database"
	"github.com/EQEmu/spire/internal/http/routes"
	"github.com/EQEmu/spire/internal/models"
	"github.com/labstack/echo/v4"
)

// Controller edits the Starfall zone title cards. Its own permission (Zone
// Cards): what players read first about every zone.
type Controller struct {
	db    *database.Resolver
	store *clientassets.Store
}

func NewController(db *database.Resolver, store *clientassets.Store) *Controller {
	return &Controller{db: db, store: store}
}

func (z *Controller) Routes() []*routes.Route {
	return []*routes.Route{
		routes.RegisterRoute(http.MethodGet, "zone-cards", z.list, nil),
		routes.RegisterRoute(http.MethodGet, "zone-cards/sound", z.sound, nil),
		routes.RegisterRoute(http.MethodGet, "zone-cards/bosses", z.bosses, nil),
		routes.RegisterRoute(http.MethodGet, "zone-cards/bosses/npcs", z.bossNPCs, nil),
		routes.RegisterRoute(http.MethodPut, "zone-cards/bosses/portrait-rules", z.savePortraitRules, nil),
		routes.RegisterRoute(http.MethodPut, "zone-cards/bosses/:npc", z.saveBoss, nil),
		routes.RegisterRoute(http.MethodDelete, "zone-cards/bosses/:npc", z.deleteBoss, nil),
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
	out.Sounds = Sounds(z.store)
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

// sound sends an MP3 Spire keeps for the client patch, to hear in the editor
func (z *Controller) sound(c echo.Context) error {
	path := c.QueryParam("path")
	if !strings.HasSuffix(strings.ToLower(path), ".mp3") {
		return c.JSON(http.StatusNotFound, echo.Map{"error": "not an MP3"})
	}
	b, ok := z.store.ClientFileData(strings.ReplaceAll(path, "\\", "/"))
	if !ok {
		return c.JSON(http.StatusNotFound, echo.Map{"error": "Spire keeps no " + path + " for the client patch"})
	}
	return c.Blob(http.StatusOK, "audio/mpeg", b)
}

// --- boss title cards ---

func (z *Controller) bosses(c echo.Context) error {
	out, err := ReadBosses(z.db.Get(&models.Zone{}, c), z.db.Get(&models.CustomBossCard{}, c))
	if err != nil {
		return bad(c, err)
	}
	return c.JSON(http.StatusOK, echo.Map{"bosses": out.Bosses, "camera": out.Camera, "sounds": Sounds(z.store)})
}

func (z *Controller) bossNPCs(c echo.Context) error {
	out, err := SearchNPCs(z.db.Get(&models.Zone{}, c), c.QueryParam("q"))
	if err != nil {
		return bad(c, err)
	}
	return c.JSON(http.StatusOK, out)
}

func (z *Controller) saveBoss(c echo.Context) error {
	var b models.CustomBossCard
	if err := c.Bind(&b); err != nil {
		return bad(c, err)
	}
	id, err := strconv.ParseUint(c.Param("npc"), 10, 32)
	if err != nil {
		return bad(c, err)
	}
	b.NpcTypeID = uint32(id)
	out, err := SaveBoss(z.db.Get(&models.Zone{}, c), z.db.Get(&models.CustomBossCard{}, c), b)
	if err != nil {
		return bad(c, err)
	}
	return c.JSON(http.StatusOK, out)
}

func (z *Controller) savePortraitRules(c echo.Context) error {
	var r PortraitRules
	if err := c.Bind(&r); err != nil {
		return bad(c, err)
	}
	out, err := SavePortraitRules(z.db.Get(&models.RuleValue{}, c), r)
	if err != nil {
		return bad(c, err)
	}
	return c.JSON(http.StatusOK, out)
}

func (z *Controller) deleteBoss(c echo.Context) error {
	id, err := strconv.ParseUint(c.Param("npc"), 10, 32)
	if err != nil {
		return bad(c, err)
	}
	if err := DeleteBoss(z.db.Get(&models.CustomBossCard{}, c), uint32(id)); err != nil {
		return bad(c, err)
	}
	return c.JSON(http.StatusOK, echo.Map{"deleted": id})
}
