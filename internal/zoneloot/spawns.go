package zoneloot

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

// A zone version's spawns: the spawn points the server loads for it, the
// spawn groups they pick from, and those groups' NPCs with their chances.

type SpawnPoint struct {
	ID           int     `json:"id"`
	SpawngroupID int     `json:"spawngroup_id"`
	Version      int     `json:"version"` // -1: every version
	X            float64 `json:"x"`
	Y            float64 `json:"y"`
	Z            float64 `json:"z"`
	Heading      float64 `json:"heading"`
	Respawntime  int     `json:"respawntime"`
	Variance     int     `json:"variance"`
	Pathgrid     int     `json:"pathgrid"`
}

type SpawnEntry struct {
	NpcID  int `json:"npc_id"`
	Chance int `json:"chance"`
}

type Spawngroup struct {
	ID           int          `json:"id"`
	Name         string       `json:"name"`
	SpawnLimit   int          `json:"spawn_limit"`
	Dist         float64      `json:"dist"`
	MinX         float64      `json:"min_x"`
	MaxX         float64      `json:"max_x"`
	MinY         float64      `json:"min_y"`
	MaxY         float64      `json:"max_y"`
	Delay        int          `json:"delay"`
	Mindelay     int          `json:"mindelay"`
	Despawn      int          `json:"despawn"`
	DespawnTimer int          `json:"despawn_timer"`
	Entries      []SpawnEntry `json:"entries" gorm:"-"`
	Points       []SpawnPoint `json:"points" gorm:"-"`
	SharedBy     []Outside    `json:"shared_by" gorm:"-"` // spawn points outside this zone version using it
}

// otherVersions says whether a zone has versions besides this one: a
// version -1 spawn point is then in them too
func otherVersions(db *gorm.DB, zone string, version int) bool {
	vs, _ := Versions(db, zone)
	for _, v := range vs {
		if v != version {
			return true
		}
	}
	return false
}

// spawnOutside is the SQL for spawn2 rows outside a zone version: other
// zones, other versions, and version -1 rows when the zone has others
func spawnOutside(zone string, version int, others bool) (string, []any) {
	if others {
		return "NOT (zone = ? AND version = ?)", []any{zone, version}
	}
	return "NOT (zone = ? AND version IN (?, -1))", []any{zone, version}
}

func loadSpawns(db *gorm.DB, zone string, version int, s scope, out *ZoneLoot) error {
	out.Spawngroups = map[int]*Spawngroup{}
	var groups []Spawngroup
	if err := db.Raw(`SELECT id, name, spawn_limit, dist, min_x, max_x, min_y, max_y, delay, mindelay, despawn, despawn_timer
		FROM spawngroup WHERE id IN ?`, nonEmpty(s.groups)).Scan(&groups).Error; err != nil {
		return err
	}
	for i := range groups {
		g := groups[i]
		g.Entries, g.Points, g.SharedBy = []SpawnEntry{}, []SpawnPoint{}, []Outside{}
		out.Spawngroups[g.ID] = &g
	}
	var entries []struct {
		SpawngroupID int
		SpawnEntry
	}
	db.Raw("SELECT spawngroupID AS spawngroup_id, npcID AS npc_id, chance FROM spawnentry WHERE spawngroupID IN ? ORDER BY chance DESC, npcID", nonEmpty(s.groups)).Scan(&entries)
	for _, e := range entries {
		if g := out.Spawngroups[e.SpawngroupID]; g != nil {
			g.Entries = append(g.Entries, e.SpawnEntry)
		}
	}
	where, args := spawnsIn(zone, &version)
	var points []SpawnPoint
	db.Raw(`SELECT id, spawngroupID AS spawngroup_id, version, x, y, z, heading, respawntime, variance, pathgrid
		FROM spawn2 WHERE `+where+" ORDER BY id", args...).Scan(&points)
	for _, p := range points {
		if g := out.Spawngroups[p.SpawngroupID]; g != nil {
			g.Points = append(g.Points, p)
		}
	}
	others := otherVersions(db, zone, version)
	ow, oargs := spawnOutside(zone, version, others)
	var outside []struct {
		ID, SpawngroupID, Version int
		Zone                      string
	}
	db.Raw("SELECT id, spawngroupID AS spawngroup_id, version, zone FROM spawn2 WHERE spawngroupID IN ? AND "+ow+" ORDER BY id",
		append([]any{nonEmpty(s.groups)}, oargs...)...).Scan(&outside)
	for _, o := range outside {
		if g := out.Spawngroups[o.SpawngroupID]; g != nil {
			where := o.Zone
			switch {
			case o.Version == -1:
				where += " (every version)"
			case o.Version != 0 || o.Zone == zone:
				where = fmt.Sprintf("%v v%d", o.Zone, o.Version)
			}
			g.SharedBy = append(g.SharedBy, Outside{ID: o.ID, Name: fmt.Sprintf("spawn point %d", o.ID), Zone: where})
		}
	}
	return nil
}

// --- edits ---

func (e *editor) ownGroup(id int, everywhere bool) error {
	if !set(e.sc.groups)[id] {
		return fmt.Errorf("spawn group %d has no spawn point in %v", id, e.where())
	}
	if everywhere {
		return nil
	}
	ow, args := spawnOutside(e.zone, e.version, e.others)
	if e.count("SELECT COUNT(*) FROM spawn2 WHERE spawngroupID = ? AND "+ow, append([]any{id}, args...)...) > 0 {
		return fmt.Errorf("spawn group %d is also used by spawn points outside %v: copy it first, or change it everywhere", id, e.where())
	}
	return nil
}

// ownPoint is a spawn point of the zone version; one in every version (-1)
// of a zone with others changes only when the edit says everywhere
func (e *editor) ownPoint(id int, everywhere bool) error {
	var p []struct {
		Zone    string
		Version int
	}
	e.tx.Raw("SELECT zone, version FROM spawn2 WHERE id = ?", id).Scan(&p)
	if len(p) == 0 || p[0].Zone != e.zone || (p[0].Version != e.version && p[0].Version != -1) {
		return fmt.Errorf("spawn point %d is not in %v", id, e.where())
	}
	if p[0].Version == -1 && e.others && !everywhere {
		return fmt.Errorf("spawn point %d is in every version of %v: change it everywhere, or not at all", id, e.zone)
	}
	return nil
}

func floatRange(name string, v *float64, lo, hi float64) error {
	if v != nil && (*v < lo || *v > hi) {
		return fmt.Errorf("%v must be from %v to %v", name, lo, hi)
	}
	return nil
}

func (e *editor) applySpawn(ed Edit) (bool, error) {
	tx := e.tx
	switch ed.Op {
	case "spawn2":
		if err := firstErr(e.ownPoint(ed.SpawnID, ed.Everywhere), inRange("respawn time", ed.Respawntime, 0, 2147483647),
			inRange("variance", ed.Variance, 0, 2147483647), inRange("grid", ed.Pathgrid, 0, 2147483647),
			floatRange("heading", ed.Heading, 0, 512)); err != nil {
			return true, err
		}
		if ed.SpawngroupID != nil && !e.exists("spawngroup", *ed.SpawngroupID) {
			return true, fmt.Errorf("there is no spawn group %d", *ed.SpawngroupID)
		}
		var c columns
		c.add("spawngroupID", ed.SpawngroupID)
		c.add("respawntime", ed.Respawntime)
		c.add("variance", ed.Variance)
		c.add("pathgrid", ed.Pathgrid)
		c.add("x", ed.X)
		c.add("y", ed.Y)
		c.add("z", ed.Z)
		c.add("heading", ed.Heading)
		if len(c.names) == 0 {
			return true, nil
		}
		return true, tx.Exec("UPDATE spawn2 SET "+c.assignments()+" WHERE id = ?", append(c.vals, ed.SpawnID)...).Error

	case "spawngroup":
		id := 0
		if ed.SpawngroupID != nil {
			id = *ed.SpawngroupID
		}
		if err := firstErr(e.ownGroup(id, ed.Everywhere), inRange("spawn limit", ed.SpawnLimit, 0, 127), inRange("delay", ed.Delay, 0, 2147483647),
			inRange("despawn", ed.Despawn, 0, 4), inRange("despawn timer", ed.DespawnTimer, 0, 2147483647), floatRange("roam distance", ed.Dist, 0, 100000)); err != nil {
			return true, err
		}
		if ed.Name != nil && strings.TrimSpace(*ed.Name) == "" {
			return true, fmt.Errorf("spawn group %d needs a name", id)
		}
		var c columns
		c.add("name", ed.Name)
		c.add("spawn_limit", ed.SpawnLimit)
		c.add("delay", ed.Delay)
		c.add("despawn", ed.Despawn)
		c.add("despawn_timer", ed.DespawnTimer)
		c.add("dist", ed.Dist)
		c.add("min_x", ed.MinX)
		c.add("max_x", ed.MaxX)
		c.add("min_y", ed.MinY)
		c.add("max_y", ed.MaxY)
		if len(c.names) == 0 {
			return true, nil
		}
		return true, tx.Exec("UPDATE spawngroup SET "+c.assignments()+" WHERE id = ?", append(c.vals, id)...).Error

	case "spawnentry":
		id := 0
		if ed.SpawngroupID != nil {
			id = *ed.SpawngroupID
		}
		if err := firstErr(e.ownGroup(id, ed.Everywhere), percent("chance", ed.Chance)); err != nil {
			return true, err
		}
		if ed.Remove {
			return true, tx.Exec("DELETE FROM spawnentry WHERE spawngroupID = ? AND npcID = ?", id, ed.NpcID).Error
		}
		if !e.exists("npc_types", ed.NpcID) {
			return true, fmt.Errorf("there is no NPC %d", ed.NpcID)
		}
		var c columns
		if ed.Chance != nil {
			chance := int(*ed.Chance + 0.5)
			c.add("chance", &chance)
		}
		return true, upsert(tx, "spawnentry", []string{"spawngroupID", "npcID"}, []any{id, ed.NpcID}, c)

	case "copy_spawngroup":
		id := 0
		if ed.SpawngroupID != nil {
			id = *ed.SpawngroupID
		}
		if !set(e.sc.groups)[id] {
			return true, fmt.Errorf("spawn group %d has no spawn point in %v", id, e.where())
		}
		where, args := spawnsIn(e.zone, &e.version)
		var users []int
		tx.Raw("SELECT id FROM spawn2 WHERE spawngroupID = ? AND "+where, append([]any{id}, args...)...).Scan(&users)
		points, err := subset(users, ed.SpawnIDs, "spawn point", fmt.Sprintf("using spawn group %d in %v", id, e.where()))
		if err != nil {
			return true, err
		}
		for _, p := range points {
			if err := e.ownPoint(p, ed.Everywhere); err != nil {
				return true, err
			}
		}
		nid, err := copyRow(tx, "spawngroup", id, fmt.Sprintf("_copy%d", e.at))
		if err != nil {
			return true, err
		}
		if err := tx.Exec("UPDATE spawngroup SET name = CONCAT(LEFT((SELECT name FROM (SELECT name FROM spawngroup WHERE id = ?) s), 180), '_', ?) WHERE id = ?", id, nid, nid).Error; err != nil {
			return true, err
		}
		if err := copyChildren(tx, "spawnentry", "spawngroupID", id, nid); err != nil {
			return true, err
		}
		e.made("spawngroup", nid)
		return true, tx.Exec("UPDATE spawn2 SET spawngroupID = ? WHERE id IN ?", nid, points).Error

	case "new_spawngroup":
		for _, p := range ed.SpawnIDs {
			if err := e.ownPoint(p, ed.Everywhere); err != nil {
				return true, err
			}
		}
		named := ed.Name != nil && strings.TrimSpace(*ed.Name) != ""
		name := fmt.Sprintf("%v_new_%d", e.zone, time.Now().UnixNano()) // named for its id once it has one
		if named {
			name = strings.TrimSpace(*ed.Name)
		}
		nid, err := insertID(tx, "INSERT INTO spawngroup (name) VALUES (?)", name)
		if err != nil {
			return true, err
		}
		if !named {
			if err := tx.Exec("UPDATE spawngroup SET name = ? WHERE id = ?", fmt.Sprintf("%v_%d", e.zone, nid), nid).Error; err != nil {
				return true, err
			}
		}
		e.made("spawngroup", nid)
		if len(ed.SpawnIDs) > 0 {
			return true, tx.Exec("UPDATE spawn2 SET spawngroupID = ? WHERE id IN ?", nid, ed.SpawnIDs).Error
		}
		return true, nil
	}
	return false, nil
}

type NPCHit struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Level       int    `json:"level"`
	MaxLevel    int    `json:"maxlevel" gorm:"column:maxlevel"`
	Race        int    `json:"race"`
	Gender      int    `json:"gender"`
	Class       int    `json:"class"`
	LoottableID int    `json:"loottable_id"`
}

// NPCSearch finds NPCs to add to a spawn group, by name or id
func NPCSearch(db *gorm.DB, q string) ([]NPCHit, error) {
	out := []NPCHit{}
	q = strings.TrimSpace(q)
	if q == "" {
		return out, nil
	}
	like := "%" + strings.ReplaceAll(q, " ", "_") + "%"
	err := db.Raw(`SELECT id, name, level, maxlevel, race, gender, class, loottable_id FROM npc_types
		WHERE id = ? OR name LIKE ? ORDER BY id = ? DESC, name LIKE ? DESC, LENGTH(name), id LIMIT 25`, q, like, q, strings.ReplaceAll(q, " ", "_")+"%").Scan(&out).Error
	return out, err
}
