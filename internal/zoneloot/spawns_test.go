package zoneloot

import (
	"fmt"
	"strings"
	"testing"

	"gorm.io/gorm"
)

func ip(v int) *int { return &v }

// keepZone snapshots a zone and puts it back when the test ends, dropping
// spawn groups the test made
func keepZone(t *testing.T, db *gorm.DB, zone string) {
	s := NewSnapshots(t.TempDir())
	snap, err := s.Take(db, zone, "test")
	if err != nil {
		t.Fatal(err)
	}
	var maxGroup int
	db.Raw("SELECT MAX(id) FROM spawngroup").Scan(&maxGroup)
	t.Cleanup(func() {
		if _, _, err := s.Restore(db, zone, snap.ID); err != nil {
			t.Error(err)
		}
		db.Exec("DELETE FROM spawnentry WHERE spawngroupID > ?", maxGroup)
		db.Exec("DELETE FROM spawngroup WHERE id > ?", maxGroup)
	})
}

func TestSpawnEdits(t *testing.T) {
	db := testDB(t)
	const zone = "warrens"
	keepZone(t, db, zone)

	z, err := Load(db, zone, 0)
	if err != nil {
		t.Fatal(err)
	}
	g := z.Spawngroups[12635]
	if g == nil || len(g.Points) != 2 || len(g.Entries) == 0 || len(g.SharedBy) != 0 {
		t.Fatalf("group 12635: %+v", g)
	}
	first := g.Entries[0].NpcID
	king := npcByName(z, "King_Gragnar")
	kingUp := king.Up

	// King Gragnar into a group of two points, a chance changed, limits set
	res, err := Apply(db, zone, 0, []Edit{
		{Op: "spawnentry", SpawngroupID: ip(12635), NpcID: king.ID, Chance: f(50)},
		{Op: "spawnentry", SpawngroupID: ip(12635), NpcID: first, Chance: f(50)},
		{Op: "spawngroup", SpawngroupID: ip(12635), SpawnLimit: ip(1), Delay: ip(30000)},
		{Op: "spawn2", SpawnID: g.Points[0].ID, Respawntime: ip(1200), Variance: ip(60)},
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	pg := res.Zone.Spawngroups[12635]
	if pg.SpawnLimit != 1 || pg.Delay != 30000 || pg.Points[0].Respawntime != 1200 || pg.Points[0].Variance != 60 {
		t.Errorf("preview group: %+v", pg)
	}
	if up := npcByName(res.Zone, "King_Gragnar").Up; up <= kingUp {
		t.Errorf("King Gragnar up %v, was %v", up, kingUp)
	}
	var n int64
	db.Raw("SELECT COUNT(*) FROM spawnentry WHERE spawngroupID = 12635 AND npcID = ?", king.ID).Scan(&n)
	if n != 0 {
		t.Fatal("the preview changed the rows")
	}

	// remove, a new group for one point, a copy for another
	var g39 Spawngroup
	if p := z.Spawngroups[12639]; p != nil {
		g39 = *p
	}
	var g37 Spawngroup
	if p := z.Spawngroups[12637]; p != nil {
		g37 = *p
	}
	res, err = Apply(db, zone, 0, []Edit{
		{Op: "spawnentry", SpawngroupID: ip(12635), NpcID: first, Remove: true},
		{Op: "new_spawngroup", SpawnIDs: []int{g39.Points[0].ID}},
		{Op: "copy_spawngroup", SpawngroupID: ip(12637), SpawnIDs: []int{g37.Points[0].ID}},
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Created) != 2 {
		t.Fatalf("created %+v", res.Created)
	}
	fresh, copied := res.Zone.Spawngroups[res.Created[0].ID], res.Zone.Spawngroups[res.Created[1].ID]
	if fresh == nil || len(fresh.Points) != 1 || len(fresh.Entries) != 0 || fresh.Name != fmt.Sprintf("warrens_%d", fresh.ID) {
		t.Errorf("new group: %+v", fresh)
	}
	if copied == nil || len(copied.Points) != 1 || len(copied.Entries) != len(g37.Entries) || !strings.HasSuffix(copied.Name, fmt.Sprintf("_%d", copied.ID)) {
		t.Errorf("copied group: %+v", copied)
	}
	if left := res.Zone.Spawngroups[12637]; left == nil || len(left.Points) != len(g37.Points)-1 {
		t.Errorf("group 12637 after the copy: %+v", left)
	}
	for _, e := range res.Zone.Spawngroups[12635].Entries {
		if e.NpcID == first {
			t.Error("entry not removed")
		}
	}

	// a point of another zone, a bad chance
	var other int
	db.Raw("SELECT id FROM spawn2 WHERE zone <> ? LIMIT 1", zone).Scan(&other)
	if _, err := Apply(db, zone, 0, []Edit{{Op: "spawn2", SpawnID: other, Respawntime: ip(5)}}, false); err == nil {
		t.Error("changed another zone's spawn point")
	}
	if _, err := Apply(db, zone, 0, []Edit{{Op: "spawnentry", SpawngroupID: ip(12635), NpcID: king.ID, Chance: f(101)}}, false); err == nil {
		t.Error("chance over 100")
	}
}

// Nektulos's version 0 and 1 share spawn groups
func TestSpawnVersions(t *testing.T) {
	db := testDB(t)
	keepZone(t, db, "nektulos")
	z, err := Load(db, "nektulos", 1)
	if err != nil {
		t.Fatal(err)
	}
	g := z.Spawngroups[47140]
	if g == nil || len(g.SharedBy) == 0 || !strings.Contains(g.SharedBy[0].Zone, "nektulos v0") {
		t.Fatalf("group 47140 in v1: %+v", g)
	}
	if _, err := Apply(db, "nektulos", 1, []Edit{{Op: "spawngroup", SpawngroupID: ip(47140), SpawnLimit: ip(1)}}, false); err == nil || !strings.Contains(err.Error(), "copy it first") {
		t.Errorf("shared spawn group: %v", err)
	}
	res, err := Apply(db, "nektulos", 1, []Edit{{Op: "copy_spawngroup", SpawngroupID: ip(47140)}}, false)
	if err != nil {
		t.Fatal(err)
	}
	copied := res.Zone.Spawngroups[res.Created[0].ID]
	if copied == nil || len(copied.SharedBy) != 0 || len(copied.Points) != len(g.Points) {
		t.Errorf("copy: %+v", copied)
	}
	var v0 int64
	db.Raw("SELECT COUNT(*) FROM spawn2 WHERE zone = 'nektulos' AND version = 0 AND spawngroupID = 47140").Scan(&v0)
	if v0 == 0 {
		t.Error("version 0 lost the group")
	}
	if _, err := Apply(db, "nektulos", 1, []Edit{{Op: "spawngroup", SpawngroupID: ip(copied.ID), SpawnLimit: ip(1)}}, false); err != nil {
		t.Errorf("editing the copy: %v", err)
	}
}

func TestNPCSearch(t *testing.T) {
	db := testDB(t)
	for _, q := range []string{"King Gragnar", "101109", "king_grag"} {
		hits, err := NPCSearch(db, q)
		if err != nil {
			t.Fatal(err)
		}
		if len(hits) == 0 || hits[0].ID != 101109 || hits[0].Level == 0 {
			t.Errorf("%q: %+v", q, hits)
		}
	}
}
