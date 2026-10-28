package itemgen

import (
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/EQEmu/spire/internal/clientassets"
	"gorm.io/gorm"
)

// New items are copies of template items with the model, icon and name
// changed: a weapon from an item model, an armor or ornament set from a
// Heroes' Forge armor set. They are numbered from FirstID up.

const (
	KindWeapon   = "weapon"
	KindArmor    = "armor"
	KindOrnament = "ornament"

	FirstID = 300000
	// items.id is an int. The server links items past 5 hex digits through
	// the link's hash (extended item links, common/say_link.cpp).
	MaxID = math.MaxInt32
	// an augment's id in a link is still 5 hex digits, and the two above it
	// are reserved
	MaxAugmentID = 0xFFFFD

	nameLimit = 64 // items.Name
)

// ids the server's links use as markers (common/features.h): never an item's
var reservedIDs = map[int]bool{
	0xFFFFE: true, // EXTENDED_ITEM_LINK_ID
	0xFFFFF: true, // SAYLINK_ITEM_ID
}

// the templates used unless others are given
var (
	weaponTemplate = "Fine Steel Short Sword"
	clothTemplates = []string{"Cloth Cap", "Cloth Shirt", "Cloth Sleeves", "Cloth Wristband", "Cloth Gloves", "Cloth Pants", "Cloth Sandals"}
	// the Old Hero's Forge set
	ornamentTemplates = []int{67983, 67984, 67985, 67986, 67987, 67988, 67989}
)

type Template struct {
	ID              int    `json:"id" gorm:"column:id"`
	Name            string `json:"name" gorm:"column:Name"`
	Icon            int    `json:"icon" gorm:"column:icon"`
	Idfile          string `json:"idfile" gorm:"column:idfile"`
	Slots           int    `json:"slots" gorm:"column:slots"`
	Herosforgemodel int    `json:"herosforgemodel" gorm:"column:herosforgemodel"`
	Augtype         int    `json:"augtype" gorm:"column:augtype"`
	Itemtype        int    `json:"itemtype" gorm:"column:itemtype"`
}

// an augment (ornaments are) must keep an id a link can carry
func (t Template) augment() bool { return t.Augtype > 0 || t.Itemtype == itemtypeAugment }

const itemtypeAugment = 54

type PlanRequest struct {
	Kind      string `json:"kind"`
	Archive   string `json:"archive"`   // weapon: the archive with its models
	Models    []int  `json:"models"`    // weapon: the item models to make items of
	Set       int    `json:"set"`       // armor, ornament: the Heroes' Forge set
	Templates []int  `json:"templates"` // the items to copy, if not the usual ones
	Name      string `json:"name"`      // weapon: its name; sets: put before each piece's name
	Icon      int    `json:"icon"`      // weapon: its icon
	Start     int    `json:"start"`     // number from here (FirstID)
	End       int    `json:"end"`       // and no further (0: as far as ids go)
}

// Row is one item to make
type Row struct {
	TemplateID      int    `json:"template_id"`
	TemplateName    string `json:"template_name"`
	Slot            int    `json:"slot"` // Heroes' Forge slot; -1 for a weapon
	SlotLabel       string `json:"slot_label"`
	Name            string `json:"name"`
	Icon            int    `json:"icon"`
	Idfile          string `json:"idfile"`
	Herosforgemodel int    `json:"herosforgemodel"`
}

type Plan struct {
	Kind     string   `json:"kind"`
	Archive  string   `json:"archive"`
	Set      int      `json:"set"`
	Rows     []Row    `json:"rows"`
	FirstID  int      `json:"first_id"`
	IDs      []int    `json:"ids"` // the id each row would get
	Warnings []string `json:"warnings"`
}

type Generator struct {
	assets *clientassets.Store
	mu     sync.Mutex // one creation at a time: ids are taken from the highest
}

func NewGenerator(assets *clientassets.Store) *Generator {
	return &Generator{assets: assets}
}

// Plan lists the items a request would make, for people to check and edit
func (g *Generator) Plan(db *gorm.DB, req PlanRequest) (Plan, error) {
	if req.Kind != KindWeapon && req.Kind != KindArmor && req.Kind != KindOrnament {
		return Plan{}, fmt.Errorf("%q is not weapon, armor or ornament", req.Kind)
	}
	idx := g.assets.Index()
	templates, err := loadTemplates(db, req)
	if err != nil {
		return Plan{}, err
	}
	augment := false
	for _, t := range templates {
		augment = augment || t.augment()
	}
	ids, err := checkRange(req.Start, req.End, augment)
	if err != nil {
		return Plan{}, err
	}

	plan := Plan{Kind: req.Kind, Rows: []Row{}, Warnings: []string{}}
	held := func(file string, beta, ignore, remove bool) {
		switch {
		case ignore:
			plan.Warnings = append(plan.Warnings, file+" is set to Ignore: players do not get it, so they will not see these models.")
		case remove:
			plan.Warnings = append(plan.Warnings, file+" is set to Remove: players' copies are deleted, so they will not see these models.")
		case beta:
			plan.Warnings = append(plan.Warnings, file+" is Beta only, but items are in the database every player uses: live players will not see these models until it goes live.")
		}
	}

	if req.Kind == KindWeapon {
		archive := idx.Archives[strings.ToLower(req.Archive)]
		if archive == nil {
			return Plan{}, fmt.Errorf("Spire holds no item model archive %v", req.Archive)
		}
		plan.Archive = archive.File
		held(archive.File, archive.Beta, archive.Ignore, archive.Remove)
		name := strings.TrimSpace(req.Name)
		if name == "" {
			name = strings.TrimSuffix(archive.File, filepath.Ext(archive.File))
		}
		plan.Rows, err = weaponRows(archive, templates[0], req.Models, name, req.Icon)
		if err != nil {
			return Plan{}, err
		}
	} else {
		if req.Set < clientassets.MinSet {
			return Plan{}, fmt.Errorf("give the Heroes' Forge set number: the server reads numbers under 1000 as shorthand, so sets start at %d", clientassets.MinSet)
		}
		plan.Set = req.Set
		set := findSet(g.assets.ArmorSets(), req.Set)
		slots := []int{0, 1, 2, 3, 4, 5, 6}
		switch {
		case set == nil:
			return Plan{}, fmt.Errorf("Spire holds no models of set %d", req.Set)
		case set.Set == 0:
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("No NewArmorTagData.txt row Spire holds names set %d's models: the client shows nothing for these pieces until the tag file with its rows is uploaded and sent to players.", req.Set))
		default:
			slots = set.Slots
			if len(set.Missing) > 0 {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("NewArmorTagData.txt names %d model(s) of set %d that no archive Spire holds has (IT%d...): those pieces show nothing for some races.", len(set.Missing), req.Set, set.Missing[0]))
			}
		}
		for _, a := range set.Archives {
			held(a.File, a.Beta, a.Ignore, a.Remove)
		}
		name := strings.TrimSpace(req.Name)
		if name == "" {
			name = fmt.Sprintf("Set %d", req.Set)
		}
		var warnings []string
		plan.Rows, warnings = setRows(req.Kind, templates, req.Set, slots, name)
		plan.Warnings = append(plan.Warnings, warnings...)
	}

	// icons from sheets players do not get
	warned := map[int]bool{}
	for _, r := range plan.Rows {
		n := clientassets.SheetOfIcon(r.Icon)
		sh := idx.Sheets[n]
		if sh == nil || warned[n] {
			continue
		}
		switch {
		case sh.Ignore || sh.Remove:
			warned[n] = true
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("Icon %d is on sheet %d, which players do not get (%v).", r.Icon, n, map[bool]string{true: "Ignore", false: "Remove"}[sh.Ignore]))
		case sh.Beta:
			warned[n] = true
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("Icon %d is on sheet %d, which is Beta only: live players will not see it.", r.Icon, n))
		}
	}

	plan.IDs, err = nextIDs(db, ids, len(plan.Rows))
	if err != nil {
		return Plan{}, err
	}
	if len(plan.IDs) > 0 {
		plan.FirstID = plan.IDs[0]
	}
	return plan, nil
}

type idRange struct {
	start, end int
	augment    bool
}

// checkRange is where new ids go: from start (FirstID) to end (as far as ids
// go, or an augment's limit)
func checkRange(start, end int, augment bool) (idRange, error) {
	if start == 0 {
		start = FirstID
	}
	limit := MaxID
	if augment {
		limit = MaxAugmentID
	}
	if end == 0 || end > limit {
		end = limit
	}
	switch {
	case start < 1:
		return idRange{}, errors.New("item ids start at 1")
	case augment && start > MaxAugmentID:
		return idRange{}, fmt.Errorf("these are augments, and an augment's id in an item link is 5 hex digits: start at or below %d", MaxAugmentID)
	case start > end:
		return idRange{}, fmt.Errorf("the ids run from %d up to %d: the start is past the end", start, end)
	}
	return idRange{start: start, end: end, augment: augment}, nil
}

// findSet is the set by its number: as NewArmorTagData.txt names it, or
// else models no tag row names whose texture suggests that number
func findSet(sets []clientassets.ArmorSet, n int) *clientassets.ArmorSet {
	for i := range sets {
		if sets[i].Set == n {
			return &sets[i]
		}
	}
	for i := range sets {
		if sets[i].Set == 0 && sets[i].Guess == n {
			return &sets[i]
		}
	}
	return nil
}

func weaponRows(archive *clientassets.Archive, t Template, models []int, name string, icon int) ([]Row, error) {
	if len(models) == 0 {
		if len(archive.Models) != 1 {
			return nil, fmt.Errorf("%v holds %d item models: choose which", archive.File, len(archive.Models))
		}
		models = archive.Models
	}
	has := map[int]bool{}
	for _, m := range archive.Models {
		has[m] = true
	}
	if icon <= 0 {
		icon = t.Icon
	}
	var rows []Row
	for i, m := range models {
		if !has[m] {
			return nil, fmt.Errorf("IT%d is not in %v", m, archive.File)
		}
		n := name
		if len(models) > 1 {
			n = fmt.Sprintf("%s %d", name, i+1)
		}
		rows = append(rows, Row{TemplateID: t.ID, TemplateName: t.Name, Slot: -1, Name: n, Icon: icon, Idfile: fmt.Sprintf("IT%d", m)})
	}
	return rows, nil
}

// setRows copies each template to the piece of the set for its slot
func setRows(kind string, templates []Template, set int, slots []int, name string) ([]Row, []string) {
	has := map[int]bool{}
	for _, s := range slots {
		has[s] = true
	}
	suffix := ""
	if kind == KindOrnament {
		suffix = " Ornament"
	}
	var rows []Row
	var warnings []string
	var chest *Template
	used := map[int]bool{}
	for i, t := range templates {
		slot := clientassets.SlotOf(t.Slots)
		switch {
		case slot < 0:
			warnings = append(warnings, fmt.Sprintf("%v (%d) is not worn in an armor slot, so it is left out.", t.Name, t.ID))
			continue
		case !has[slot]:
			warnings = append(warnings, fmt.Sprintf("Set %d has no %v models, so %v (%d) is left out.", set, strings.ToLower(clientassets.SlotLabel(slot)), t.Name, t.ID))
			continue
		case used[slot]:
			warnings = append(warnings, fmt.Sprintf("%v (%d) is a second %v: both are made.", t.Name, t.ID, strings.ToLower(clientassets.SlotLabel(slot))))
		}
		used[slot] = true
		if slot == 1 && chest == nil {
			chest = &templates[i]
		}
		rows = append(rows, Row{
			TemplateID: t.ID, TemplateName: t.Name, Slot: slot, SlotLabel: clientassets.SlotLabel(slot),
			Name: name + " " + clientassets.SlotLabel(slot) + suffix, Icon: t.Icon, Idfile: t.Idfile, Herosforgemodel: set*100 + slot,
		})
	}
	// a robe is worn in the chest slot
	if has[clientassets.SlotRobe] && chest != nil {
		rows = append(rows, Row{
			TemplateID: chest.ID, TemplateName: chest.Name, Slot: clientassets.SlotRobe, SlotLabel: clientassets.SlotLabel(clientassets.SlotRobe),
			Name: name + " " + clientassets.SlotLabel(clientassets.SlotRobe) + suffix, Icon: chest.Icon, Idfile: chest.Idfile, Herosforgemodel: set*100 + clientassets.SlotRobe,
		})
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Slot < rows[j].Slot })
	return rows, warnings
}

// loadTemplates reads the items to copy: those asked for, in that order, or
// the usual ones for the kind
func loadTemplates(db *gorm.DB, req PlanRequest) ([]Template, error) {
	cols := "id, Name, icon, idfile, slots, herosforgemodel, augtype, itemtype"
	if len(req.Templates) > 0 {
		var found []Template
		if err := db.Table("items").Select(cols).Where("id IN ?", req.Templates).Find(&found).Error; err != nil {
			return nil, err
		}
		byID := map[int]Template{}
		for _, t := range found {
			byID[t.ID] = t
		}
		var out []Template
		for _, id := range req.Templates {
			t, ok := byID[id]
			if !ok {
				return nil, fmt.Errorf("there is no item %d to copy", id)
			}
			out = append(out, t)
		}
		return out, nil
	}

	switch req.Kind {
	case KindOrnament:
		return loadTemplates(db, PlanRequest{Templates: ornamentTemplates})
	case KindWeapon:
		return templatesNamed(db, cols, []string{weaponTemplate})
	}
	return templatesNamed(db, cols, clothTemplates)
}

func templatesNamed(db *gorm.DB, cols string, names []string) ([]Template, error) {
	var found []Template
	if err := db.Table("items").Select(cols).Where("Name IN ?", names).Order("id").Find(&found).Error; err != nil {
		return nil, err
	}
	first := map[string]Template{}
	for _, t := range found {
		if _, ok := first[strings.ToLower(t.Name)]; !ok {
			first[strings.ToLower(t.Name)] = t
		}
	}
	var out []Template
	var missing []string
	for _, n := range names {
		t, ok := first[strings.ToLower(n)]
		if !ok {
			missing = append(missing, n)
			continue
		}
		out = append(out, t)
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("no item is named %v: give the item ids to copy instead", strings.Join(missing, ", "))
	}
	return out, nil
}

// nextIDs are n free ids in the range: past the highest id in use in it,
// skipping the ids links reserve
func nextIDs(db *gorm.DB, r idRange, n int) ([]int, error) {
	var highest *int
	if err := db.Table("items").Select("MAX(id)").Where("id >= ? AND id <= ?", r.start, r.end).Scan(&highest).Error; err != nil {
		return nil, err
	}
	next := r.start
	if highest != nil {
		next = *highest + 1
	}
	return takeIDs(next, r, n)
}

func takeIDs(next int, r idRange, n int) ([]int, error) {
	ids := []int{}
	for id := next; len(ids) < n; id++ {
		if id > r.end {
			why := ""
			if r.augment && r.end == MaxAugmentID {
				why = " (augments, ornaments included, stay at or below it)"
			}
			return nil, fmt.Errorf("no %d free item ids from %d up to %d%v", n, next, r.end, why)
		}
		if !reservedIDs[id] {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

type CreateRequest struct {
	Kind    string `json:"kind"`
	Archive string `json:"archive"` // weapon
	Set     int    `json:"set"`     // armor, ornament
	Start   int    `json:"start"`
	End     int    `json:"end"`
	Rows    []Row  `json:"rows"`
}

type Created struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Create makes the items, numbered on from the highest at or above start
func (g *Generator) Create(db *gorm.DB, req CreateRequest) ([]Created, error) {
	if len(req.Rows) == 0 {
		return nil, errors.New("no items to make")
	}
	for _, r := range req.Rows {
		switch {
		case strings.TrimSpace(r.Name) == "":
			return nil, errors.New("every item needs a name")
		case len(r.Name) > nameLimit:
			return nil, fmt.Errorf("%q is longer than %d characters", r.Name, nameLimit)
		case r.Icon < 0:
			return nil, fmt.Errorf("%v: no icon", r.Name)
		case len(r.Idfile) > 30:
			return nil, fmt.Errorf("%v: %q is not an item model", r.Name, r.Idfile)
		}
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	var columns []string
	if err := db.Raw("SELECT COLUMN_NAME FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'items' ORDER BY ORDINAL_POSITION").Scan(&columns).Error; err != nil {
		return nil, err
	}
	if len(columns) == 0 {
		return nil, errors.New("there is no items table")
	}

	var templateIDs []int
	for _, r := range req.Rows {
		templateIDs = append(templateIDs, r.TemplateID)
	}
	var augments int64
	if err := db.Table("items").Where("id IN ? AND (augtype > 0 OR itemtype = ?)", templateIDs, itemtypeAugment).Count(&augments).Error; err != nil {
		return nil, err
	}
	ids, err := checkRange(req.Start, req.End, augments > 0)
	if err != nil {
		return nil, err
	}

	var created []Created
	err = db.Transaction(func(tx *gorm.DB) error {
		newIDs, err := nextIDs(tx, ids, len(req.Rows))
		if err != nil {
			return err
		}
		for i, r := range req.Rows {
			id := newIDs[i]
			from := req.Archive
			if req.Kind != KindWeapon {
				from = fmt.Sprintf("Heroes' Forge set %d", req.Set)
			}
			comment := fmt.Sprintf("Spire: %v from %v, a copy of %d", req.Kind, from, r.TemplateID)
			sql, args := copySQL(columns, r.TemplateID, map[string]any{
				"id":              id,
				"name":            strings.TrimSpace(r.Name),
				"icon":            r.Icon,
				"idfile":          r.Idfile,
				"herosforgemodel": r.Herosforgemodel,
				"comment":         comment,
				"updated":         time.Now(),
			})
			res := tx.Exec(sql, args...)
			if res.Error != nil {
				return fmt.Errorf("%v: %v", r.Name, res.Error)
			}
			if res.RowsAffected != 1 {
				return fmt.Errorf("%v: there is no item %d to copy", r.Name, r.TemplateID)
			}
			created = append(created, Created{ID: id, Name: strings.TrimSpace(r.Name)})
		}
		return nil
	})
	return created, err
}

// copySQL copies a template item with some columns changed: every column is
// copied, so any the server added (and Spire does not know) come along.
// The lore line follows the name when it was the template's name.
func copySQL(columns []string, templateID int, set map[string]any) (string, []any) {
	var names, values []string
	var args []any
	for _, c := range columns {
		names = append(names, "`"+c+"`")
		lower := strings.ToLower(c)
		if v, ok := set[lower]; ok {
			values = append(values, "?")
			args = append(args, v)
			continue
		}
		if lower == "lore" {
			if name, ok := set["name"]; ok {
				values = append(values, "IF(`lore` = `Name`, ?, IF(`lore` = CONCAT('*', `Name`), CONCAT('*', ?), `lore`))")
				args = append(args, name, name)
				continue
			}
		}
		values = append(values, "`"+c+"`")
	}
	args = append(args, templateID)
	return "INSERT INTO items (" + strings.Join(names, ", ") + ") SELECT " + strings.Join(values, ", ") + " FROM items WHERE id = ?", args
}
