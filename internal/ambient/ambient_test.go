package ambient

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
// custom_ambient_line table (custom migration 71, and 72 if run)
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

func TestCleanName(t *testing.T) {
	for in, want := range map[string]string{"a_skeletal_servant": "a skeletal servant", "#Necromancer_Ardent": "Necromancer Ardent",
		"Guard_Ryn`Dal01": "Guard Ryn`Dal", "": ""} {
		if got := CleanName(in); got != want {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
}

func TestAmbientLines(t *testing.T) {
	db := testDB(t)
	var maxID uint32
	db.Raw("SELECT COALESCE(MAX(id), 0) FROM custom_ambient_line").Scan(&maxID)
	t.Cleanup(func() { db.Exec("DELETE FROM custom_ambient_line WHERE id > ?", maxID) })
	audience := hasColumn(db, "custom_ambient_line", "audience")

	l, err := Read(db, db, "paineel")
	if err != nil {
		t.Fatal(err)
	}
	if l.Counts["paineel"] < 13 || len(l.Lines) < 13 || len(l.NPCs) == 0 || l.Audience != audience {
		t.Fatalf("paineel: %d lines, %d NPCs, counts %v, audience %v", len(l.Lines), len(l.NPCs), l.Counts, l.Audience)
	}
	undead := 0
	for _, n := range l.NPCs {
		if n.Bodytype == 3 && !n.Silent {
			undead++
		}
	}
	if undead == 0 {
		t.Error("no undead NPCs in Paineel for the servants' lines")
	}

	// a greeting for necromancers, private
	aud := ""
	if audience {
		aud = "player"
	}
	line, err := Save(db, db, models.CustomAmbientLine{ZoneShortName: "Paineel", NpcRace: 3, Kind: "approach", Mode: "say", Audience: aud,
		PlayerClass: 11, Weight: 0, Text: " Welcome home, {Class}. ", Enabled: 1})
	if err != nil {
		t.Fatal(err)
	}
	if line.ID <= maxID || line.ZoneShortName != "paineel" || line.Text != "Welcome home, {class}." || line.Weight != 1 || line.TimeOfDay != "any" {
		t.Errorf("saved %+v", line)
	}
	// edited
	line.Mode, line.Text, line.TimeOfDay = "emote", "bows, just slightly.", "night"
	if _, err := Save(db, db, *line); err != nil {
		t.Fatal(err)
	}
	var row models.CustomAmbientLine
	q := db.Where("id = ?", line.ID)
	if !audience {
		q = q.Select("id, mode, text, time_of_day, player_class")
	}
	q.First(&row)
	if row.Mode != "emote" || row.Text != "bows, just slightly." || row.TimeOfDay != "night" || row.PlayerClass != 11 || row.Audience != aud {
		t.Errorf("row %+v", row)
	}

	for what, c := range map[string]models.CustomAmbientLine{
		"kind":        {Kind: "shout", Mode: "say", Text: "x"},
		"mode":        {Kind: "idle", Mode: "yell", Text: "x"},
		"audience":    {Kind: "idle", Mode: "say", Audience: "zone", Text: "x"},
		"time":        {Kind: "idle", Mode: "say", TimeOfDay: "dusk", Text: "x"},
		"no text":     {Kind: "idle", Mode: "say"},
		"two lines":   {Kind: "idle", Mode: "say", Text: "a\nb"},
		"long":        {Kind: "idle", Mode: "say", Text: strings.Repeat("x", 256)},
		"placeholder": {Kind: "idle", Mode: "say", Text: "Hello, {name}."},
		"levels":      {Kind: "idle", Mode: "say", Text: "x", MinLevel: 30, MaxLevel: 20},
		"zone":        {ZoneShortName: "nozonehere", Kind: "idle", Mode: "say", Text: "x"},
		"npc":         {NpcTypeID: 999999999, Kind: "idle", Mode: "say", Text: "x"},
		"no such id":  {ID: 999999999, Kind: "idle", Mode: "say", Text: "x"},
	} {
		if _, err := Save(db, db, c); err == nil {
			t.Errorf("%v: saved", what)
		}
	}
	if !audience {
		if _, err := Save(db, db, models.CustomAmbientLine{Kind: "idle", Mode: "say", Audience: "player", Text: "x"}); err == nil || !strings.Contains(err.Error(), "migration 72") {
			t.Errorf("an audience before migration 72: %v", err)
		}
	}

	// a line for every zone shows with each zone's
	every, err := Save(db, db, models.CustomAmbientLine{Kind: "idle", Mode: "emote", Text: "sneezes.", Enabled: 1})
	if err != nil {
		t.Fatal(err)
	}
	l, _ = Read(db, db, "warrens")
	found := false
	for _, x := range l.Lines {
		found = found || x.ID == every.ID
	}
	if !found {
		t.Error("a line for every zone isn't listed with the Warrens")
	}

	if err := Delete(db, line.ID); err != nil {
		t.Fatal(err)
	}
	if err := Delete(db, line.ID); err == nil {
		t.Error("deleted twice")
	}
}
