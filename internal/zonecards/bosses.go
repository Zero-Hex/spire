package zonecards

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/EQEmu/spire/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Boss title cards (starfall-3 zone/boss_cards.cpp, docs/design/boss_cards.md):
// a boss NPC type in custom_boss_card announces itself to the players near it
// with the zone-card look, on the first pull of each fight or on spawn. Its
// card is always timed: it never holds the keyboard or waits for input.

const (
	MaxRadius = 5000
	MaxNotes  = 255
	// the camera shot (custom migration 75): 0 for none, else the server
	// clamps it to this
	MinCameraMs = 1000
	MaxCameraMs = 6000
)

// cameraModes are the shot's framings (custom migration 76; boss_cards.cpp
// CameraMode): ” auto, keep, or a client camera mode by name; the server also
// takes the mode's number, stored here as its name
var cameraModes = []string{"first", "top", "close", "side", "far", "front"}

// bossColumns are the columns custom migration 74 made
var bossColumns = []string{"title", "subtitle", "trigger", "radius", "title_font", "title_color", "subtitle_color", "accent_color",
	"emblem_icon", "reveal", "display_ms", "shroud", "shroud_color", "sound", "repeat_seconds", "enabled", "notes"}

// laterBossColumns are the ones later migrations add, with their defaults
var laterBossColumns = []struct {
	name      string
	migration int
	isDefault func(b models.CustomBossCard) bool
}{
	{"camera_ms", 75, func(b models.CustomBossCard) bool { return b.CameraMs == 0 }},
	// before 76 a shot kept the player's own camera mode, and before 77 every
	// shot was live: a server without the column does that, which a card
	// with no shot doesn't care about
	{"camera_mode", 76, func(b models.CustomBossCard) bool { return b.CameraMs == 0 || b.CameraMode == "keep" }},
	{"camera_style", 77, func(b models.CustomBossCard) bool { return b.CameraMs == 0 || b.CameraStyle == "live" }},
}

// CameraMode is a framing as it is stored: ” (auto), keep or a mode name;
// ok is false for one the server would read as auto by mistake
func CameraMode(v string) (string, bool) {
	v = strings.ToLower(strings.TrimSpace(v))
	switch v {
	case "", "auto":
		return "", true
	case "keep":
		return v, true
	}
	for i, m := range cameraModes {
		if v == m || v == strconv.Itoa(i) {
			return m, true
		}
	}
	return "", false
}

// cameraStyle is how the server reads a style: anything but live is a portrait
func cameraStyle(v string) string {
	if strings.EqualFold(strings.TrimSpace(v), "live") {
		return "live"
	}
	return "snap"
}

var (
	triggers    = map[string]bool{"engage": true, "spawn": true}
	bossReveals = map[string]bool{"fade": true, "rise": true, "letters": true}
	errNoBoss   = errors.New("there is no custom_boss_card table: the server makes it (Starfall custom migration 74) when world starts on a build with boss cards")
)

// BossNPC is a boss's NPC, as the editor shows it
type BossNPC struct {
	ID    uint32 `json:"id"`
	Name  string `json:"name"`
	Level int    `json:"level"`
	Race  int    `json:"race"`
	// its look: the race model picture
	Gender      int `json:"gender"`
	Texture     int `json:"texture"`
	HelmTexture int `json:"helmtexture"`
	// its size: a portrait's camera distance and height scale with it
	Size  float64 `json:"size"`
	Zones string  `json:"zones"` // where it spawns
	Found bool    `json:"found"` // npc_types has it
}

type Boss struct {
	models.CustomBossCard
	NPC BossNPC `json:"npc"`
}

// PortraitRules are the server rules every boss portrait follows
// (common/ruletypes.h, Custom:Boss_*), editable here
type PortraitRules struct {
	// Custom:Boss_Snap_Hide_Ui_Value: the screen mode that hides the client's
	// UI while a portrait is captured (3 on RoF2; -1: left alone)
	HideUiValue int `json:"hide_ui_value"`
	// the portrait framing (a portrait's auto): the camera straight in front
	// of the boss at size x DistancePerSize (at least DistanceMin) and
	// size x HeightPerSize; Custom:Boss_Portrait_*
	DistancePerSize float64 `json:"distance_per_size"`
	DistanceMin     float64 `json:"distance_min"`
	HeightPerSize   float64 `json:"height_per_size"`
	// the portrait shows 1/CropZoom of the captured frame's width and height,
	// centred across it and at CropCenterY down it (0 top, 1 bottom)
	CropZoom    float64 `json:"crop_zoom"`
	CropCenterY float64 `json:"crop_center_y"`
}

// DefaultPortraitRules are ruletypes.h's defaults
var DefaultPortraitRules = PortraitRules{HideUiValue: 3, DistancePerSize: 2.0, DistanceMin: 10, HeightPerSize: 0.4, CropZoom: 2.2, CropCenterY: 0.70}

// portraitRuleNames are the rules, with each one's place in PortraitRules
// and its allowed range (the server clamps the camera to distance 1-500,
// height -200..200, the crop zoom to 1-8 and its centre to 0.01-0.99; the
// user camera's height can't go below 0)
var portraitRuleNames = []struct {
	name     string
	field    func(r *PortraitRules) *float64
	min, max float64
	note     string
}{
	{"Custom:Boss_Portrait_Distance_Per_Size", func(r *PortraitRules) *float64 { return &r.DistancePerSize }, 0.1, 25, "Starfall: boss portrait camera distance per point of size"},
	{"Custom:Boss_Portrait_Distance_Min", func(r *PortraitRules) *float64 { return &r.DistanceMin }, 1, 500, "Starfall: boss portrait shortest camera distance"},
	{"Custom:Boss_Portrait_Height_Per_Size", func(r *PortraitRules) *float64 { return &r.HeightPerSize }, 0, 10, "Starfall: boss portrait camera height per point of size"},
	{"Custom:Boss_Portrait_Crop_Zoom", func(r *PortraitRules) *float64 { return &r.CropZoom }, 1, 8, "Starfall: boss portrait crop zoom"},
	{"Custom:Boss_Portrait_Crop_Center_Y", func(r *PortraitRules) *float64 { return &r.CropCenterY }, 0.01, 0.99, "Starfall: boss portrait crop centre (0 top, 1 bottom)"},
}

const hideUiRule = "Custom:Boss_Snap_Hide_Ui_Value"

// BossCamera is which camera columns the server has, and the portrait rules
type BossCamera struct {
	Shot  bool `json:"shot"`  // camera_ms (custom migration 75)
	Mode  bool `json:"mode"`  // camera_mode (76)
	Style bool `json:"style"` // camera_style (77)
	PortraitRules
	// the server has a rule_values table to save them in
	RulesEditable bool `json:"rules_editable"`
}

type BossList struct {
	Bosses []Boss     `json:"bosses"`
	Camera BossCamera `json:"camera"`
}

func readPortraitRules(server *gorm.DB) PortraitRules {
	r := DefaultPortraitRules
	if v, ok := ruleValue(server, hideUiRule); ok {
		if n, err := strconv.Atoi(v); err == nil {
			r.HideUiValue = n
		}
	}
	for _, x := range portraitRuleNames {
		if v, ok := ruleValue(server, x.name); ok {
			if n, err := strconv.ParseFloat(v, 64); err == nil {
				*x.field(&r) = n
			}
		}
	}
	return r
}

func readBossCamera(server *gorm.DB) BossCamera {
	return BossCamera{Shot: hasColumn(server, "custom_boss_card", "camera_ms"), Mode: hasColumn(server, "custom_boss_card", "camera_mode"),
		Style: hasColumn(server, "custom_boss_card", "camera_style"), PortraitRules: readPortraitRules(server),
		RulesEditable: hasTable(server, "rule_values")}
}

// SavePortraitRules writes the rules that changed into ruleset 1 (a zone
// picks them up with #reload rules)
func SavePortraitRules(server *gorm.DB, r PortraitRules) (*PortraitRules, error) {
	if r.HideUiValue < -1 || r.HideUiValue > 254 {
		return nil, errors.New("the UI-hiding screen mode is from 0 to 254, or -1 to leave the UI alone")
	}
	for _, x := range portraitRuleNames {
		v := *x.field(&r)
		if math.IsNaN(v) || v < x.min || v > x.max {
			return nil, fmt.Errorf("%v is from %v to %v", strings.TrimPrefix(x.name, "Custom:"), x.min, x.max)
		}
	}
	if !hasTable(server, "rule_values") {
		return nil, errors.New("this server's database has no rule_values table")
	}
	old := readPortraitRules(server)
	set := func(name, value, note string) error {
		return server.Exec("INSERT INTO rule_values (ruleset_id, rule_name, rule_value, notes) VALUES (1, ?, ?, ?) "+
			"ON DUPLICATE KEY UPDATE rule_value = VALUES(rule_value)", name, value, note).Error
	}
	return &r, server.Transaction(func(tx *gorm.DB) error {
		if r.HideUiValue != old.HideUiValue {
			if err := set(hideUiRule, strconv.Itoa(r.HideUiValue), "Starfall: the screen mode that hides the client UI during boss portraits"); err != nil {
				return err
			}
		}
		for _, x := range portraitRuleNames {
			if v := *x.field(&r); v != *x.field(&old) {
				if err := set(x.name, strconv.FormatFloat(v, 'f', -1, 64), x.note); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// npcInfo is each NPC's name and where it spawns
func npcInfo(content *gorm.DB, ids []uint32) map[uint32]BossNPC {
	out := map[uint32]BossNPC{}
	if len(ids) == 0 {
		return out
	}
	var rows []BossNPC
	content.Raw(`SELECT n.id, n.name, n.level, n.race, n.gender, n.texture, n.helmtexture, n.size, COALESCE(GROUP_CONCAT(DISTINCT s.zone ORDER BY s.zone SEPARATOR ', '), '') AS zones
		FROM npc_types n LEFT JOIN spawnentry e ON e.npcID = n.id LEFT JOIN spawn2 s ON s.spawngroupID = e.spawngroupID
		WHERE n.id IN ? GROUP BY n.id`, ids).Scan(&rows)
	for _, r := range rows {
		r.Found = true
		out[r.ID] = r
	}
	return out
}

// ReadBosses lists the boss cards with their NPCs
func ReadBosses(content, server *gorm.DB) (*BossList, error) {
	if !hasTable(server, "custom_boss_card") {
		return nil, errNoBoss
	}
	var rows []models.CustomBossCard
	if err := server.Order("npc_type_id").Find(&rows).Error; err != nil {
		return nil, err
	}
	var ids []uint32
	for _, r := range rows {
		ids = append(ids, r.NpcTypeID)
	}
	info := npcInfo(content, ids)
	out := &BossList{Bosses: []Boss{}, Camera: readBossCamera(server)}
	for _, r := range rows {
		// as the server reads them, and what a server without migration 76 or
		// 77 does
		r.CameraMode, _ = CameraMode(r.CameraMode)
		r.CameraStyle = cameraStyle(r.CameraStyle)
		if !out.Camera.Mode {
			r.CameraMode = "keep"
		}
		if !out.Camera.Style {
			r.CameraStyle = "live"
		}
		n, ok := info[r.NpcTypeID]
		if !ok {
			n = BossNPC{ID: r.NpcTypeID}
		}
		out.Bosses = append(out.Bosses, Boss{CustomBossCard: r, NPC: n})
	}
	return out, nil
}

// SearchNPCs finds NPCs to give a card, by name or id
func SearchNPCs(content *gorm.DB, q string) ([]BossNPC, error) {
	q = strings.TrimSpace(q)
	out := []BossNPC{}
	if q == "" {
		return out, nil
	}
	var ids []uint32
	like := "%" + strings.ReplaceAll(q, " ", "_") + "%"
	if err := content.Raw(`SELECT id FROM npc_types WHERE id = ? OR name LIKE ?
		ORDER BY id = ? DESC, raid_target DESC, rare_spawn DESC, LENGTH(name), id LIMIT 25`, q, like, q).Scan(&ids).Error; err != nil {
		return nil, err
	}
	info := npcInfo(content, ids)
	for _, id := range ids {
		out = append(out, info[id])
	}
	return out, nil
}

// SaveBoss writes a boss's card
func SaveBoss(content, server *gorm.DB, b models.CustomBossCard) (*models.CustomBossCard, error) {
	b.Title = strings.TrimSpace(b.Title)
	b.Subtitle = strings.TrimSpace(b.Subtitle)
	b.TitleFont = strings.ToLower(strings.TrimSpace(b.TitleFont))
	b.Reveal = strings.ToLower(strings.TrimSpace(b.Reveal))
	b.Trigger = strings.ToLower(strings.TrimSpace(b.Trigger))
	b.Sound = strings.TrimSpace(b.Sound)
	b.Notes = strings.TrimSpace(b.Notes)
	if b.Reveal == "" {
		b.Reveal = "fade" // what the server reads an empty one as
	}
	if b.Trigger == "" {
		b.Trigger = "engage"
	}
	mode, modeOK := CameraMode(b.CameraMode)
	b.CameraMode = mode
	b.CameraStyle = strings.ToLower(strings.TrimSpace(b.CameraStyle))
	if b.CameraStyle == "" {
		b.CameraStyle = "snap"
	}
	for _, c := range []*string{&b.TitleColor, &b.SubtitleColor, &b.AccentColor, &b.ShroudColor} {
		*c = strings.ToUpper(strings.TrimPrefix(strings.TrimSpace(*c), "#"))
		if *c != "" && !hexColor.MatchString(*c) {
			return nil, fmt.Errorf("%q is not a colour: hex RRGGBB, or empty for the default", *c)
		}
	}
	switch {
	case b.NpcTypeID == 0:
		return nil, errors.New("pick the boss's NPC")
	case len(b.Title) > MaxTitle:
		return nil, fmt.Errorf("the title is %d bytes: players see at most %d", len(b.Title), MaxTitle)
	case len(b.Subtitle) > MaxSubtitle:
		return nil, fmt.Errorf("the epithet is %d bytes: players see at most %d", len(b.Subtitle), MaxSubtitle)
	case !utf8.ValidString(b.Title) || !utf8.ValidString(b.Subtitle) || strings.ContainsAny(b.Title+b.Subtitle, "\r\n\t"):
		return nil, errors.New("a card is one line of title and one of epithet")
	case !fonts[b.TitleFont]:
		return nil, fmt.Errorf("%q is not a font: cinzel, pirata or arimo", b.TitleFont)
	case !bossReveals[b.Reveal]:
		return nil, fmt.Errorf("%q is not a reveal: fade, rise or letters", b.Reveal)
	case !triggers[b.Trigger]:
		return nil, fmt.Errorf("%q is not a trigger: engage or spawn", b.Trigger)
	case b.DisplayMs > MaxDisplayMs:
		return nil, fmt.Errorf("a card stays up at most %d ms", MaxDisplayMs)
	case b.Shroud > MaxShroud:
		return nil, fmt.Errorf("the shroud dims the scene at most %d%%", MaxShroud)
	case b.Radius == 0 || b.Radius > MaxRadius:
		return nil, fmt.Errorf("the radius is from 1 to %d", MaxRadius)
	case b.Sound != "" && (len(b.Sound) > MaxSound || !soundPath.MatchString(b.Sound) || strings.Contains(b.Sound, "..")):
		return nil, fmt.Errorf("%q is not an MP3 in the client folder (at most %d characters)", b.Sound, MaxSound)
	case len(b.Notes) > MaxNotes:
		return nil, fmt.Errorf("the notes are at most %d characters", MaxNotes)
	case b.CameraMs != 0 && (b.CameraMs < MinCameraMs || b.CameraMs > MaxCameraMs):
		return nil, fmt.Errorf("a camera shot lasts %d to %d ms (0 for none)", MinCameraMs, MaxCameraMs)
	case !modeOK:
		return nil, fmt.Errorf("%q is not a camera framing: auto, keep, first, top, close, side, far or front", strings.TrimSpace(b.CameraMode))
	case b.CameraStyle != "snap" && b.CameraStyle != "live":
		return nil, fmt.Errorf("%q is not a camera style: snap (a portrait) or live", b.CameraStyle)
	}
	if b.TitleFont == "cinzel" {
		b.TitleFont = ""
	}
	if b.Enabled > 1 {
		b.Enabled = 1
	}
	var n int64
	content.Raw("SELECT COUNT(*) FROM npc_types WHERE id = ?", b.NpcTypeID).Scan(&n)
	if n == 0 {
		return nil, fmt.Errorf("there is no NPC %d", b.NpcTypeID)
	}
	if !hasTable(server, "custom_boss_card") {
		return nil, errNoBoss
	}
	// the columns this server has: one a later migration adds is written only
	// once it's there, and can't be set before
	cols := append([]string{}, bossColumns...)
	for _, c := range laterBossColumns {
		if hasColumn(server, "custom_boss_card", c.name) {
			cols = append(cols, c.name)
		} else if !c.isDefault(b) {
			return nil, fmt.Errorf("this server's custom_boss_card has no %v column yet (Starfall custom migration %d, run when world starts on a build that has it)", c.name, c.migration)
		}
	}
	if err := server.Select(append([]string{"npc_type_id"}, cols...)).
		Clauses(clause.OnConflict{DoUpdates: clause.AssignmentColumns(cols)}).Create(&b).Error; err != nil {
		return nil, err
	}
	return &b, nil
}

// DeleteBoss removes a boss's card
func DeleteBoss(server *gorm.DB, npcTypeID uint32) error {
	if !hasTable(server, "custom_boss_card") {
		return errNoBoss
	}
	res := server.Where("npc_type_id = ?", npcTypeID).Delete(&models.CustomBossCard{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("NPC %d has no boss card", npcTypeID)
	}
	return nil
}
