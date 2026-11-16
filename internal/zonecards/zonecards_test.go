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
	var before []models.CustomZoneCard
	db.Find(&before)
	t.Cleanup(func() {
		db.Exec("DELETE FROM custom_zone_card")
		for _, c := range before {
			db.Create(&c)
		}
	})

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
	card, err := Save(db, db, models.CustomZoneCard{ZoneShortName: "hole", Subtitle: " What Paineel Buried, It Did Not Forget ", Enabled: 1})
	if err != nil {
		t.Fatal(err)
	}
	if card.Subtitle != "What Paineel Buried, It Did Not Forget" || card.Title != "" {
		t.Errorf("saved %+v", card)
	}
	// saved again: the row is replaced, not doubled
	if _, err := Save(db, db, models.CustomZoneCard{ZoneShortName: "hole", Title: "The Hole", Subtitle: "Below Paineel", Enabled: 0}); err != nil {
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
	if _, err := Save(db, db, models.CustomZoneCard{ZoneShortName: "hole", Title: strings.Repeat("x", 63), Subtitle: strings.Repeat("y", 127), Enabled: 1}); err != nil {
		t.Errorf("at the limits: %v", err)
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
