package zoneloot

import (
	"fmt"
	"sort"
	"strings"

	"gorm.io/gorm"
)

// A zone's loot, as the server builds it: spawn points (spawn2) pick from
// spawn groups, whose entries name NPCs; an NPC's loot table rolls drop
// groups (lootdrops), which roll items.

type Spawn struct {
	SpawngroupID int     `json:"spawngroup_id"`
	Name         string  `json:"name"`
	Points       int     `json:"points"` // spawn points using the group
	Chance       float64 `json:"chance"` // this NPC's share of the group, 0-1
}

type NPC struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Level       int     `json:"level"`
	MaxLevel    int     `json:"maxlevel"`
	Race        int     `json:"race"`
	Gender      int     `json:"gender"`
	Class       int     `json:"class"`
	LoottableID int     `json:"loottable_id"`
	RareSpawn   bool    `json:"rare_spawn"`
	RaidTarget  bool    `json:"raid_target"`
	Role        string  `json:"role"` // raid, named or trash
	Up          float64 `json:"up"`   // average spawned at once, over its spawn points
	Spawns      []Spawn `json:"spawns"`
	Drops       []Drop  `json:"drops"` // expected per kill, most first
}

type Drop struct {
	ItemID   int     `json:"item_id"`
	Expected float64 `json:"expected"`
}

type Table struct {
	ID       int          `json:"id"`
	Name     string       `json:"name"`
	MinCash  int          `json:"mincash"`
	MaxCash  int          `json:"maxcash"`
	Entries  []TableEntry `json:"entries"`
	SharedBy []Outside    `json:"shared_by"` // NPCs outside this zone using it
}

type Dropgroup struct {
	ID       int         `json:"id"`
	Name     string      `json:"name"`
	Entries  []DropEntry `json:"entries"`
	SharedBy []Outside   `json:"shared_by"` // loot tables outside this zone using it
}

type Outside struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Zone string `json:"zone,omitempty"`
}

type Item struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Icon     int    `json:"icon"`
	ItemType int    `json:"itemtype"`
	Slots    int    `json:"slots"`
	ReqLevel int    `json:"reqlevel"`
	RecLevel int    `json:"reclevel"`
	AC       int    `json:"ac"`
	HP       int    `json:"hp"`
	Mana     int    `json:"mana"`
	Damage   int    `json:"damage"`
	Delay    int    `json:"delay"`
	Classes  int    `json:"classes"`
	Magic    bool   `json:"magic"`
	NoDrop   bool   `json:"nodrop"`
	Lore     bool   `json:"lore"`
}

type ZoneLoot struct {
	Zone             string              `json:"zone"`
	Version          int                 `json:"version"`  // the zone version shown
	Versions         []int               `json:"versions"` // versions the zone has spawns or a zone row for
	LongName         string              `json:"long_name"`
	GlobalMultiplier int                 `json:"global_multiplier"`
	NPCs             []NPC               `json:"npcs"`
	Tables           map[int]*Table      `json:"tables"`
	Dropgroups       map[int]*Dropgroup  `json:"dropgroups"`
	Items            map[int]*Item       `json:"items"`
	Spawngroups      map[int]*Spawngroup `json:"spawngroups"`
}

// scope is the rows a zone's loot is made of, by key
type scope struct {
	spawn2, groups, npcs, tables, drops []int
}

// spawnsIn is the spawn2 rows the server loads for a zone version (zone/
// spawn2.cpp: version = N or -1), or every version's when version is nil
func spawnsIn(zone string, version *int) (string, []any) {
	if version == nil {
		return "zone = ?", []any{zone}
	}
	return "zone = ? AND (version = ? OR version = -1)", []any{zone, *version}
}

func zoneScope(db *gorm.DB, zone string, version *int) (scope, error) {
	var s scope
	where, args := spawnsIn(zone, version)
	steps := []struct {
		dst  *[]int
		sql  string
		args func() []any
	}{
		{&s.spawn2, "SELECT id FROM spawn2 WHERE " + where, func() []any { return args }},
		{&s.groups, "SELECT DISTINCT spawngroupID FROM spawn2 WHERE " + where, func() []any { return args }},
		{&s.npcs, "SELECT DISTINCT npcID FROM spawnentry WHERE spawngroupID IN ?", func() []any { return []any{nonEmpty(s.groups)} }},
		{&s.tables, "SELECT DISTINCT loottable_id FROM npc_types WHERE id IN ? AND loottable_id > 0", func() []any { return []any{nonEmpty(s.npcs)} }},
		{&s.drops, "SELECT DISTINCT lootdrop_id FROM loottable_entries WHERE loottable_id IN ?", func() []any { return []any{nonEmpty(s.tables)} }},
	}
	for _, st := range steps {
		if err := db.Raw(st.sql, st.args()...).Scan(st.dst).Error; err != nil {
			return s, err
		}
	}
	return s, nil
}

// Versions is the versions a zone has spawn points or a zone row for
func Versions(db *gorm.DB, zone string) ([]int, error) {
	var vs []int
	err := db.Raw("SELECT version FROM spawn2 WHERE zone = ? AND version >= 0 UNION SELECT version FROM zone WHERE short_name = ? AND version >= 0", zone, zone).Scan(&vs).Error
	sort.Ints(vs)
	return vs, err
}

// nonEmpty keeps "IN ?" valid for no keys: id 0 matches no row
func nonEmpty(ids []int) []int {
	if len(ids) == 0 {
		return []int{0}
	}
	return ids
}

// Load reads a zone version's spawns, NPCs, loot tables, drop groups and
// items, and works out what each NPC drops per kill
func Load(db *gorm.DB, zone string, version int) (*ZoneLoot, error) {
	zone = strings.ToLower(strings.TrimSpace(zone))
	out := &ZoneLoot{Zone: zone, Version: version, GlobalMultiplier: 1, Tables: map[int]*Table{}, Dropgroups: map[int]*Dropgroup{}, Items: map[int]*Item{}, Spawngroups: map[int]*Spawngroup{}}
	var names []struct{ LongName string }
	if err := db.Raw("SELECT long_name FROM zone WHERE short_name = ? ORDER BY version = ? DESC, version LIMIT 1", zone, version).Scan(&names).Error; err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("there is no zone %q", zone)
	}
	out.LongName = names[0].LongName
	versions, err := Versions(db, zone)
	if err != nil {
		return nil, err
	}
	out.Versions = versions
	if !containsInt(out.Versions, version) {
		out.Versions = append(out.Versions, version)
		sort.Ints(out.Versions)
	}

	if hasTable(db, "rule_values") {
		var rule []struct{ RuleValue string }
		db.Raw("SELECT rule_value FROM rule_values WHERE rule_name = 'Zone:GlobalLootMultiplier' AND ruleset_id = 1 LIMIT 1").Scan(&rule)
		if len(rule) == 1 {
			fmt.Sscan(rule[0].RuleValue, &out.GlobalMultiplier)
		}
	}

	s, err := zoneScope(db, zone, &version)
	if err != nil {
		return nil, err
	}
	out.NPCs = []NPC{}
	if len(s.spawn2) == 0 {
		return out, nil
	}

	// spawn groups: points and each NPC's share
	var points []struct{ SpawngroupID, Points int }
	where, args := spawnsIn(zone, &version)
	db.Raw("SELECT spawngroupID AS spawngroup_id, COUNT(*) AS points FROM spawn2 WHERE "+where+" GROUP BY spawngroupID", args...).Scan(&points)
	var groups []struct {
		ID   int
		Name string
	}
	db.Raw("SELECT id, name FROM spawngroup WHERE id IN ?", s.groups).Scan(&groups)
	var entries []struct {
		SpawngroupID, NpcID int
		Chance              float64
	}
	db.Raw("SELECT spawngroupID AS spawngroup_id, npcID AS npc_id, chance FROM spawnentry WHERE spawngroupID IN ?", s.groups).Scan(&entries)
	groupName, groupPoints, groupTotal := map[int]string{}, map[int]int{}, map[int]float64{}
	for _, g := range groups {
		groupName[g.ID] = g.Name
	}
	for _, p := range points {
		groupPoints[p.SpawngroupID] = p.Points
	}
	for _, e := range entries {
		groupTotal[e.SpawngroupID] += e.Chance
	}
	spawns := map[int][]Spawn{}
	for _, e := range entries {
		share := 0.0
		if groupTotal[e.SpawngroupID] > 0 {
			share = e.Chance / groupTotal[e.SpawngroupID]
		}
		spawns[e.NpcID] = append(spawns[e.NpcID], Spawn{SpawngroupID: e.SpawngroupID, Name: groupName[e.SpawngroupID], Points: groupPoints[e.SpawngroupID], Chance: share})
	}

	// NPCs
	var npcs []struct {
		ID, Level, Maxlevel, Race, Gender, Class, LoottableID int
		Name                                                  string
		RareSpawn, RaidTarget                                 int
	}
	if err := db.Raw("SELECT id, name, level, maxlevel, race, gender, class, loottable_id, rare_spawn, raid_target FROM npc_types WHERE id IN ?", nonEmpty(s.npcs)).Scan(&npcs).Error; err != nil {
		return nil, err
	}

	// loot tables and their rows
	var tables []struct {
		ID, Mincash, Maxcash int
		Name                 string
	}
	db.Raw("SELECT id, name, mincash, maxcash FROM loottable WHERE id IN ?", nonEmpty(s.tables)).Scan(&tables)
	for _, t := range tables {
		out.Tables[t.ID] = &Table{ID: t.ID, Name: t.Name, MinCash: t.Mincash, MaxCash: t.Maxcash, Entries: []TableEntry{}, SharedBy: []Outside{}}
	}
	var lte []struct {
		LoottableID int
		TableEntry
	}
	db.Raw("SELECT loottable_id, lootdrop_id, multiplier, droplimit, mindrop, probability FROM loottable_entries WHERE loottable_id IN ?", nonEmpty(s.tables)).Scan(&lte)
	for _, e := range lte {
		if t := out.Tables[e.LoottableID]; t != nil {
			t.Entries = append(t.Entries, e.TableEntry)
		}
	}

	// drop groups and their items
	var drops []struct {
		ID   int
		Name string
	}
	db.Raw("SELECT id, name FROM lootdrop WHERE id IN ?", nonEmpty(s.drops)).Scan(&drops)
	for _, d := range drops {
		out.Dropgroups[d.ID] = &Dropgroup{ID: d.ID, Name: d.Name, Entries: []DropEntry{}, SharedBy: []Outside{}}
	}
	var lde []struct {
		LootdropID int
		DropEntry
	}
	db.Raw("SELECT lootdrop_id, item_id, chance, multiplier, item_charges AS charges, equip_item, npc_min_level, npc_max_level FROM lootdrop_entries WHERE lootdrop_id IN ?", nonEmpty(s.drops)).Scan(&lde)
	dropEntries := map[int][]DropEntry{}
	itemIDs := map[int]bool{}
	for _, e := range lde {
		if d := out.Dropgroups[e.LootdropID]; d != nil {
			d.Entries = append(d.Entries, e.DropEntry)
		}
		dropEntries[e.LootdropID] = append(dropEntries[e.LootdropID], e.DropEntry)
		itemIDs[e.ItemID] = true
	}

	// what's shared with the rest of the world
	inZone := map[int]bool{}
	for _, id := range s.npcs {
		inZone[id] = true
	}
	var tableUsers []struct {
		LoottableID, ID int
		Name, Zone      string
		Version         int
	}
	db.Raw(`SELECT DISTINCT n.loottable_id, n.id, n.name, COALESCE(s2.zone, '') AS zone, COALESCE(s2.version, 0) AS version FROM npc_types n
		LEFT JOIN spawnentry se ON se.npcID = n.id LEFT JOIN spawn2 s2 ON s2.spawngroupID = se.spawngroupID
		WHERE n.loottable_id IN ? ORDER BY n.id, zone = ? DESC, version`, nonEmpty(s.tables), zone).Scan(&tableUsers)
	listed := map[int]bool{}
	for _, u := range tableUsers {
		if t := out.Tables[u.LoottableID]; t != nil && !inZone[u.ID] && !listed[u.ID] {
			listed[u.ID] = true
			where := u.Zone
			if u.Version != 0 && where != "" {
				where = fmt.Sprintf("%v v%v", u.Zone, u.Version)
			}
			t.SharedBy = append(t.SharedBy, Outside{ID: u.ID, Name: u.Name, Zone: where})
		}
	}
	inTables := map[int]bool{}
	for _, id := range s.tables {
		inTables[id] = true
	}
	var dropUsers []struct {
		LootdropID, ID int
		Name           string
	}
	db.Raw(`SELECT e.lootdrop_id, t.id, t.name FROM loottable_entries e JOIN loottable t ON t.id = e.loottable_id
		WHERE e.lootdrop_id IN ?`, nonEmpty(s.drops)).Scan(&dropUsers)
	for _, u := range dropUsers {
		if d := out.Dropgroups[u.LootdropID]; d != nil && !inTables[u.ID] {
			d.SharedBy = append(d.SharedBy, Outside{ID: u.ID, Name: u.Name})
		}
	}

	// items
	ids := []int{0}
	for id := range itemIDs {
		ids = append(ids, id)
	}
	var items []Item
	if err := db.Raw(`SELECT id, Name AS name, icon, itemtype AS item_type, slots, reqlevel AS req_level, reclevel AS rec_level, ac, hp, mana, damage, delay, classes,
		magic, nodrop = 0 AS no_drop, (lore LIKE '*%' OR loregroup <> 0) AS lore FROM items WHERE id IN ?`, ids).Scan(&items).Error; err != nil {
		return nil, err
	}
	for i := range items {
		out.Items[items[i].ID] = &items[i]
	}

	for _, n := range npcs {
		npc := NPC{ID: n.ID, Name: n.Name, Level: n.Level, MaxLevel: n.Maxlevel, Race: n.Race, Gender: n.Gender, Class: n.Class,
			LoottableID: n.LoottableID, RareSpawn: n.RareSpawn != 0, RaidTarget: n.RaidTarget != 0, Spawns: spawns[n.ID], Drops: []Drop{}}
		switch {
		case npc.RaidTarget:
			npc.Role = "raid"
		case npc.RareSpawn || isNamed(n.Name):
			npc.Role = "named"
		default:
			npc.Role = "trash"
		}
		for _, sp := range npc.Spawns {
			npc.Up += float64(sp.Points) * sp.Chance
		}
		if t := out.Tables[n.LoottableID]; t != nil {
			for item, e := range TableExpected(t.Entries, dropEntries, n.Level, out.GlobalMultiplier) {
				if e > 0 {
					npc.Drops = append(npc.Drops, Drop{ItemID: item, Expected: e})
				}
			}
			sort.Slice(npc.Drops, func(i, j int) bool { return npc.Drops[i].Expected > npc.Drops[j].Expected })
		}
		out.NPCs = append(out.NPCs, npc)
	}
	sort.Slice(out.NPCs, func(i, j int) bool {
		if out.NPCs[i].Level != out.NPCs[j].Level {
			return out.NPCs[i].Level > out.NPCs[j].Level
		}
		return out.NPCs[i].Name < out.NPCs[j].Name
	})
	if err := loadSpawns(db, zone, version, s, out); err != nil {
		return nil, err
	}
	return out, nil
}

func containsInt(list []int, v int) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// isNamed says whether a name is a named NPC's: marked # or, by EQ's
// convention, capitalised (King_Gragnar), where common ones are lower case
// (a_kobold)
func isNamed(name string) bool {
	if strings.HasPrefix(name, "#") {
		return true
	}
	return name != "" && name[0] >= 'A' && name[0] <= 'Z'
}

func hasTable(db *gorm.DB, table string) bool {
	var n int64
	db.Raw("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?", table).Scan(&n)
	return n > 0
}
