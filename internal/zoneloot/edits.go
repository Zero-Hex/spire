package zoneloot

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"gorm.io/gorm"
)

// An Edit changes one loot row of a zone version. Values left out (nil) stay
// as they are. Rows the zone version's NPCs share with NPCs elsewhere (a loot
// table another zone's NPC uses, a drop group another loot table uses) are
// changed only when the edit says Everywhere; otherwise copy them first.
//
// Ops:
//
//	npc_loottable    NpcID → LoottableID (0: no loot)
//	loottable        LoottableID: Name, MinCash, MaxCash
//	loottable_entry  LoottableID + LootdropID: Multiplier, Droplimit, Mindrop, Probability, or Remove
//	lootdrop         LootdropID: Name
//	lootdrop_entry   LootdropID + ItemID: Chance, Multiplier, Charges, EquipItem, NpcMinLevel, NpcMaxLevel, or Remove
//	copy_loottable   LoottableID, for NpcIDs (default: every NPC here using it); Deep also copies
//	                 its drop groups that loot tables outside the zone version use
//	copy_lootdrop    LootdropID, for LoottableIDs (default: every loot table here using it)
//	new_loottable    Name, for NpcIDs
//	new_lootdrop     Name, added to LoottableID
type Edit struct {
	Op string `json:"op"`

	NpcID       int `json:"npc_id,omitempty"`
	LoottableID int `json:"loottable_id,omitempty"`
	LootdropID  int `json:"lootdrop_id,omitempty"`
	ItemID      int `json:"item_id,omitempty"`

	Name        *string  `json:"name,omitempty"`
	MinCash     *int     `json:"mincash,omitempty"`
	MaxCash     *int     `json:"maxcash,omitempty"`
	Multiplier  *int     `json:"multiplier,omitempty"`
	Droplimit   *int     `json:"droplimit,omitempty"`
	Mindrop     *int     `json:"mindrop,omitempty"`
	Probability *float64 `json:"probability,omitempty"`
	Chance      *float64 `json:"chance,omitempty"`
	Charges     *int     `json:"item_charges,omitempty"`
	EquipItem   *bool    `json:"equip_item,omitempty"`
	NpcMinLevel *int     `json:"npc_min_level,omitempty"`
	NpcMaxLevel *int     `json:"npc_max_level,omitempty"`
	Remove      bool     `json:"remove,omitempty"`

	NpcIDs       []int `json:"npc_ids,omitempty"`
	Deep         bool  `json:"deep,omitempty"`
	LoottableIDs []int `json:"loottable_ids,omitempty"`

	Everywhere bool `json:"everywhere,omitempty"`
}

// Created is a row an edit made
type Created struct {
	Edit  int    `json:"edit"` // its index in the request
	Table string `json:"table"`
	ID    int    `json:"id"`
}

type EditResult struct {
	Created []Created `json:"created"`
	Zone    *ZoneLoot `json:"zone"`
}

var errPreview = errors.New("preview")

var lootTables = []string{"npc_types", "loottable", "loottable_entries", "lootdrop", "lootdrop_entries"}

// Transactional says whether the loot tables can be changed in a transaction
// (InnoDB), which previews and all-or-nothing saves need
func Transactional(db *gorm.DB) error {
	var rows []struct{ TableName, Engine string }
	db.Raw("SELECT TABLE_NAME AS table_name, ENGINE AS engine FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME IN ?", lootTables).Scan(&rows)
	for _, r := range rows {
		if !strings.EqualFold(r.Engine, "InnoDB") {
			return fmt.Errorf("%v is a %v table: loot can only be edited where those tables are InnoDB", r.TableName, r.Engine)
		}
	}
	return nil
}

// Apply makes the edits in one transaction and reads the zone version back.
// With preview, it reads the zone as the edits would leave it and changes
// nothing.
func Apply(db *gorm.DB, zone string, version int, edits []Edit, preview bool) (*EditResult, error) {
	if err := Transactional(db); err != nil {
		return nil, err
	}
	res := &EditResult{Created: []Created{}}
	err := db.Transaction(func(tx *gorm.DB) error {
		e := &editor{tx: tx, zone: zone, version: version}
		for i, ed := range edits {
			if err := e.refresh(); err != nil {
				return err
			}
			e.at = i
			if err := e.apply(ed); err != nil {
				return fmt.Errorf("change %d (%v): %v", i+1, ed.Op, err)
			}
		}
		res.Created = append(res.Created, e.created...)
		z, err := Load(tx, zone, version)
		if err != nil {
			return err
		}
		res.Zone = z
		if preview {
			return errPreview
		}
		return nil
	})
	if err != nil && err != errPreview {
		return nil, err
	}
	return res, nil
}

type editor struct {
	tx      *gorm.DB
	zone    string
	version int
	sc      scope
	npcs    map[int]bool
	tables  map[int]bool
	drops   map[int]bool
	at      int // the edit being applied
	created []Created
}

func (e *editor) made(table string, id int) {
	e.created = append(e.created, Created{Edit: e.at, Table: table, ID: id})
}

func set(ids []int) map[int]bool {
	m := map[int]bool{}
	for _, id := range ids {
		m[id] = true
	}
	return m
}

func (e *editor) refresh() error {
	sc, err := zoneScope(e.tx, e.zone, &e.version)
	if err != nil {
		return err
	}
	e.sc, e.npcs, e.tables, e.drops = sc, set(sc.npcs), set(sc.tables), set(sc.drops)
	return nil
}

func (e *editor) count(q string, args ...any) int64 {
	var n int64
	e.tx.Raw(q, args...).Scan(&n)
	return n
}

func (e *editor) exists(table string, id int) bool {
	return e.count("SELECT COUNT(*) FROM `"+table+"` WHERE id = ?", id) > 0
}

// the loot table, which must be this zone version's, and not used outside it
// unless the edit says everywhere
func (e *editor) ownTable(id int, everywhere bool) error {
	if !e.tables[id] {
		return fmt.Errorf("loot table %d is not used by an NPC in %v", id, e.where())
	}
	if !everywhere && e.count("SELECT COUNT(*) FROM npc_types WHERE loottable_id = ? AND id NOT IN ?", id, nonEmpty(e.sc.npcs)) > 0 {
		return fmt.Errorf("loot table %d is also used by NPCs outside %v: copy it first, or change it everywhere", id, e.where())
	}
	return nil
}

func (e *editor) ownDrop(id int, everywhere bool) error {
	if !e.drops[id] {
		return fmt.Errorf("drop group %d is not in a loot table in %v", id, e.where())
	}
	if !everywhere && e.count("SELECT COUNT(*) FROM loottable_entries WHERE lootdrop_id = ? AND loottable_id NOT IN ?", id, nonEmpty(e.sc.tables)) > 0 {
		return fmt.Errorf("drop group %d is also in loot tables outside %v: copy it first, or change it everywhere", id, e.where())
	}
	return nil
}

func (e *editor) where() string {
	if e.version == 0 {
		return e.zone
	}
	return fmt.Sprintf("%v version %v", e.zone, e.version)
}

func inRange(name string, v *int, lo, hi int) error {
	if v != nil && (*v < lo || *v > hi) {
		return fmt.Errorf("%v must be from %d to %d", name, lo, hi)
	}
	return nil
}

func percent(name string, v *float64) error {
	if v != nil && (*v < 0 || *v > 100) {
		return fmt.Errorf("%v must be from 0 to 100", name)
	}
	return nil
}

func firstErr(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// columns adds col = value for each value given
type columns struct {
	names []string
	vals  []any
}

func (c *columns) add(name string, v any) {
	switch x := v.(type) {
	case *int:
		if x == nil {
			return
		}
		v = *x
	case *float64:
		if x == nil {
			return
		}
		v = *x
	case *string:
		if x == nil {
			return
		}
		v = *x
	case *bool:
		if x == nil {
			return
		}
		b := 0
		if *x {
			b = 1
		}
		v = b
	}
	c.names = append(c.names, name)
	c.vals = append(c.vals, v)
}

func (c *columns) assignments() string {
	var parts []string
	for _, n := range c.names {
		parts = append(parts, "`"+n+"` = ?")
	}
	return strings.Join(parts, ", ")
}

func (e *editor) apply(ed Edit) error {
	tx := e.tx
	switch ed.Op {
	case "npc_loottable":
		if !e.npcs[ed.NpcID] {
			return fmt.Errorf("NPC %d does not spawn in %v", ed.NpcID, e.where())
		}
		if ed.LoottableID < 0 || (ed.LoottableID > 0 && !e.exists("loottable", ed.LoottableID)) {
			return fmt.Errorf("there is no loot table %d", ed.LoottableID)
		}
		return tx.Exec("UPDATE npc_types SET loottable_id = ? WHERE id = ?", ed.LoottableID, ed.NpcID).Error

	case "loottable":
		if err := firstErr(e.ownTable(ed.LoottableID, ed.Everywhere), inRange("mincash", ed.MinCash, 0, 2147483647), inRange("maxcash", ed.MaxCash, 0, 2147483647)); err != nil {
			return err
		}
		var c columns
		c.add("name", ed.Name)
		c.add("mincash", ed.MinCash)
		c.add("maxcash", ed.MaxCash)
		if len(c.names) == 0 {
			return nil
		}
		if err := tx.Exec("UPDATE loottable SET "+c.assignments()+" WHERE id = ?", append(c.vals, ed.LoottableID)...).Error; err != nil {
			return err
		}
		if e.count("SELECT COUNT(*) FROM loottable WHERE id = ? AND mincash > maxcash", ed.LoottableID) > 0 {
			return errors.New("the least cash is more than the most")
		}
		return nil

	case "loottable_entry":
		if err := firstErr(e.ownTable(ed.LoottableID, ed.Everywhere), inRange("multiplier", ed.Multiplier, 0, 255),
			inRange("drop limit", ed.Droplimit, 0, 255), inRange("minimum drops", ed.Mindrop, 0, 255), percent("probability", ed.Probability)); err != nil {
			return err
		}
		if ed.Remove {
			return tx.Exec("DELETE FROM loottable_entries WHERE loottable_id = ? AND lootdrop_id = ?", ed.LoottableID, ed.LootdropID).Error
		}
		if !e.exists("lootdrop", ed.LootdropID) {
			return fmt.Errorf("there is no drop group %d", ed.LootdropID)
		}
		var c columns
		c.add("multiplier", ed.Multiplier)
		c.add("droplimit", ed.Droplimit)
		c.add("mindrop", ed.Mindrop)
		c.add("probability", ed.Probability)
		return upsert(tx, "loottable_entries", []string{"loottable_id", "lootdrop_id"}, []any{ed.LoottableID, ed.LootdropID}, c)

	case "lootdrop":
		if err := e.ownDrop(ed.LootdropID, ed.Everywhere); err != nil {
			return err
		}
		if ed.Name == nil {
			return nil
		}
		return tx.Exec("UPDATE lootdrop SET name = ? WHERE id = ?", *ed.Name, ed.LootdropID).Error

	case "lootdrop_entry":
		if err := firstErr(e.ownDrop(ed.LootdropID, ed.Everywhere), percent("chance", ed.Chance), inRange("multiplier", ed.Multiplier, 0, 255),
			inRange("charges", ed.Charges, 0, 65535), inRange("NPC min level", ed.NpcMinLevel, 0, 65535), inRange("NPC max level", ed.NpcMaxLevel, 0, 65535)); err != nil {
			return err
		}
		if ed.Remove {
			return tx.Exec("DELETE FROM lootdrop_entries WHERE lootdrop_id = ? AND item_id = ?", ed.LootdropID, ed.ItemID).Error
		}
		if !e.exists("items", ed.ItemID) {
			return fmt.Errorf("there is no item %d", ed.ItemID)
		}
		var c columns
		c.add("chance", ed.Chance)
		c.add("multiplier", ed.Multiplier)
		c.add("item_charges", ed.Charges)
		c.add("equip_item", ed.EquipItem)
		c.add("npc_min_level", ed.NpcMinLevel)
		c.add("npc_max_level", ed.NpcMaxLevel)
		return upsert(tx, "lootdrop_entries", []string{"lootdrop_id", "item_id"}, []any{ed.LootdropID, ed.ItemID}, c)

	case "copy_loottable":
		if !e.tables[ed.LoottableID] {
			return fmt.Errorf("loot table %d is not used by an NPC in %v", ed.LoottableID, e.where())
		}
		var users []int
		tx.Raw("SELECT id FROM npc_types WHERE loottable_id = ? AND id IN ?", ed.LoottableID, nonEmpty(e.sc.npcs)).Scan(&users)
		npcs, err := subset(users, ed.NpcIDs, "NPC", fmt.Sprintf("using loot table %d in %v", ed.LoottableID, e.where()))
		if err != nil {
			return err
		}
		id, err := copyRow(tx, "loottable", ed.LoottableID, e.suffix())
		if err != nil {
			return err
		}
		if err := copyChildren(tx, "loottable_entries", "loottable_id", ed.LoottableID, id); err != nil {
			return err
		}
		e.made("loottable", id)
		if err := tx.Exec("UPDATE npc_types SET loottable_id = ? WHERE id IN ?", id, npcs).Error; err != nil {
			return err
		}
		if !ed.Deep {
			return nil
		}
		// the copy's drop groups that tables outside use: copied for it too
		if err := e.refresh(); err != nil {
			return err
		}
		var groups []int
		tx.Raw("SELECT lootdrop_id FROM loottable_entries WHERE loottable_id = ?", id).Scan(&groups)
		for _, g := range groups {
			if e.count("SELECT COUNT(*) FROM loottable_entries WHERE lootdrop_id = ? AND loottable_id NOT IN ?", g, nonEmpty(e.sc.tables)) == 0 {
				continue
			}
			gid, err := copyRow(tx, "lootdrop", g, e.suffix())
			if err != nil {
				return err
			}
			if err := copyChildren(tx, "lootdrop_entries", "lootdrop_id", g, gid); err != nil {
				return err
			}
			if err := tx.Exec("UPDATE loottable_entries SET lootdrop_id = ? WHERE loottable_id = ? AND lootdrop_id = ?", gid, id, g).Error; err != nil {
				return err
			}
			e.made("lootdrop", gid)
		}
		return nil

	case "copy_lootdrop":
		if !e.drops[ed.LootdropID] {
			return fmt.Errorf("drop group %d is not in a loot table in %v", ed.LootdropID, e.where())
		}
		var users []int
		tx.Raw("SELECT loottable_id FROM loottable_entries WHERE lootdrop_id = ? AND loottable_id IN ?", ed.LootdropID, nonEmpty(e.sc.tables)).Scan(&users)
		tables, err := subset(users, ed.LoottableIDs, "loot table", fmt.Sprintf("with drop group %d in %v", ed.LootdropID, e.where()))
		if err != nil {
			return err
		}
		id, err := copyRow(tx, "lootdrop", ed.LootdropID, e.suffix())
		if err != nil {
			return err
		}
		if err := copyChildren(tx, "lootdrop_entries", "lootdrop_id", ed.LootdropID, id); err != nil {
			return err
		}
		e.made("lootdrop", id)
		return tx.Exec("UPDATE loottable_entries SET lootdrop_id = ? WHERE lootdrop_id = ? AND loottable_id IN ?", id, ed.LootdropID, tables).Error

	case "new_loottable":
		for _, n := range ed.NpcIDs {
			if !e.npcs[n] {
				return fmt.Errorf("NPC %d does not spawn in %v", n, e.where())
			}
		}
		name := e.zone + "_loot"
		if ed.Name != nil && strings.TrimSpace(*ed.Name) != "" {
			name = strings.TrimSpace(*ed.Name)
		}
		id, err := insertID(tx, "INSERT INTO loottable (name) VALUES (?)", name)
		if err != nil {
			return err
		}
		e.made("loottable", id)
		if len(ed.NpcIDs) > 0 {
			return tx.Exec("UPDATE npc_types SET loottable_id = ? WHERE id IN ?", id, ed.NpcIDs).Error
		}
		return nil

	case "new_lootdrop":
		if err := e.ownTable(ed.LoottableID, ed.Everywhere); err != nil {
			return err
		}
		name := fmt.Sprintf("%d_%v", ed.LoottableID, e.zone)
		if ed.Name != nil && strings.TrimSpace(*ed.Name) != "" {
			name = strings.TrimSpace(*ed.Name)
		}
		id, err := insertID(tx, "INSERT INTO lootdrop (name) VALUES (?)", name)
		if err != nil {
			return err
		}
		e.made("lootdrop", id)
		// one pick from the group, every kill: the usual start
		return tx.Exec("INSERT INTO loottable_entries (loottable_id, lootdrop_id, multiplier, droplimit, mindrop, probability) VALUES (?, ?, 1, 1, 0, 100)", ed.LoottableID, id).Error
	}
	return fmt.Errorf("unknown change %q", ed.Op)
}

func (e *editor) suffix() string {
	if e.version == 0 {
		return "_" + e.zone
	}
	return fmt.Sprintf("_%v_v%d", e.zone, e.version)
}

// subset is want if every id is in have, or have when want is empty
func subset(have, want []int, what, where string) ([]int, error) {
	if len(have) == 0 {
		return nil, fmt.Errorf("no %v is %v", what, where)
	}
	if len(want) == 0 {
		return have, nil
	}
	in := set(have)
	for _, id := range want {
		if !in[id] {
			return nil, fmt.Errorf("%v %d is not %v", what, id, where)
		}
	}
	return want, nil
}

func upsert(tx *gorm.DB, table string, keys []string, keyVals []any, c columns) error {
	names := append(append([]string{}, keys...), c.names...)
	vals := append(append([]any{}, keyVals...), c.vals...)
	quoted := make([]string, len(names))
	for i, n := range names {
		quoted[i] = "`" + n + "`"
	}
	q := "INSERT INTO `" + table + "` (" + strings.Join(quoted, ", ") + ") VALUES (" + strings.TrimSuffix(strings.Repeat("?, ", len(names)), ", ") + ")"
	if len(c.names) > 0 {
		var upd []string
		for _, n := range c.names {
			upd = append(upd, "`"+n+"` = VALUES(`"+n+"`)")
		}
		q += " ON DUPLICATE KEY UPDATE " + strings.Join(upd, ", ")
	} else {
		q = strings.Replace(q, "INSERT INTO", "INSERT IGNORE INTO", 1)
	}
	return tx.Exec(q, vals...).Error
}

func insertID(tx *gorm.DB, q string, args ...any) (int, error) {
	if err := tx.Exec(q, args...).Error; err != nil {
		return 0, err
	}
	var id int
	if err := tx.Raw("SELECT LAST_INSERT_ID()").Scan(&id).Error; err != nil {
		return 0, err
	}
	return id, nil
}

func orderedColumns(tx *gorm.DB, table string) ([]string, error) {
	live, err := liveColumns(tx, table)
	if err != nil {
		return nil, err
	}
	var cols []string
	for _, c := range live {
		cols = append(cols, c)
	}
	sort.Strings(cols)
	return cols, nil
}

// copyRow copies a loottable or lootdrop row under a new id, its name
// suffixed
func copyRow(tx *gorm.DB, table string, id int, suffix string) (int, error) {
	cols, err := orderedColumns(tx, table)
	if err != nil {
		return 0, err
	}
	var names, sel []string
	for _, c := range cols {
		switch strings.ToLower(c) {
		case "id":
			continue
		case "name":
			names = append(names, "`"+c+"`")
			sel = append(sel, "LEFT(CONCAT(`"+c+"`, ?), 255)")
		default:
			names = append(names, "`"+c+"`")
			sel = append(sel, "`"+c+"`")
		}
	}
	return insertID(tx, "INSERT INTO `"+table+"` ("+strings.Join(names, ", ")+") SELECT "+strings.Join(sel, ", ")+" FROM `"+table+"` WHERE id = ?", suffix, id)
}

// copyChildren copies a loot table's or drop group's rows to the copy
func copyChildren(tx *gorm.DB, table, parent string, from, to int) error {
	cols, err := orderedColumns(tx, table)
	if err != nil {
		return err
	}
	var names, sel []string
	for _, c := range cols {
		names = append(names, "`"+c+"`")
		if strings.EqualFold(c, parent) {
			sel = append(sel, "?")
		} else {
			sel = append(sel, "`"+c+"`")
		}
	}
	return tx.Exec("INSERT INTO `"+table+"` ("+strings.Join(names, ", ")+") SELECT "+strings.Join(sel, ", ")+" FROM `"+table+"` WHERE `"+parent+"` = ?", to, from).Error
}
