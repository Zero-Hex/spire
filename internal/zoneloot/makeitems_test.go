package zoneloot

import (
	"testing"
)

func TestSuggest(t *testing.T) {
	db := testDB(t)
	byTier := map[string][]Suggestion{}
	for _, tier := range []string{"trash", "named", "raid"} {
		s, err := Suggest(db, SuggestRequest{Pieces: []string{"head", "1hs", "fingers"}, Level: 24, Tier: tier})
		if err != nil {
			t.Fatal(err)
		}
		byTier[tier] = s
		for _, sg := range s {
			t.Logf("%-5v %-11v levels %v, %3d items, copy %v: %v", tier, sg.Piece.Label, sg.Levels, sg.Sample, sg.Template.Name, sg.Stats)
		}
	}
	head := func(tier string) Suggestion { return byTier[tier][0] }
	if head("named").Sample < 12 || head("named").Template == nil || head("named").Template.Slots&4 == 0 {
		t.Errorf("named head: %+v", head("named"))
	}
	if head("raid").Stats["ac"] < head("named").Stats["ac"] || head("named").Stats["ac"] < head("trash").Stats["ac"] {
		t.Errorf("AC by tier: %v %v %v", head("trash").Stats["ac"], head("named").Stats["ac"], head("raid").Stats["ac"])
	}
	if len(head("raid").Stats) < len(head("trash").Stats) {
		t.Errorf("raid has fewer stats than trash: %v / %v", head("raid").Stats, head("trash").Stats)
	}
	sword := byTier["named"][1]
	if sword.Template.ItemType != 0 || sword.Stats["damage"] == 0 || sword.Stats["delay"] == 0 {
		t.Errorf("sword: %+v", sword)
	}
	if _, err := Suggest(db, SuggestRequest{Pieces: []string{"head"}, Level: 24, Tier: "epic"}); err == nil {
		t.Error("unknown tier")
	}
}

func TestMake(t *testing.T) {
	db := testDB(t)
	const zone = "warrens"
	s := NewSnapshots(t.TempDir())
	snap, err := s.Take(db, zone, "before make test")
	if err != nil {
		t.Fatal(err)
	}
	var maxTable, maxDrop, maxItem int
	db.Raw("SELECT MAX(id) FROM loottable").Scan(&maxTable)
	db.Raw("SELECT MAX(id) FROM lootdrop").Scan(&maxDrop)
	db.Raw("SELECT COALESCE(MAX(id), 0) FROM items WHERE id >= 900000 AND id < 1000000").Scan(&maxItem)
	t.Cleanup(func() {
		s.Restore(db, zone, snap.ID)
		cleanCopies(db, maxTable, maxDrop)
		db.Exec("DELETE FROM items WHERE id > ? AND id >= 900000 AND id < 1000000", maxItem)
	})

	sug, err := Suggest(db, SuggestRequest{Pieces: []string{"head", "1hs"}, Level: 24, Tier: "named"})
	if err != nil {
		t.Fatal(err)
	}
	var items []NewItem
	for i, sg := range sug {
		items = append(items, NewItem{TemplateID: sg.Template.ID, Name: []string{"Gragnar's Crown", "Gragnar's Cleaver"}[i], Icon: sg.Template.Icon,
			Idfile: sg.Template.Idfile, Stats: sg.Stats, RecLevel: 24, NoDrop: true, Lore: i == 0, Chance: 20})
	}
	// a template with a clicky: the copy must not keep it
	var clicky int
	db.Raw("SELECT id FROM items WHERE clickeffect > 0 AND slots & 4 LIMIT 1").Scan(&clicky)
	items[0].TemplateID = clicky

	res, err := Make(db, zone, MakeRequest{Version: 0, Start: 900000, End: 999999, Items: items, LoottableID: 283, GroupName: "Gragnar test loot"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 2 || res.Items[0].ID <= maxItem || res.Items[1].ID != res.Items[0].ID+1 {
		t.Fatalf("items: %+v (highest before %d)", res.Items, maxItem)
	}
	var made []struct {
		ID, Ac, Damage, Delay, Clickeffect, Nodrop, Reclevel int
		Name, Lore                                           string
	}
	db.Raw("SELECT id, Name AS name, lore, ac, damage, delay, clickeffect, nodrop, reclevel FROM items WHERE id IN ? ORDER BY id", []int{res.Items[0].ID, res.Items[1].ID}).Scan(&made)
	if len(made) != 2 {
		t.Fatalf("made: %+v", made)
	}
	if made[0].Name != "Gragnar's Crown" || made[0].Lore != "*Gragnar's Crown" || made[0].Clickeffect != -1 || made[0].Nodrop != 0 || made[0].Reclevel != 24 || made[0].Ac != sug[0].Stats["ac"] {
		t.Errorf("crown: %+v", made[0])
	}
	if made[1].Lore != "Gragnar's Cleaver" || made[1].Damage != sug[1].Stats["damage"] || made[1].Delay != sug[1].Stats["delay"] {
		t.Errorf("cleaver: %+v", made[1])
	}
	king := npcByName(res.Zone, "King_Gragnar")
	if king == nil || expected(king, res.Items[0].ID) == 0 || expected(king, res.Items[1].ID) == 0 {
		t.Errorf("the items don't drop: %+v", king)
	}
	if g := res.Zone.Dropgroups[res.LootdropID]; g == nil || g.Name != "Gragnar test loot" || len(g.Entries) != 2 {
		t.Errorf("drop group: %+v", g)
	}

	// a failure makes nothing: a shared table can't take a new group
	before := 0
	db.Raw("SELECT COUNT(*) FROM items WHERE id >= 900000 AND id < 1000000").Scan(&before)
	if _, err := Make(db, zone, MakeRequest{Start: 900000, End: 999999, Items: items[:1], LoottableID: 392}); err == nil {
		t.Error("made items for a shared loot table")
	}
	after := 0
	db.Raw("SELECT COUNT(*) FROM items WHERE id >= 900000 AND id < 1000000").Scan(&after)
	if after != before {
		t.Errorf("a failed make left %d item(s)", after-before)
	}
}
