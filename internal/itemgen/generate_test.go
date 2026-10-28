package itemgen

import (
	"fmt"
	"strings"
	"testing"

	"github.com/EQEmu/spire/internal/clientassets"
)

var cloth = []Template{
	{ID: 1001, Name: "Cloth Cap", Icon: 640, Slots: 4},
	{ID: 1002, Name: "Cloth Shirt", Icon: 631, Slots: 131072, Idfile: "IT63"},
	{ID: 1003, Name: "Cloth Sleeves", Icon: 632, Slots: 128},
	{ID: 1004, Name: "Cloth Wristband", Icon: 633, Slots: 1536},
	{ID: 1005, Name: "Cloth Gloves", Icon: 634, Slots: 4096},
	{ID: 1006, Name: "Cloth Pants", Icon: 635, Slots: 262144},
	{ID: 1007, Name: "Cloth Sandals", Icon: 636, Slots: 524288},
}

func TestArmorSetRows(t *testing.T) {
	rows, warnings := setRows(KindArmor, cloth, 253, []int{0, 1, 2, 3, 4, 5, 6}, "Molten")
	if len(rows) != 7 || len(warnings) != 0 {
		t.Fatalf("%+v %v", rows, warnings)
	}
	chest := rows[1]
	if chest.Name != "Molten Chestpiece" || chest.Herosforgemodel != 25301 || chest.Icon != 631 || chest.TemplateID != 1002 || chest.Idfile != "IT63" {
		t.Errorf("%+v", chest)
	}
	if rows[6].Herosforgemodel != 25306 || rows[6].Name != "Molten Boots" {
		t.Errorf("%+v", rows[6])
	}

	// a set with a robe and no feet: the robe copies the chest template
	rows, warnings = setRows(KindOrnament, cloth, 253, []int{0, 1, 2, 3, 4, 5, 7}, "Molten")
	if len(rows) != 7 || len(warnings) != 1 || !strings.Contains(warnings[0], "no boots") {
		t.Fatalf("%+v %v", rows, warnings)
	}
	robe := rows[6]
	if robe.Slot != clientassets.SlotRobe || robe.Herosforgemodel != 25307 || robe.TemplateID != 1002 || robe.Name != "Molten Robe Ornament" {
		t.Errorf("%+v", robe)
	}
}

func TestWeaponRows(t *testing.T) {
	a := &clientassets.Archive{File: "IT62100.eqg", Models: []int{62100, 62101}}
	sword := Template{ID: 5019, Name: "Fine Steel Short Sword", Icon: 592}
	if _, err := weaponRows(a, sword, nil, "Blade", 0); err == nil {
		t.Error("two models and none chosen")
	}
	if _, err := weaponRows(a, sword, []int{1}, "Blade", 0); err == nil {
		t.Error("a model not in the archive")
	}
	rows, err := weaponRows(a, sword, []int{62101}, "Blade", 18464)
	if err != nil || len(rows) != 1 || rows[0].Idfile != "IT62101" || rows[0].Icon != 18464 || rows[0].Herosforgemodel != 0 || rows[0].Name != "Blade" {
		t.Fatalf("%+v %v", rows, err)
	}
	rows, _ = weaponRows(a, sword, []int{62100, 62101}, "Blade", 0)
	if rows[1].Name != "Blade 2" || rows[1].Icon != 592 {
		t.Errorf("%+v", rows)
	}
}

func TestCopySQLCopiesEveryColumn(t *testing.T) {
	sql, args := copySQL([]string{"id", "Name", "lore", "icon", "custom_col"}, 1002, map[string]any{"id": 300000, "name": "Molten Helm", "icon": 640})
	want := "INSERT INTO items (`id`, `Name`, `lore`, `icon`, `custom_col`) SELECT ?, ?, IF(`lore` = `Name`, ?, IF(`lore` = CONCAT('*', `Name`), CONCAT('*', ?), `lore`)), ?, `custom_col` FROM items WHERE id = ?"
	if sql != want {
		t.Errorf("%v", sql)
	}
	if len(args) != 6 || args[0] != 300000 || args[5] != 1002 {
		t.Errorf("%v", args)
	}
}

func TestIDRange(t *testing.T) {
	r, err := checkRange(0, 0, false)
	if err != nil || r.start != FirstID || r.end != MaxID {
		t.Fatalf("%+v %v", r, err)
	}
	// augments (ornaments) keep ids a link can carry
	if r, _ := checkRange(0, 0, true); r.end != MaxAugmentID {
		t.Errorf("augment end %d", r.end)
	}
	if r, _ := checkRange(300000, 2000000, true); r.end != MaxAugmentID {
		t.Errorf("augment end past the limit: %d", r.end)
	}
	if _, err := checkRange(1101000, 0, true); err == nil || !strings.Contains(err.Error(), "augment") {
		t.Errorf("augment start past the limit: %v", err)
	}
	if _, err := checkRange(500000, 400000, false); err == nil {
		t.Error("start past the end")
	}
	if r, _ := checkRange(300000, 999999, false); r.end != 999999 {
		t.Errorf("end %d", r.end)
	}
}

func TestIDsSkipTheLinkMarkers(t *testing.T) {
	ids, err := takeIDs(0xFFFFC, idRange{start: 300000, end: MaxID}, 4)
	if err != nil || strings.Join(itoas(ids), ",") != "1048572,1048573,1048576,1048577" {
		t.Fatalf("%v %v", ids, err)
	}
	// an ornament set past the augment limit is refused, not wrapped
	if _, err := takeIDs(0xFFFFB, idRange{start: 300000, end: MaxAugmentID, augment: true}, 7); err == nil || !strings.Contains(err.Error(), "augments") {
		t.Errorf("%v", err)
	}
	if _, err := takeIDs(999995, idRange{start: 300000, end: 999999}, 7); err == nil {
		t.Error("past the end")
	}
}

func itoas(n []int) []string {
	var out []string
	for _, i := range n {
		out = append(out, fmt.Sprint(i))
	}
	return out
}
