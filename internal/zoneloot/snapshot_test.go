package zoneloot

import (
	"os"
	"reflect"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Runs against a database with PEQ content when ZONELOOT_TEST_DSN names one
// (e.g. eq:eq@tcp(127.0.0.1:3306)/peq). It changes The Warrens' rows and
// restores them.
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

func TestSnapshotRestore(t *testing.T) {
	db := testDB(t)
	s := NewSnapshots(t.TempDir())
	const zone = "warrens"

	orig, err := read(db, zone)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := s.Take(db, zone, "test")
	if err != nil {
		t.Fatal(err)
	}
	if snap.Counts["spawn2"] == 0 || snap.Counts["lootdrop_entries"] == 0 {
		t.Fatalf("empty snapshot: %v", snap.Counts)
	}

	// pick rows to change
	var npc struct {
		ID          int
		Name        string
		LoottableID int
	}
	db.Raw("SELECT n.id, n.name, n.loottable_id FROM npc_types n JOIN spawnentry e ON e.npcID = n.id JOIN spawn2 s ON s.spawngroupID = e.spawngroupID WHERE s.zone = ? AND n.loottable_id > 0 ORDER BY n.id LIMIT 1", zone).Scan(&npc)
	var le struct{ LootdropID, ItemID int }
	db.Raw("SELECT lootdrop_id, item_id FROM lootdrop_entries WHERE lootdrop_id IN (SELECT lootdrop_id FROM loottable_entries WHERE loottable_id = ?) LIMIT 1", npc.LoottableID).Scan(&le)
	var maxSpawn int
	db.Raw("SELECT MAX(id) FROM spawn2").Scan(&maxSpawn)

	must := func(q string, args ...any) {
		t.Helper()
		if err := db.Exec(q, args...).Error; err != nil {
			t.Fatal(err)
		}
	}
	must("UPDATE npc_types SET name = 'Changed_Name' WHERE id = ?", npc.ID)
	must("UPDATE lootdrop_entries SET chance = 99.5 WHERE lootdrop_id = ? AND item_id = ?", le.LootdropID, le.ItemID)
	must("DELETE FROM loottable_entries WHERE loottable_id = ? LIMIT 1", npc.LoottableID)
	must("INSERT INTO spawn2 (id, spawngroupID, zone, x, y, z) VALUES (?, 1, ?, 0, 0, 0)", maxSpawn+1, zone)

	diff, err := Diff(db, snap)
	if err != nil {
		t.Fatal(err)
	}
	if len(diff["npc_types"].Changed) != 1 || diff["npc_types"].Changed[0].Columns[0] != "name" {
		t.Errorf("npc_types: %+v", diff["npc_types"])
	}
	if len(diff["lootdrop_entries"].Changed) != 1 {
		t.Errorf("lootdrop_entries: %+v", diff["lootdrop_entries"])
	}
	if len(diff["loottable_entries"].Added) != 1 {
		t.Errorf("loottable_entries: %+v", diff["loottable_entries"])
	}
	if len(diff["spawn2"].Removed) != 1 {
		t.Errorf("spawn2: %+v", diff["spawn2"])
	}

	before, _, err := s.Restore(db, zone, snap.ID)
	if err != nil {
		t.Fatal(err)
	}
	after, err := read(db, zone)
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range tableOrder {
		if !reflect.DeepEqual(sorted(table, orig[table]), sorted(table, after[table])) {
			t.Errorf("%v differs after restore", table)
		}
	}
	diff, _ = Diff(db, snap)
	for table, d := range diff {
		if len(d.Changed)+len(d.Added)+len(d.Removed) > 0 {
			t.Errorf("%v still differs: %+v", table, d)
		}
	}

	list, _ := s.List(zone)
	if len(list) != 2 || list[0].ID != before.ID || list[0].Note != "before restoring "+snap.ID {
		t.Errorf("list: %+v", list)
	}
	if err := s.Delete(zone, before.ID); err != nil {
		t.Error(err)
	}
	if _, err := s.Get(zone, "../../etc"); err == nil {
		t.Error("read outside the snapshot folder")
	}
}

func sorted(table string, r *Rows) map[string][]*string {
	out := map[string][]*string{}
	for _, row := range r.Rows {
		out[keyOf(r.Columns, row, tableKeys[table])] = row
	}
	return out
}
