package zoneloot

import (
	"strings"
	"testing"

	"gorm.io/gorm"
)

func f(v float64) *float64 { return &v }
func n(v int) *int         { return &v }

func npcByName(z *ZoneLoot, name string) *NPC {
	for i := range z.NPCs {
		if z.NPCs[i].Name == name {
			return &z.NPCs[i]
		}
	}
	return nil
}

func expected(npc *NPC, item int) float64 {
	for _, d := range npc.Drops {
		if d.ItemID == item {
			return d.Expected
		}
	}
	return 0
}

// The Warrens in PEQ: King_Gragnar (loot table 283, drop group 659 has Honey
// Mead 13033); loot table 392 (A_Kobold) is shared with Stonebrunt; drop
// group 18406 with loot tables elsewhere
func TestEdits(t *testing.T) {
	db := testDB(t)
	const zone = "warrens"
	s := NewSnapshots(t.TempDir())
	snap, err := s.Take(db, zone, "before edit tests")
	if err != nil {
		t.Fatal(err)
	}
	var maxTable, maxDrop int
	db.Raw("SELECT MAX(id) FROM loottable").Scan(&maxTable)
	db.Raw("SELECT MAX(id) FROM lootdrop").Scan(&maxDrop)
	t.Cleanup(func() {
		if _, _, err := s.Restore(db, zone, snap.ID); err != nil {
			t.Error(err)
		}
		cleanCopies(db, maxTable, maxDrop)
	})

	before, err := Load(db, zone, 0)
	if err != nil {
		t.Fatal(err)
	}
	king := npcByName(before, "King_Gragnar")
	mead := expected(king, 13033)

	// a preview changes nothing
	res, err := Apply(db, zone, 0, []Edit{{Op: "lootdrop_entry", LootdropID: 659, ItemID: 13033, Chance: f(100)}}, true)
	if err != nil {
		t.Fatal(err)
	}
	if got := expected(npcByName(res.Zone, "King_Gragnar"), 13033); got <= mead {
		t.Errorf("preview: Honey Mead %v, was %v", got, mead)
	}
	var chance float64
	db.Raw("SELECT chance FROM lootdrop_entries WHERE lootdrop_id = 659 AND item_id = 13033").Scan(&chance)
	if chance == 100 {
		t.Fatal("the preview changed the row")
	}

	// add an item, change one, remove one, in one save
	var gone int
	db.Raw("SELECT item_id FROM lootdrop_entries WHERE lootdrop_id = 659 AND item_id <> 13033 LIMIT 1").Scan(&gone)
	res, err = Apply(db, zone, 0, []Edit{
		{Op: "lootdrop_entry", LootdropID: 659, ItemID: 13033, Chance: f(100)},
		{Op: "lootdrop_entry", LootdropID: 659, ItemID: 1001, Chance: f(25), Multiplier: n(2)},
		{Op: "lootdrop_entry", LootdropID: 659, ItemID: gone, Remove: true},
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		ItemID     int
		Chance     float64
		Multiplier int
	}
	db.Raw("SELECT item_id, chance, multiplier FROM lootdrop_entries WHERE lootdrop_id = 659 AND item_id IN (13033, 1001, ?) ORDER BY item_id", gone).Scan(&rows)
	if len(rows) != 2 || rows[0].ItemID != 1001 || rows[0].Chance != 25 || rows[0].Multiplier != 2 || rows[1].Chance != 100 {
		t.Errorf("rows after save: %+v", rows)
	}
	if expected(npcByName(res.Zone, "King_Gragnar"), 1001) == 0 {
		t.Error("the new item does not drop")
	}

	// a bad change in a batch changes nothing
	_, err = Apply(db, zone, 0, []Edit{
		{Op: "lootdrop_entry", LootdropID: 659, ItemID: 13033, Chance: f(50)},
		{Op: "lootdrop_entry", LootdropID: 659, ItemID: 13033, Chance: f(150)},
	}, false)
	if err == nil || !strings.Contains(err.Error(), "change 2") {
		t.Errorf("out of range chance: %v", err)
	}
	db.Raw("SELECT chance FROM lootdrop_entries WHERE lootdrop_id = 659 AND item_id = 13033").Scan(&chance)
	if chance != 100 {
		t.Errorf("a failed save changed a row: %v", chance)
	}

	// shared rows: refused, then copied for this zone
	if _, err := Apply(db, zone, 0, []Edit{{Op: "loottable", LoottableID: 392, MaxCash: n(1000)}}, false); err == nil || !strings.Contains(err.Error(), "copy it first") {
		t.Errorf("shared loot table: %v", err)
	}
	if _, err := Apply(db, zone, 0, []Edit{{Op: "lootdrop_entry", LootdropID: 18406, ItemID: 1001, Chance: f(5)}}, false); err == nil || !strings.Contains(err.Error(), "copy it first") {
		t.Errorf("shared drop group: %v", err)
	}
	if _, err := Apply(db, zone, 0, []Edit{{Op: "lootdrop_entry", LootdropID: 1, ItemID: 1001, Chance: f(5)}}, false); err == nil {
		t.Error("changed a drop group of another zone")
	}
	// a deep copy copies the drop groups other zones use too
	res, err = Apply(db, zone, 0, []Edit{{Op: "copy_loottable", LoottableID: 392, Deep: true}}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Created) < 2 || res.Created[0].Table != "loottable" {
		t.Fatalf("deep copy made %+v", res.Created)
	}
	for _, e := range res.Zone.Tables[res.Created[0].ID].Entries {
		if g := res.Zone.Dropgroups[e.LootdropID]; g == nil || len(g.SharedBy) > 0 {
			t.Errorf("deep copy: drop group %d still shared: %+v", e.LootdropID, g)
		}
	}

	var outside int64
	db.Raw("SELECT COUNT(*) FROM npc_types WHERE loottable_id = 392").Scan(&outside)
	res, err = Apply(db, zone, 0, []Edit{
		{Op: "copy_loottable", LoottableID: 392},
		{Op: "copy_lootdrop", LootdropID: 18406},
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Created) != 2 {
		t.Fatalf("created: %+v", res.Created)
	}
	tableCopy, dropCopy := res.Created[0].ID, res.Created[1].ID
	if tbl := res.Zone.Tables[tableCopy]; tbl == nil || len(tbl.SharedBy) != 0 || !strings.HasSuffix(tbl.Name, "_warrens") || len(tbl.Entries) == 0 {
		t.Errorf("table copy: %+v", tbl)
	}
	if _, ok := res.Zone.Tables[392]; ok {
		t.Error("warrens NPCs still use loot table 392")
	}
	var left int64
	db.Raw("SELECT COUNT(*) FROM npc_types WHERE loottable_id = 392").Scan(&left)
	if left == 0 || left >= outside {
		t.Errorf("loot table 392 users: %v, were %v", left, outside)
	}
	if d := res.Zone.Dropgroups[dropCopy]; d == nil || len(d.SharedBy) != 0 || len(d.Entries) == 0 {
		t.Errorf("drop group copy: %+v", d)
	}
	if _, err := Apply(db, zone, 0, []Edit{
		{Op: "loottable", LoottableID: tableCopy, MaxCash: n(1000)},
		{Op: "lootdrop_entry", LootdropID: dropCopy, ItemID: 1001, Chance: f(5)},
	}, false); err != nil {
		t.Errorf("editing the copies: %v", err)
	}

	// new table for one NPC, a new drop group in it, an item in that
	var trash struct{ ID, LoottableID int }
	db.Raw("SELECT id, loottable_id FROM npc_types WHERE name = 'a_kobold_runt' AND id IN (SELECT npcID FROM spawnentry WHERE spawngroupID IN (SELECT spawngroupID FROM spawn2 WHERE zone = ?)) LIMIT 1", zone).Scan(&trash)
	if trash.ID == 0 {
		t.Fatal("no a_kobold_runt in warrens")
	}
	res, err = Apply(db, zone, 0, []Edit{{Op: "new_loottable", NpcIDs: []int{trash.ID}}}, false)
	if err != nil {
		t.Fatal(err)
	}
	newTable := res.Created[0].ID
	res, err = Apply(db, zone, 0, []Edit{{Op: "new_lootdrop", LoottableID: newTable}}, false)
	if err != nil {
		t.Fatal(err)
	}
	newDrop := res.Created[0].ID
	res, err = Apply(db, zone, 0, []Edit{{Op: "lootdrop_entry", LootdropID: newDrop, ItemID: 1001, Chance: f(100)}}, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, npc := range res.Zone.NPCs {
		if npc.ID == trash.ID && (npc.LoottableID != newTable || expected(&npc, 1001) != 1) {
			t.Errorf("new table: %+v", npc)
		}
	}
}

func cleanCopies(db *gorm.DB, maxTable, maxDrop int) {
	db.Exec("DELETE FROM loottable_entries WHERE loottable_id > ? OR lootdrop_id > ?", maxTable, maxDrop)
	db.Exec("DELETE FROM lootdrop_entries WHERE lootdrop_id > ?", maxDrop)
	db.Exec("DELETE FROM loottable WHERE id > ?", maxTable)
	db.Exec("DELETE FROM lootdrop WHERE id > ?", maxDrop)
}

// Nektulos has version 0 and version 1 spawns in PEQ
func TestVersions(t *testing.T) {
	db := testDB(t)
	v0, err := Load(db, "nektulos", 0)
	if err != nil {
		t.Fatal(err)
	}
	v1, err := Load(db, "nektulos", 1)
	if err != nil {
		t.Fatal(err)
	}
	if !containsInt(v0.Versions, 0) || !containsInt(v0.Versions, 1) {
		t.Errorf("versions: %v", v0.Versions)
	}
	if len(v0.NPCs) == 0 || len(v1.NPCs) == 0 {
		t.Fatalf("npcs: %d, %d", len(v0.NPCs), len(v1.NPCs))
	}
	ids := map[int]bool{}
	for _, npc := range v0.NPCs {
		ids[npc.ID] = true
	}
	differ := 0
	for _, npc := range v1.NPCs {
		if !ids[npc.ID] {
			differ++
		}
	}
	if differ == 0 {
		t.Error("version 1 has the same NPCs as version 0")
	}
	var points0, points1 int64
	db.Raw("SELECT COUNT(*) FROM spawn2 WHERE zone = 'nektulos' AND version IN (0, -1)").Scan(&points0)
	db.Raw("SELECT COUNT(*) FROM spawn2 WHERE zone = 'nektulos' AND version IN (1, -1)").Scan(&points1)
	t.Logf("nektulos: v0 %d NPCs on %d points, v1 %d NPCs on %d points", len(v0.NPCs), points0, len(v1.NPCs), points1)
	shared := 0
	for _, tbl := range v1.Tables {
		for _, o := range tbl.SharedBy {
			if o.Zone == "nektulos" {
				shared++
			}
		}
	}
	t.Logf("version 1 loot tables shared with version 0 NPCs: %d", shared)
}
