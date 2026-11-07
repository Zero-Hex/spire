package zoneloot

import (
	"compress/gzip"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
)

// A snapshot keeps every row a zone's loot is made of, as it was: the zone's
// spawn points, their spawn groups and entries, the NPCs those name, the
// NPCs' loot tables and table rows, and the drop groups those use with their
// items. Restoring writes them back.

var tableKeys = map[string][]string{
	"spawn2":            {"id"},
	"spawngroup":        {"id"},
	"spawnentry":        {"spawngroupID", "npcID"},
	"npc_types":         {"id"},
	"loottable":         {"id"},
	"loottable_entries": {"loottable_id", "lootdrop_id"},
	"lootdrop":          {"id"},
	"lootdrop_entries":  {"lootdrop_id", "item_id"},
}

// the order rows are written back in
var tableOrder = []string{"spawn2", "spawngroup", "spawnentry", "npc_types", "loottable", "loottable_entries", "lootdrop", "lootdrop_entries"}

type Rows struct {
	Columns []string    `json:"columns"`
	Rows    [][]*string `json:"rows"`
}

type Snapshot struct {
	ID     string           `json:"id"`
	Zone   string           `json:"zone"`
	Note   string           `json:"note"`
	Auto   bool             `json:"auto,omitempty"` // taken before a change; only the newest are kept
	Taken  int64            `json:"taken"`
	Counts map[string]int   `json:"counts"`
	Tables map[string]*Rows `json:"tables,omitempty"`
}

type Snapshots struct {
	dir string
}

func NewSnapshots(dir string) *Snapshots {
	return &Snapshots{dir: dir}
}

var zoneName = regexp.MustCompile(`^[a-z0-9_]+$`)
var snapshotID = regexp.MustCompile(`^\d{8}-\d{6}(-\d+)?$`)

func (s *Snapshots) zoneDir(zone string) (string, error) {
	if s.dir == "" {
		return "", errors.New("Spire does not know the server folder to keep snapshots in")
	}
	if !zoneName.MatchString(zone) {
		return "", fmt.Errorf("%q is not a zone short name", zone)
	}
	return filepath.Join(s.dir, zone), nil
}

func dump(db *gorm.DB, table, where string, args ...any) (*Rows, error) {
	rows, err := db.Raw("SELECT * FROM `"+table+"` WHERE "+where, args...).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	out := &Rows{Columns: cols, Rows: [][]*string{}}
	raw := make([]sql.RawBytes, len(cols))
	ptrs := make([]any, len(cols))
	for i := range raw {
		ptrs[i] = &raw[i]
	}
	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := make([]*string, len(cols))
		for i, b := range raw {
			if b != nil {
				v := string(b)
				row[i] = &v
			}
		}
		out.Rows = append(out.Rows, row)
	}
	return out, rows.Err()
}

// read is the rows a zone's loot is made of now
func read(db *gorm.DB, zone string) (map[string]*Rows, error) {
	sc, err := zoneScope(db, zone, nil) // every version
	if err != nil {
		return nil, err
	}
	where := map[string]struct {
		sql string
		arg any
	}{
		"spawn2":            {"zone = ?", zone},
		"spawngroup":        {"id IN ?", nonEmpty(sc.groups)},
		"spawnentry":        {"spawngroupID IN ?", nonEmpty(sc.groups)},
		"npc_types":         {"id IN ?", nonEmpty(sc.npcs)},
		"loottable":         {"id IN ?", nonEmpty(sc.tables)},
		"loottable_entries": {"loottable_id IN ?", nonEmpty(sc.tables)},
		"lootdrop":          {"id IN ?", nonEmpty(sc.drops)},
		"lootdrop_entries":  {"lootdrop_id IN ?", nonEmpty(sc.drops)},
	}
	out := map[string]*Rows{}
	for _, t := range tableOrder {
		r, err := dump(db, t, where[t].sql, where[t].arg)
		if err != nil {
			return nil, fmt.Errorf("%v: %v", t, err)
		}
		out[t] = r
	}
	return out, nil
}

// how many automatic snapshots a zone keeps
const keepAuto = 20

// Take saves a snapshot of a zone's loot rows
func (s *Snapshots) Take(db *gorm.DB, zone, note string) (*Snapshot, error) {
	return s.take(db, zone, note, false)
}

// TakeAuto saves a snapshot before a change, and drops the oldest automatic
// ones past keepAuto
func (s *Snapshots) TakeAuto(db *gorm.DB, zone, note string) (*Snapshot, error) {
	snap, err := s.take(db, zone, note, true)
	if err != nil {
		return nil, err
	}
	list, _ := s.List(zone)
	kept := 0
	for _, o := range list {
		if !o.Auto {
			continue
		}
		if kept++; kept > keepAuto {
			s.Delete(zone, o.ID)
		}
	}
	return snap, nil
}

func (s *Snapshots) take(db *gorm.DB, zone, note string, auto bool) (*Snapshot, error) {
	dir, err := s.zoneDir(zone)
	if err != nil {
		return nil, err
	}
	tables, err := read(db, zone)
	if err != nil {
		return nil, err
	}
	if len(tables["spawn2"].Rows) == 0 {
		return nil, fmt.Errorf("%v has no spawn points to keep", zone)
	}
	now := time.Now()
	snap := &Snapshot{ID: now.Format("20060102-150405"), Zone: zone, Note: strings.TrimSpace(note), Auto: auto, Taken: now.Unix(), Counts: map[string]int{}, Tables: tables}
	for t, r := range tables {
		snap.Counts[t] = len(r.Rows)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	// two in one second get their own names
	for i := 2; ; i++ {
		if _, err := os.Stat(filepath.Join(dir, snap.ID+".json.gz")); os.IsNotExist(err) {
			break
		}
		snap.ID = fmt.Sprintf("%s-%d", now.Format("20060102-150405"), i)
	}
	if err := writeSnapshot(filepath.Join(dir, snap.ID+".json.gz"), snap); err != nil {
		return nil, err
	}
	return snap, nil
}

func writeSnapshot(path string, snap *Snapshot) error {
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	zw := gzip.NewWriter(f)
	if err := json.NewEncoder(zw).Encode(snap); err != nil {
		f.Close()
		return err
	}
	if err := zw.Close(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Get reads a snapshot
func (s *Snapshots) Get(zone, id string) (*Snapshot, error) {
	dir, err := s.zoneDir(zone)
	if err != nil {
		return nil, err
	}
	if !snapshotID.MatchString(id) {
		return nil, fmt.Errorf("%q is not a snapshot", id)
	}
	f, err := os.Open(filepath.Join(dir, id+".json.gz"))
	if err != nil {
		return nil, fmt.Errorf("there is no snapshot %v of %v", id, zone)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	var snap Snapshot
	if err := json.NewDecoder(zr).Decode(&snap); err != nil {
		return nil, err
	}
	return &snap, nil
}

// List is a zone's snapshots, newest first, without their rows
func (s *Snapshots) List(zone string) ([]Snapshot, error) {
	dir, err := s.zoneDir(zone)
	if err != nil {
		return nil, err
	}
	names, _ := filepath.Glob(filepath.Join(dir, "*.json.gz"))
	out := []Snapshot{}
	for _, n := range names {
		snap, err := s.Get(zone, strings.TrimSuffix(filepath.Base(n), ".json.gz"))
		if err != nil {
			continue
		}
		snap.Tables = nil
		out = append(out, *snap)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out, nil
}

// Delete removes a snapshot
func (s *Snapshots) Delete(zone, id string) error {
	if _, err := s.Get(zone, id); err != nil {
		return err
	}
	dir, _ := s.zoneDir(zone)
	return os.Remove(filepath.Join(dir, id+".json.gz"))
}

// keys of a snapshot's parent rows: what its link rows hang under
type parents struct {
	spawn2, groups, tables, drops []string
}

func column(r *Rows, name string) []string {
	if r == nil {
		return nil
	}
	at := -1
	for i, c := range r.Columns {
		if strings.EqualFold(c, name) {
			at = i
		}
	}
	if at < 0 {
		return nil
	}
	seen, out := map[string]bool{}, []string{}
	for _, row := range r.Rows {
		if row[at] != nil && !seen[*row[at]] {
			seen[*row[at]] = true
			out = append(out, *row[at])
		}
	}
	return out
}

func union(a, b []string) []string {
	seen, out := map[string]bool{}, []string{}
	for _, v := range append(append([]string{}, a...), b...) {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		out = []string{"0"}
	}
	return out
}

func parentsOf(snap *Snapshot) parents {
	t := snap.Tables
	return parents{
		spawn2: union(column(t["spawn2"], "id"), nil),
		groups: union(column(t["spawngroup"], "id"), column(t["spawn2"], "spawngroupID")),
		tables: union(column(t["loottable"], "id"), column(t["loottable_entries"], "loottable_id")),
		drops:  union(column(t["lootdrop"], "id"), column(t["lootdrop_entries"], "lootdrop_id")),
	}
}

// current is the rows a restore would replace: the snapshot's own keys, the
// link rows under its parents, and spawn points now in the zone
func current(db *gorm.DB, snap *Snapshot) (map[string]*Rows, error) {
	p := parentsOf(snap)
	where := map[string]struct {
		sql  string
		args []any
	}{
		"spawn2":            {"zone = ? OR id IN ?", []any{snap.Zone, p.spawn2}},
		"spawngroup":        {"id IN ?", []any{p.groups}},
		"spawnentry":        {"spawngroupID IN ?", []any{p.groups}},
		"npc_types":         {"id IN ?", []any{union(column(snap.Tables["npc_types"], "id"), nil)}},
		"loottable":         {"id IN ?", []any{p.tables}},
		"loottable_entries": {"loottable_id IN ?", []any{p.tables}},
		"lootdrop":          {"id IN ?", []any{p.drops}},
		"lootdrop_entries":  {"lootdrop_id IN ?", []any{p.drops}},
	}
	out := map[string]*Rows{}
	for _, t := range tableOrder {
		r, err := dump(db, t, where[t].sql, where[t].args...)
		if err != nil {
			return nil, fmt.Errorf("%v: %v", t, err)
		}
		out[t] = r
	}
	return out, nil
}

type Change struct {
	Key     string   `json:"key"`
	Columns []string `json:"columns,omitempty"` // changed columns
}

type TableDiff struct {
	Changed []Change `json:"changed"` // put back as they were
	Added   []Change `json:"added"`   // gone since: written back
	Removed []Change `json:"removed"` // new since: deleted
}

func keyOf(cols []string, row []*string, keys []string) string {
	var parts []string
	for _, k := range keys {
		for i, c := range cols {
			if strings.EqualFold(c, k) {
				v := "NULL"
				if row[i] != nil {
					v = *row[i]
				}
				parts = append(parts, v)
			}
		}
	}
	return strings.Join(parts, "/")
}

func diffRows(table string, was, now *Rows) TableDiff {
	keys := tableKeys[table]
	d := TableDiff{Changed: []Change{}, Added: []Change{}, Removed: []Change{}}
	nowByKey := map[string][]*string{}
	for _, r := range now.Rows {
		nowByKey[keyOf(now.Columns, r, keys)] = r
	}
	nowCol := map[string]int{}
	for i, c := range now.Columns {
		nowCol[strings.ToLower(c)] = i
	}
	seen := map[string]bool{}
	for _, r := range was.Rows {
		k := keyOf(was.Columns, r, keys)
		seen[k] = true
		cur, ok := nowByKey[k]
		if !ok {
			d.Added = append(d.Added, Change{Key: k})
			continue
		}
		var changed []string
		for i, c := range was.Columns {
			j, ok := nowCol[strings.ToLower(c)]
			if !ok {
				continue
			}
			a, b := r[i], cur[j]
			if (a == nil) != (b == nil) || (a != nil && *a != *b) {
				changed = append(changed, c)
			}
		}
		if len(changed) > 0 {
			d.Changed = append(d.Changed, Change{Key: k, Columns: changed})
		}
	}
	for k := range nowByKey {
		if !seen[k] {
			d.Removed = append(d.Removed, Change{Key: k})
		}
	}
	sort.Slice(d.Removed, func(i, j int) bool { return d.Removed[i].Key < d.Removed[j].Key })
	return d
}

// Diff is what restoring a snapshot would change, table by table
func Diff(db *gorm.DB, snap *Snapshot) (map[string]TableDiff, error) {
	now, err := current(db, snap)
	if err != nil {
		return nil, err
	}
	out := map[string]TableDiff{}
	for _, t := range tableOrder {
		was := snap.Tables[t]
		if was == nil {
			was = &Rows{}
		}
		out[t] = diffRows(t, was, now[t])
	}
	return out, nil
}

func liveColumns(tx *gorm.DB, table string) (map[string]string, error) {
	var cols []string
	if err := tx.Raw("SELECT COLUMN_NAME FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?", table).Scan(&cols).Error; err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, c := range cols {
		out[strings.ToLower(c)] = c
	}
	return out, nil
}

// put writes rows back (REPLACE), in batches, with the columns the table
// still has
func put(tx *gorm.DB, table string, r *Rows) error {
	if r == nil || len(r.Rows) == 0 {
		return nil
	}
	live, err := liveColumns(tx, table)
	if err != nil {
		return err
	}
	var idx []int
	var names []string
	for i, c := range r.Columns {
		if name, ok := live[strings.ToLower(c)]; ok {
			idx = append(idx, i)
			names = append(names, "`"+name+"`")
		}
	}
	rowMarks := "(" + strings.TrimSuffix(strings.Repeat("?,", len(idx)), ",") + ")"
	batch := max(1, 20000/max(len(idx), 1))
	for start := 0; start < len(r.Rows); start += batch {
		end := min(start+batch, len(r.Rows))
		var marks []string
		var args []any
		for _, row := range r.Rows[start:end] {
			marks = append(marks, rowMarks)
			for _, i := range idx {
				if row[i] == nil {
					args = append(args, nil)
				} else {
					args = append(args, *row[i])
				}
			}
		}
		q := "REPLACE INTO `" + table + "` (" + strings.Join(names, ",") + ") VALUES " + strings.Join(marks, ",")
		if err := tx.Exec(q, args...).Error; err != nil {
			return fmt.Errorf("%v: %v", table, err)
		}
	}
	return nil
}

// Restore puts a zone's loot rows back as a snapshot has them, after
// snapshotting how they are now. Spawn points added since are deleted, and
// the link rows (spawn entries, loot table rows, drop group items) under the
// snapshot's spawn groups, loot tables and drop groups are replaced by its
// own. Rows outside the snapshot's keys are left alone.
func (s *Snapshots) Restore(db *gorm.DB, zone, id string) (*Snapshot, map[string]TableDiff, error) {
	snap, err := s.Get(zone, id)
	if err != nil {
		return nil, nil, err
	}
	diff, err := Diff(db, snap)
	if err != nil {
		return nil, nil, err
	}
	before, err := s.TakeAuto(db, zone, "before restoring "+id)
	if err != nil {
		return nil, nil, fmt.Errorf("could not snapshot the zone before restoring: %v", err)
	}
	p := parentsOf(snap)
	err = db.Transaction(func(tx *gorm.DB) error {
		deletes := []struct {
			sql  string
			args []any
		}{
			{"DELETE FROM spawn2 WHERE zone = ? AND id NOT IN ?", []any{zone, p.spawn2}},
			{"DELETE FROM spawnentry WHERE spawngroupID IN ?", []any{p.groups}},
			{"DELETE FROM loottable_entries WHERE loottable_id IN ?", []any{p.tables}},
			{"DELETE FROM lootdrop_entries WHERE lootdrop_id IN ?", []any{p.drops}},
		}
		for _, d := range deletes {
			if err := tx.Exec(d.sql, d.args...).Error; err != nil {
				return err
			}
		}
		for _, t := range tableOrder {
			if err := put(tx, t, snap.Tables[t]); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("nothing was changed: %v (a snapshot of how it was is kept as %v)", err, before.ID)
	}
	before.Tables = nil
	return before, diff, nil
}
