package zonecards

import (
	"os"
	"strings"
	"testing"

	"github.com/EQEmu/spire/internal/models"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ZONELOOT_TEST_DSN: a database with PEQ content and the Starfall
// custom_zone_card table (custom migration 68)
func testDB(t *testing.T) *gorm.DB {
	dsn := os.Getenv("ZONELOOT_TEST_DSN")
	if dsn == "" {
		t.Skip("ZONELOOT_TEST_DSN is not set")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestZoneCards(t *testing.T) {
	db := testDB(t)
	// the test makes and removes The Hole's row, and touches no other
	var held int64
	db.Raw("SELECT COUNT(*) FROM custom_zone_card WHERE zone_short_name = 'hole'").Scan(&held)
	if held > 0 {
		t.Skip("this database already has a card for hole")
	}
	t.Cleanup(func() { db.Exec("DELETE FROM custom_zone_card WHERE zone_short_name = 'hole'") })

	l, err := Read(db, db)
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Zones) < 400 {
		t.Fatalf("%d zones", len(l.Zones))
	}
	seen := map[string]bool{}
	for _, z := range l.Zones {
		if seen[z.ShortName] {
			t.Errorf("%v listed twice", z.ShortName)
		}
		seen[z.ShortName] = true
	}

	// a card for The Hole, like the in-game one
	card, err := Save(db, db, models.CustomZoneCard{ZoneShortName: "hole", Subtitle: " What Paineel Buried, It Did Not Forget ", Enabled: 1, ReturnCard: 1, Shroud: RowShroud})
	if err != nil {
		t.Fatal(err)
	}
	if card.Subtitle != "What Paineel Buried, It Did Not Forget" || card.Title != "" {
		t.Errorf("saved %+v", card)
	}
	// saved again: the row is replaced, not doubled
	if _, err := Save(db, db, models.CustomZoneCard{ZoneShortName: "hole", Title: "The Hole", Subtitle: "Below Paineel", Enabled: 0, ReturnCard: 1, Shroud: RowShroud}); err != nil {
		t.Fatal(err)
	}
	var rows []models.CustomZoneCard
	db.Where("zone_short_name = 'hole'").Find(&rows)
	if len(rows) != 1 || rows[0].Title != "The Hole" || rows[0].Enabled != 0 {
		t.Errorf("rows %+v", rows)
	}

	for what, c := range map[string]models.CustomZoneCard{
		"no such zone":      {ZoneShortName: "nozonehere", Enabled: 1},
		"title too long":    {ZoneShortName: "hole", Title: strings.Repeat("x", 64), Enabled: 1},
		"subtitle too long": {ZoneShortName: "hole", Subtitle: strings.Repeat("x", 128), Enabled: 1},
		"two lines":         {ZoneShortName: "hole", Subtitle: "a\nb", Enabled: 1},
		"bad name":          {ZoneShortName: "../x", Enabled: 1},
	} {
		if _, err := Save(db, db, c); err == nil {
			t.Errorf("%v: saved", what)
		}
	}
	if _, err := Save(db, db, models.CustomZoneCard{ZoneShortName: "hole", Title: strings.Repeat("x", 63), Subtitle: strings.Repeat("y", 127), Enabled: 1, ReturnCard: 1, Shroud: RowShroud}); err != nil {
		t.Errorf("at the limits: %v", err)
	}

	if hasColumn(db, "custom_zone_card", "shroud") {
		shroud(t, db)
	} else if _, err := Save(db, db, models.CustomZoneCard{ZoneShortName: "hole", Shroud: 60, Enabled: 1, ReturnCard: 1}); err == nil || !strings.Contains(err.Error(), "migration 70") {
		t.Errorf("a shroud before migration 70: %v", err)
	}
	if l, _ := Read(db, db); l.DefaultShroud != 45 {
		t.Errorf("default shroud %d", l.DefaultShroud)
	}
	if hasColumn(db, "custom_zone_card", "title_font") {
		styles(t, db)
	} else {
		if _, err := Save(db, db, models.CustomZoneCard{ZoneShortName: "hole", Reveal: "rise", Enabled: 1, ReturnCard: 1, Shroud: RowShroud}); err == nil || !strings.Contains(err.Error(), "migration 69") {
			t.Errorf("a style before migration 69: %v", err)
		}
		l, _ := Read(db, db)
		for _, c := range l.Cards {
			if c.ReturnCard != 1 {
				t.Errorf("before migration 69 %v reads return_card %d", c.ZoneShortName, c.ReturnCard)
			}
		}
		t.Log("before custom migration 69: styles not checked")
	}

	if err := Delete(db, "hole"); err != nil {
		t.Fatal(err)
	}
	l, _ = Read(db, db)
	for _, c := range l.Cards {
		if c.ZoneShortName == "hole" {
			t.Error("not deleted")
		}
	}
}

// after custom migration 69
func styles(t *testing.T, db *gorm.DB) {
	card, err := Save(db, db, models.CustomZoneCard{ZoneShortName: "hole", Title: "The Hole", TitleFont: "Pirata", TitleColor: "#e8e2d0",
		AccentColor: "7fa37a", EmblemIcon: 1234, Reveal: "letters", DisplayMs: 7000, ReturnCard: 0, Sound: "sounds/hole_sting.mp3", SoundOnReturn: 1, Enabled: 1,
		Shroud: RowShroud})
	if err != nil {
		t.Fatal(err)
	}
	var row models.CustomZoneCard
	db.Where("zone_short_name = 'hole'").First(&row)
	if !hasColumn(db, "custom_zone_card", "shroud") {
		row.Shroud = RowShroud // before migration 70: read as the default, as Read does
	}
	if row != *card || row.TitleFont != "pirata" || row.TitleColor != "E8E2D0" || row.AccentColor != "7FA37A" || row.Reveal != "letters" ||
		row.DisplayMs != 7000 || row.ReturnCard != 0 || row.SoundOnReturn != 1 || row.EmblemIcon != 1234 {
		t.Errorf("row %+v, saved %+v", row, card)
	}
	// the defaults are stored empty
	card, err = Save(db, db, models.CustomZoneCard{ZoneShortName: "hole", TitleFont: "cinzel", Reveal: "fade", Enabled: 1, ReturnCard: 1, Shroud: RowShroud})
	if err != nil || card.TitleFont != "" || card.Reveal != "" {
		t.Errorf("defaults: %+v %v", card, err)
	}
	for what, c := range map[string]models.CustomZoneCard{
		"font":       {ZoneShortName: "hole", TitleFont: "comic", Enabled: 1},
		"colour":     {ZoneShortName: "hole", TitleColor: "gold", Enabled: 1},
		"short hex":  {ZoneShortName: "hole", AccentColor: "FFF", Enabled: 1},
		"reveal":     {ZoneShortName: "hole", Reveal: "spin", Enabled: 1},
		"display":    {ZoneShortName: "hole", DisplayMs: 15001, Enabled: 1},
		"not an mp3": {ZoneShortName: "hole", Sound: "sting.wav", Enabled: 1},
		"up a level": {ZoneShortName: "hole", Sound: "../sting.mp3", Enabled: 1},
		"too long":   {ZoneShortName: "hole", Sound: strings.Repeat("a", 61) + ".mp3", Enabled: 1},
	} {
		if _, err := Save(db, db, c); err == nil {
			t.Errorf("%v: saved", what)
		}
	}
	l, _ := Read(db, db)
	if !l.Styles {
		t.Error("styles not seen")
	}
}

// after custom migration 70
func shroud(t *testing.T, db *gorm.DB) {
	card, err := Save(db, db, models.CustomZoneCard{ZoneShortName: "hole", Shroud: 55, ShroudColor: "#0c1a10", Enabled: 1, ReturnCard: 1})
	if err != nil {
		t.Fatal(err)
	}
	var row models.CustomZoneCard
	db.Where("zone_short_name = 'hole'").First(&row)
	if row.Shroud != 55 || row.ShroudColor != "0C1A10" || *card != row {
		t.Errorf("row %+v", row)
	}
	if _, err := Save(db, db, models.CustomZoneCard{ZoneShortName: "hole", Shroud: 0, Enabled: 1, ReturnCard: 1}); err != nil {
		t.Errorf("no shroud: %v", err)
	}
	db.Where("zone_short_name = 'hole'").First(&row)
	if row.Shroud != 0 {
		t.Errorf("shroud 0 saved as %d", row.Shroud)
	}
	for what, c := range map[string]models.CustomZoneCard{
		"too dark":   {ZoneShortName: "hole", Shroud: 91, Enabled: 1},
		"bad colour": {ZoneShortName: "hole", Shroud: 45, ShroudColor: "green", Enabled: 1},
	} {
		if _, err := Save(db, db, c); err == nil {
			t.Errorf("%v: saved", what)
		}
	}
	l, _ := Read(db, db)
	if !l.Shroud {
		t.Error("shroud columns not seen")
	}
	for _, c := range l.Cards {
		if c.ZoneShortName == "paineel" && (c.Shroud != 55 || c.ShroudColor != "0C1A10") {
			t.Errorf("paineel as seeded: %+v", c)
		}
	}
}

func TestBossCards(t *testing.T) {
	db := testDB(t)
	if l, _ := Read(db, db); !l.Rules.UntilInput || l.Rules.MaxMs != 60000 || l.Rules.HoldMs != 2000 {
		t.Errorf("zone card rules (defaults here): %+v", l.Rules)
	}
	if !hasTable(db, "custom_boss_card") {
		if _, err := ReadBosses(db, db); err == nil || !strings.Contains(err.Error(), "migration 74") {
			t.Errorf("before migration 74: %v", err)
		}
		t.Log("before custom migration 74: boss cards not checked")
		return
	}
	var held int64
	db.Raw("SELECT COUNT(*) FROM custom_boss_card WHERE npc_type_id = 101109").Scan(&held)
	if held > 0 {
		t.Skip("this database already has a card for King Gragnar")
	}
	t.Cleanup(func() { db.Exec("DELETE FROM custom_boss_card WHERE npc_type_id = 101109") })

	b, err := SaveBoss(db, db, models.CustomBossCard{NpcTypeID: 101109, Subtitle: " Who Would Not Stay Dead ", TitleFont: "Pirata",
		AccentColor: "#7fa37a", Sound: "boss_sting.mp3", Radius: 300, Shroud: 35, DisplayMs: 3500, RepeatSeconds: 3600, Enabled: 1})
	if err != nil {
		t.Fatal(err)
	}
	if b.Subtitle != "Who Would Not Stay Dead" || b.TitleFont != "pirata" || b.AccentColor != "7FA37A" || b.Reveal != "fade" || b.Trigger != "engage" {
		t.Errorf("saved %+v", b)
	}
	// again: replaced, the reserved-word column included
	if _, err := SaveBoss(db, db, models.CustomBossCard{NpcTypeID: 101109, Title: "The King Below", Trigger: "spawn", Reveal: "rise", Radius: 500, Shroud: 50, Enabled: 1}); err != nil {
		t.Fatal(err)
	}
	l, err := ReadBosses(db, db)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, x := range l.Bosses {
		if x.NpcTypeID == 101109 {
			found = true
			if x.Title != "The King Below" || x.Trigger != "spawn" || x.Radius != 500 || x.NPC.Name != "King_Gragnar" || !strings.Contains(x.NPC.Zones, "warrens") {
				t.Errorf("listed %+v", x)
			}
		}
	}
	if !found {
		t.Error("not listed")
	}
	for what, c := range map[string]models.CustomBossCard{
		"no npc":     {Radius: 300},
		"no such":    {NpcTypeID: 999999999, Radius: 300},
		"trigger":    {NpcTypeID: 101109, Trigger: "death", Radius: 300},
		"reveal":     {NpcTypeID: 101109, Reveal: "spin", Radius: 300},
		"radius":     {NpcTypeID: 101109, Radius: 0},
		"far":        {NpcTypeID: 101109, Radius: 5001},
		"shroud":     {NpcTypeID: 101109, Radius: 300, Shroud: 91},
		"display":    {NpcTypeID: 101109, Radius: 300, DisplayMs: 15001},
		"colour":     {NpcTypeID: 101109, Radius: 300, TitleColor: "red"},
		"sound":      {NpcTypeID: 101109, Radius: 300, Sound: "../x.mp3"},
		"long title": {NpcTypeID: 101109, Radius: 300, Title: strings.Repeat("x", 64)},
	} {
		if _, err := SaveBoss(db, db, c); err == nil {
			t.Errorf("%v: saved", what)
		}
	}
	bossCamera(t, db)
	hits, err := SearchNPCs(db, "king gragnar")
	if err != nil || len(hits) == 0 || hits[0].ID != 101109 {
		t.Errorf("search: %+v %v", hits, err)
	}
	if err := DeleteBoss(db, 101109); err != nil {
		t.Fatal(err)
	}
	if err := DeleteBoss(db, 101109); err == nil {
		t.Error("deleted twice")
	}
}

// the camera shot: custom migrations 75 (camera_ms), 76 (camera_mode) and 77
// (camera_style), each checked if the server has it and refused if not
func bossCamera(t *testing.T, db *gorm.DB) {
	cam := readBossCamera(db)
	base := models.CustomBossCard{NpcTypeID: 101109, Radius: 300, Enabled: 1}
	with := func(f func(b *models.CustomBossCard)) models.CustomBossCard { b := base; f(&b); return b }
	read := func() models.CustomBossCard {
		l, err := ReadBosses(db, db)
		if err != nil {
			t.Fatal(err)
		}
		for _, x := range l.Bosses {
			if x.NpcTypeID == 101109 {
				return x.CustomBossCard
			}
		}
		t.Fatal("not listed")
		return models.CustomBossCard{}
	}
	if cam.HideUiValue != -1 {
		t.Errorf("Custom:Boss_Snap_Hide_Ui_Value (unset here): %d", cam.HideUiValue)
	}
	// a card with no shot saves on any of them, whatever it says of the rest
	if _, err := SaveBoss(db, db, with(func(b *models.CustomBossCard) { b.CameraMode = "auto"; b.CameraStyle = "" })); err != nil {
		t.Fatal(err)
	}
	if !cam.Shot {
		if _, err := SaveBoss(db, db, with(func(b *models.CustomBossCard) { b.CameraMs = 3000; b.CameraMode = "keep"; b.CameraStyle = "live" })); err == nil || !strings.Contains(err.Error(), "migration 75") {
			t.Errorf("a shot before migration 75: %v", err)
		}
		t.Log("before custom migration 75: the camera shot not checked")
		return
	}
	for what, c := range map[string]models.CustomBossCard{
		"short": with(func(b *models.CustomBossCard) { b.CameraMs = 999 }),
		"long":  with(func(b *models.CustomBossCard) { b.CameraMs = 6001 }),
		"mode":  with(func(b *models.CustomBossCard) { b.CameraMs = 3000; b.CameraMode = "orbit" }),
		"num":   with(func(b *models.CustomBossCard) { b.CameraMs = 3000; b.CameraMode = "6" }),
		"style": with(func(b *models.CustomBossCard) { b.CameraMs = 3000; b.CameraStyle = "pip" }),
	} {
		if _, err := SaveBoss(db, db, c); err == nil {
			t.Errorf("camera %v: saved", what)
		}
	}
	if !cam.Mode {
		// migration 75's server: a live shot through the player's own mode
		if _, err := SaveBoss(db, db, with(func(b *models.CustomBossCard) { b.CameraMs = 3000; b.CameraMode = "far"; b.CameraStyle = "live" })); err == nil || !strings.Contains(err.Error(), "migration 76") {
			t.Errorf("a framing before migration 76: %v", err)
		}
		if _, err := SaveBoss(db, db, with(func(b *models.CustomBossCard) { b.CameraMs = 3000; b.CameraMode = "keep"; b.CameraStyle = "live" })); err != nil {
			t.Fatal(err)
		}
		if r := read(); r.CameraMs != 3000 || r.CameraMode != "keep" || r.CameraStyle != "live" {
			t.Errorf("before migration 76: %+v", r)
		}
		t.Log("before custom migration 76: framing not checked")
		return
	}
	if !cam.Style {
		if _, err := SaveBoss(db, db, with(func(b *models.CustomBossCard) { b.CameraMs = 3000; b.CameraStyle = "snap" })); err == nil || !strings.Contains(err.Error(), "migration 77") {
			t.Errorf("a portrait before migration 77: %v", err)
		}
		if _, err := SaveBoss(db, db, with(func(b *models.CustomBossCard) { b.CameraMs = 2500; b.CameraMode = "4"; b.CameraStyle = "live" })); err != nil {
			t.Fatal(err)
		}
		if r := read(); r.CameraMs != 2500 || r.CameraMode != "far" || r.CameraStyle != "live" {
			t.Errorf("before migration 77: %+v", r)
		}
		t.Log("before custom migration 77: portraits not checked")
		return
	}
	b, err := SaveBoss(db, db, with(func(b *models.CustomBossCard) { b.CameraMs = 4000; b.CameraMode = " Front "; b.CameraStyle = "" }))
	if err != nil {
		t.Fatal(err)
	}
	if b.CameraMode != "front" || b.CameraStyle != "snap" {
		t.Errorf("saved %+v", b)
	}
	if r := read(); r.CameraMs != 4000 || r.CameraMode != "front" || r.CameraStyle != "snap" {
		t.Errorf("read %+v", r)
	}
	if _, err := SaveBoss(db, db, with(func(b *models.CustomBossCard) { b.CameraMs = 1000; b.CameraMode = "0"; b.CameraStyle = "LIVE" })); err != nil {
		t.Fatal(err)
	}
	if r := read(); r.CameraMs != 1000 || r.CameraMode != "first" || r.CameraStyle != "live" {
		t.Errorf("read %+v", r)
	}
	// what the server reads a stray value as
	db.Exec("UPDATE custom_boss_card SET camera_mode = 'weird', camera_style = 'odd' WHERE npc_type_id = 101109")
	if r := read(); r.CameraMode != "" || r.CameraStyle != "snap" {
		t.Errorf("stray values read %+v", r)
	}
}
