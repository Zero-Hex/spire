package itemgen

import (
	"errors"
	"fmt"
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
	// RoF2 item links write the item id as 5 hex digits
	MaxID = 0xFFFFF

	nameLimit = 64 // items.Name
)

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
}

type PlanRequest struct {
	Kind      string `json:"kind"`
	Archive   string `json:"archive"`
	Models    []int  `json:"models"`    // weapon: the item models to make items of
	Set       int    `json:"set"`       // armor, ornament: the Heroes' Forge set, if not the one found
	Templates []int  `json:"templates"` // the items to copy, if not the usual ones
	Name      string `json:"name"`      // weapon: its name; sets: put before each piece's name
	Icon      int    `json:"icon"`      // weapon: its icon
	Start     int    `json:"start"`     // number from here (FirstID)
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
	Kind      string     `json:"kind"`
	Archive   string     `json:"archive"`
	Set       int        `json:"set"`
	SetSource string     `json:"set_source"`
	Sets      []SetMatch `json:"sets"` // every set the archive's models are in
	Rows      []Row      `json:"rows"`
	FirstID   int        `json:"first_id"`
	Warnings  []string   `json:"warnings"`
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
	archive := idx.Archives[strings.ToLower(req.Archive)]
	if archive == nil {
		return Plan{}, fmt.Errorf("Spire holds no item model archive %v", req.Archive)
	}
	start, err := checkStart(req.Start)
	if err != nil {
		return Plan{}, err
	}
	templates, err := loadTemplates(db, req)
	if err != nil {
		return Plan{}, err
	}

	plan := Plan{Kind: req.Kind, Archive: archive.File, Sets: []SetMatch{}, Rows: []Row{}, Warnings: []string{}}
	switch {
	case archive.Ignore:
		plan.Warnings = append(plan.Warnings, archive.File+" is set to Ignore: players do not get it, so they will not see these models.")
	case archive.Remove:
		plan.Warnings = append(plan.Warnings, archive.File+" is set to Remove: players' copies are deleted, so they will not see these models.")
	case archive.Beta:
		plan.Warnings = append(plan.Warnings, archive.File+" is Beta only, but items are in the database every player uses: live players will not see these models until it goes live.")
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = strings.TrimSuffix(archive.File, filepath.Ext(archive.File))
	}
	if req.Kind == KindWeapon {
		plan.Rows, err = weaponRows(archive, templates[0], req.Models, name, req.Icon)
		if err != nil {
			return Plan{}, err
		}
	} else {
		var tags ArmorTags
		if data, ok := g.assets.ClientFileData("Resources/NewArmorTagData.txt"); ok {
			tags = ParseArmorTags(data)
		}
		plan.Sets = FindSets(tags, archive.Models)
		set, slots, source := chooseSet(plan.Sets, req.Set, archive.Texture, tags != nil)
		if set == 0 {
			return Plan{}, fmt.Errorf("no Heroes' Forge set names %v's models: upload the client's Resources/NewArmorTagData.txt with its rows, or give the set number", archive.File)
		}
		if set < minSet {
			return Plan{}, fmt.Errorf("set %d: the server reads Heroes' Forge numbers under 1000 as shorthand, so sets start at %d", set, minSet)
		}
		plan.Set, plan.SetSource = set, source
		var warnings []string
		plan.Rows, warnings = setRows(req.Kind, templates, set, slots, name)
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

	plan.FirstID, err = nextID(db, start, len(plan.Rows))
	return plan, err
}

func checkStart(start int) (int, error) {
	if start == 0 {
		start = FirstID
	}
	if start < 1 || start > MaxID {
		return 0, fmt.Errorf("item ids go up to %d: RoF2 item links cannot hold more", MaxID)
	}
	return start, nil
}

// chooseSet is the set to use, the slots it has models for (all but the robe
// when not known), and where the set number came from
func chooseSet(found []SetMatch, asked int, texture string, haveTags bool) (int, []int, string) {
	allSlots := []int{0, 1, 2, 3, 4, 5, 6}
	if asked > 0 {
		for _, m := range found {
			if m.Set == asked {
				return asked, m.Slots, "chosen (NewArmorTagData.txt names its slots)"
			}
		}
		return asked, allSlots, "chosen"
	}
	if len(found) > 0 {
		return found[0].Set, found[0].Slots, "NewArmorTagData.txt"
	}
	if set := SetFromTexture(texture); set > 0 {
		why := "the texture name " + texture + " (Spire holds no Resources/NewArmorTagData.txt)"
		if haveTags {
			why = "the texture name " + texture + " (NewArmorTagData.txt has no rows for these models yet)"
		}
		return set, allSlots, why
	}
	return 0, nil, ""
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
		slot := slotOf(t.Slots)
		switch {
		case slot < 0:
			warnings = append(warnings, fmt.Sprintf("%v (%d) is not worn in an armor slot, so it is left out.", t.Name, t.ID))
			continue
		case !has[slot]:
			warnings = append(warnings, fmt.Sprintf("Set %d has no %v models in this archive, so %v (%d) is left out.", set, strings.ToLower(slotLabel(slot)), t.Name, t.ID))
			continue
		case used[slot]:
			warnings = append(warnings, fmt.Sprintf("%v (%d) is a second %v: both are made.", t.Name, t.ID, strings.ToLower(slotLabel(slot))))
		}
		used[slot] = true
		if slot == 1 && chest == nil {
			chest = &templates[i]
		}
		rows = append(rows, Row{
			TemplateID: t.ID, TemplateName: t.Name, Slot: slot, SlotLabel: slotLabel(slot),
			Name: name + " " + slotLabel(slot) + suffix, Icon: t.Icon, Idfile: t.Idfile, Herosforgemodel: set*100 + slot,
		})
	}
	// a robe is worn in the chest slot
	if has[SlotRobe] && chest != nil {
		rows = append(rows, Row{
			TemplateID: chest.ID, TemplateName: chest.Name, Slot: SlotRobe, SlotLabel: slotLabel(SlotRobe),
			Name: name + " " + slotLabel(SlotRobe) + suffix, Icon: chest.Icon, Idfile: chest.Idfile, Herosforgemodel: set*100 + SlotRobe,
		})
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Slot < rows[j].Slot })
	return rows, warnings
}

// loadTemplates reads the items to copy: those asked for, in that order, or
// the usual ones for the kind
func loadTemplates(db *gorm.DB, req PlanRequest) ([]Template, error) {
	cols := "id, Name, icon, idfile, slots, herosforgemodel"
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

// nextID is the first of n free ids from start: past the highest id in use
// at or above it
func nextID(db *gorm.DB, start, n int) (int, error) {
	var highest *int
	if err := db.Table("items").Select("MAX(id)").Where("id >= ? AND id <= ?", start, MaxID).Scan(&highest).Error; err != nil {
		return 0, err
	}
	next := start
	if highest != nil {
		next = *highest + 1
	}
	if next+n-1 > MaxID {
		return 0, fmt.Errorf("no %d free item ids from %d up to %d", n, start, MaxID)
	}
	return next, nil
}

type CreateRequest struct {
	Kind    string `json:"kind"`
	Archive string `json:"archive"`
	Start   int    `json:"start"`
	Rows    []Row  `json:"rows"`
}

type Created struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Create makes the items, numbered on from the highest at or above start
func (g *Generator) Create(db *gorm.DB, req CreateRequest) ([]Created, error) {
	start, err := checkStart(req.Start)
	if err != nil {
		return nil, err
	}
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

	var created []Created
	err = db.Transaction(func(tx *gorm.DB) error {
		id, err := nextID(tx, start, len(req.Rows))
		if err != nil {
			return err
		}
		for _, r := range req.Rows {
			comment := fmt.Sprintf("Spire: %v from %v, a copy of %d", req.Kind, req.Archive, r.TemplateID)
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
			id++
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
