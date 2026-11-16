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
