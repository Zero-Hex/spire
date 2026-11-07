package zoneloot

import (
	"database/sql"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/EQEmu/spire/internal/itemgen"
	"gorm.io/gorm"
)

// New loot is made from what the database already drops: the items NPCs near
// a level drop, for a slot or weapon type. A suggestion puts each stat at the
// tier's percentile of those items (zeros included, so a higher tier also
// picks up the stats fewer items have), and copies the comparable item
// nearest to it, keeping its type, slots, look and classes.

type Piece struct {
	Key       string `json:"key"`
	Label     string `json:"label"`
	Slots     int    `json:"slots"`
	ItemTypes []int  `json:"itemtypes,omitempty"` // a weapon's types; none: worn gear
}

// slot bits (RoF2 inventory slots) and item types (common/item_data.h)
var weaponTypes = []int{0, 1, 2, 3, 4, 5, 7, 8, 35, 45}

const weaponSlots = 8192 | 16384 | 2048

var Pieces = []Piece{
	{Key: "head", Label: "Head", Slots: 4},
	{Key: "face", Label: "Face", Slots: 8},
	{Key: "ears", Label: "Ear", Slots: 2 | 16},
	{Key: "neck", Label: "Neck", Slots: 32},
	{Key: "shoulders", Label: "Shoulders", Slots: 64},
	{Key: "arms", Label: "Arms", Slots: 128},
	{Key: "back", Label: "Back", Slots: 256},
	{Key: "wrist", Label: "Wrist", Slots: 512 | 1024},
	{Key: "hands", Label: "Hands", Slots: 4096},
	{Key: "chest", Label: "Chest", Slots: 131072},
	{Key: "legs", Label: "Legs", Slots: 262144},
	{Key: "feet", Label: "Feet", Slots: 524288},
	{Key: "waist", Label: "Waist", Slots: 1048576},
	{Key: "fingers", Label: "Ring", Slots: 32768 | 65536},
	{Key: "range", Label: "Range (worn)", Slots: 2048},
	{Key: "charm", Label: "Charm", Slots: 1},
	{Key: "1hs", Label: "1H Slashing", Slots: weaponSlots, ItemTypes: []int{0}},
	{Key: "2hs", Label: "2H Slashing", Slots: weaponSlots, ItemTypes: []int{1}},
	{Key: "1hp", Label: "1H Piercing", Slots: weaponSlots, ItemTypes: []int{2}},
	{Key: "2hp", Label: "2H Piercing", Slots: weaponSlots, ItemTypes: []int{35}},
	{Key: "1hb", Label: "1H Blunt", Slots: weaponSlots, ItemTypes: []int{3}},
	{Key: "2hb", Label: "2H Blunt", Slots: weaponSlots, ItemTypes: []int{4}},
	{Key: "h2h", Label: "Hand to Hand", Slots: weaponSlots, ItemTypes: []int{45}},
	{Key: "bow", Label: "Bow", Slots: weaponSlots, ItemTypes: []int{5}},
	{Key: "shield", Label: "Shield", Slots: weaponSlots, ItemTypes: []int{8}},
}

// the stats a suggestion sets, in the order they're shown
var StatColumns = []string{
	"ac", "hp", "mana", "endur",
	"astr", "asta", "aagi", "adex", "awis", "aint", "acha",
	"mr", "fr", "cr", "dr", "pr",
	"damage", "delay",
	"haste", "attack", "regen", "manaregen", "enduranceregen",
	"damageshield", "shielding", "spellshield", "avoidance", "accuracy", "strikethrough", "stunresist", "dotshielding",
	"spelldmg", "healamt", "clairvoyance",
	"heroic_str", "heroic_sta", "heroic_agi", "heroic_dex", "heroic_wis", "heroic_int", "heroic_cha",
}

// a copy starts with none of its template's effects, scripts or flags
var cleared = map[string]any{
	"clickeffect": -1, "proceffect": -1, "worneffect": -1, "focuseffect": -1, "scrolleffect": -1, "bardeffect": -1,
	"clicktype": 0, "clicklevel": 0, "clicklevel2": 0, "proctype": 0, "proclevel": 0, "proclevel2": 0, "worntype": 0,
	"focustype": 0, "casttime": 0, "recastdelay": 0, "recasttype": 0, "maxcharges": 0, "procrate": 0,
	"scriptfileid": 0, "charmfileid": "", "charmfile": "", "questitemflag": 0, "artifactflag": 0, "attuneable": 0,
	"evolvinglevel": 0, "loregroup": 0, "norent": 1, "magic": 1,
}

var tiers = map[string]float64{"trash": 50, "named": 75, "raid": 90}

type ItemBrief struct {
	ID       int            `json:"id"`
	Name     string         `json:"name"`
	Icon     int            `json:"icon"`
	Idfile   string         `json:"idfile"`
	ItemType int            `json:"itemtype"`
	Slots    int            `json:"slots"`
	Classes  int            `json:"classes"`
	Races    int            `json:"races"`
	ReqLevel int            `json:"reqlevel"`
	RecLevel int            `json:"reclevel"`
	Stats    map[string]int `json:"stats"`
}

type Suggestion struct {
	Piece    Piece          `json:"piece"`
	Levels   [2]int         `json:"levels"` // NPC levels the comparable items drop from
	Sample   int            `json:"sample"`
	Stats    map[string]int `json:"stats"`
	Template *ItemBrief     `json:"template"`
	Examples []ItemBrief    `json:"examples"` // the comparable items nearest the suggestion
}

type SuggestRequest struct {
	Pieces []string `json:"pieces"`
	Level  int      `json:"level"`
	Tier   string   `json:"tier"`
}

func liveStats(db *gorm.DB) ([]string, error) {
	live, err := liveColumns(db, "items")
	if err != nil {
		return nil, err
	}
	var out []string
	for _, c := range StatColumns {
		if _, ok := live[c]; ok {
			out = append(out, c)
		}
	}
	return out, nil
}

// comparable is the items NPCs from lo to hi drop that fit a piece
func comparable(db *gorm.DB, p Piece, lo, hi int, magicOnly bool, stats []string) ([]ItemBrief, error) {
	where := "(i.slots & ?) <> 0 AND i.itemtype <> 54"
	args := []any{lo, hi, p.Slots}
	if len(p.ItemTypes) > 0 {
		where += " AND i.itemtype IN ?"
		args = append(args, p.ItemTypes)
	} else {
		where += " AND i.itemtype NOT IN ?"
		args = append(args, weaponTypes)
	}
	if magicOnly {
		where += " AND i.magic = 1"
	}
	q := `SELECT DISTINCT i.id, i.Name, i.icon, i.idfile, i.itemtype, i.slots, i.classes, i.races, i.reqlevel, i.reclevel, i.` + strings.Join(stats, ", i.") + `
		FROM (SELECT DISTINCT loottable_id FROM npc_types WHERE level BETWEEN ? AND ? AND loottable_id > 0) t
		JOIN loottable_entries lte ON lte.loottable_id = t.loottable_id
		JOIN lootdrop_entries lde ON lde.lootdrop_id = lte.lootdrop_id
		JOIN items i ON i.id = lde.item_id
		WHERE ` + where + ` LIMIT 2000`
	rows, err := db.Raw(q, args...).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	raw := make([]sql.RawBytes, len(cols))
	ptrs := make([]any, len(cols))
	for i := range raw {
		ptrs[i] = &raw[i]
	}
	var out []ItemBrief
	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		num := func(i int) int {
			v, _ := strconv.ParseFloat(string(raw[i]), 64)
			return int(v)
		}
		it := ItemBrief{ID: num(0), Name: string(raw[1]), Icon: num(2), Idfile: string(raw[3]), ItemType: num(4), Slots: num(5),
			Classes: num(6), Races: num(7), ReqLevel: num(8), RecLevel: num(9), Stats: map[string]int{}}
		for k, c := range stats {
			it.Stats[c] = num(10 + k)
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func percentileOf(vals []int, p float64) int {
	if len(vals) == 0 {
		return 0
	}
	s := append([]int{}, vals...)
	sort.Ints(s)
	at := p / 100 * float64(len(s)-1)
	lo := int(math.Floor(at))
	hi := int(math.Ceil(at))
	return int(math.Round(float64(s[lo]) + (float64(s[hi])-float64(s[lo]))*(at-float64(lo))))
}

// Suggest works out stats for each piece from what NPCs near the level drop
func Suggest(db *gorm.DB, req SuggestRequest) ([]Suggestion, error) {
	p, ok := tiers[req.Tier]
	if !ok {
		return nil, fmt.Errorf("tier is trash, named or raid, not %q", req.Tier)
	}
	if req.Level < 1 || req.Level > 255 {
		return nil, errors.New("level must be from 1 to 255")
	}
	stats, err := liveStats(db)
	if err != nil {
		return nil, err
	}
	byKey := map[string]Piece{}
	for _, pc := range Pieces {
		byKey[pc.Key] = pc
	}
	out := []Suggestion{}
	for _, key := range req.Pieces {
		pc, ok := byKey[key]
		if !ok {
			return nil, fmt.Errorf("no piece %q", key)
		}
		var items []ItemBrief
		var lo, hi int
		// widen the level window until there's enough to go on
		for _, w := range []int{2, 4, 7, 10, 15} {
			lo, hi = max(1, req.Level-w), req.Level+w
			if items, err = comparable(db, pc, lo, hi, req.Tier != "trash", stats); err != nil {
				return nil, err
			}
			if len(items) >= 12 {
				break
			}
		}
		sug := Suggestion{Piece: pc, Levels: [2]int{lo, hi}, Sample: len(items), Stats: map[string]int{}, Examples: []ItemBrief{}}
		if len(items) == 0 {
			out = append(out, sug)
			continue
		}
		scale := map[string]float64{}
		for _, c := range stats {
			vals := make([]int, len(items))
			var nonzero []int
			for i, it := range items {
				vals[i] = it.Stats[c]
				if it.Stats[c] != 0 {
					nonzero = append(nonzero, it.Stats[c])
				}
			}
			v := percentileOf(vals, p)
			if c == "delay" {
				v = percentileOf(nonzero, 50) // a weapon's speed, not its quality
			}
			if v != 0 {
				sug.Stats[c] = v
			}
			scale[c] = math.Max(1, float64(percentileOf(nonzero, 90)))
		}
		// the comparable items nearest the suggestion: the nearest is copied
		dist := func(it ItemBrief) float64 {
			d := 0.0
			for _, c := range stats {
				d += math.Abs(float64(it.Stats[c]-sug.Stats[c])) / scale[c]
			}
			return d
		}
		sort.SliceStable(items, func(i, j int) bool { return dist(items[i]) < dist(items[j]) })
		t := items[0]
		sug.Template = &t
		sug.Examples = items[:min(6, len(items))]
		out = append(out, sug)
	}
	return out, nil
}

// NewItem is an item to make: a copy of its template with these set
type NewItem struct {
	TemplateID int            `json:"template_id"`
	Name       string         `json:"name"`
	Icon       int            `json:"icon"`
	Idfile     string         `json:"idfile"`
	Stats      map[string]int `json:"stats"` // stats left out are 0
	ReqLevel   int            `json:"reqlevel"`
	RecLevel   int            `json:"reclevel"`
	Classes    int            `json:"classes"` // 0: the template's
	NoDrop     bool           `json:"nodrop"`
	Lore       bool           `json:"lore"`
	Chance     float64        `json:"chance"` // in the drop group
}

type MakeRequest struct {
	Version int       `json:"version"`
	Start   int       `json:"start"`
	End     int       `json:"end"`
	Items   []NewItem `json:"items"`
	// the drop group they go in: an existing one, or a new one in LoottableID
	LoottableID int    `json:"loottable_id"`
	LootdropID  int    `json:"lootdrop_id"`
	GroupName   string `json:"group_name"`
	Everywhere  bool   `json:"everywhere"`
	Comment     string `json:"comment"`
}

type MakeResult struct {
	Items      []itemgen.Created `json:"items"`
	LootdropID int               `json:"lootdrop_id"`
	Zone       *ZoneLoot         `json:"zone"`
}

// Make creates the items and adds them to the drop group, in one transaction
func Make(db *gorm.DB, zone string, req MakeRequest) (*MakeResult, error) {
	if err := Transactional(db); err != nil {
		return nil, err
	}
	if len(req.Items) == 0 {
		return nil, errors.New("no items to make")
	}
	stats, err := liveStats(db)
	if err != nil {
		return nil, err
	}
	known := map[string]bool{}
	for _, c := range stats {
		known[c] = true
	}
	live, err := liveColumns(db, "items")
	if err != nil {
		return nil, err
	}
	var copies []itemgen.Copy
	for _, it := range req.Items {
		name := strings.TrimSpace(it.Name)
		switch {
		case it.TemplateID <= 0:
			return nil, fmt.Errorf("%v: no item to copy", name)
		case it.Icon < 0:
			return nil, fmt.Errorf("%v: no icon", name)
		case len(it.Idfile) > 30:
			return nil, fmt.Errorf("%v: %q is not an item model", name, it.Idfile)
		case it.Chance < 0 || it.Chance > 100:
			return nil, fmt.Errorf("%v: chance must be from 0 to 100", name)
		case it.ReqLevel < 0 || it.ReqLevel > 255 || it.RecLevel < 0 || it.RecLevel > 255:
			return nil, fmt.Errorf("%v: levels are from 0 to 255", name)
		}
		set := map[string]any{"name": name, "icon": it.Icon, "idfile": it.Idfile, "reqlevel": it.ReqLevel, "reclevel": it.RecLevel}
		for k, v := range cleared {
			if _, ok := live[k]; ok {
				set[k] = v
			}
		}
		for _, c := range stats {
			set[c] = 0
		}
		for k, v := range it.Stats {
			if !known[k] {
				return nil, fmt.Errorf("%v: %q is not a stat", name, k)
			}
			if v < -32768 || v > 2000000 {
				return nil, fmt.Errorf("%v: %v %d is out of range", name, k, v)
			}
			set[k] = v
		}
		if it.Classes > 0 {
			set["classes"] = it.Classes
		}
		// nodrop: 0 is NO DROP
		set["nodrop"] = 1
		if it.NoDrop {
			set["nodrop"] = 0
		}
		set["lore"] = name
		if it.Lore {
			set["lore"] = "*" + name
		}
		comment := strings.TrimSpace(req.Comment)
		if comment == "" {
			comment = "Spire Zone Loot"
		}
		if _, ok := live["comment"]; ok {
			set["comment"] = fmt.Sprintf("%v: %v, a copy of %d", comment, zone, it.TemplateID)
		}
		copies = append(copies, itemgen.Copy{TemplateID: it.TemplateID, Set: set})
	}

	res := &MakeResult{}
	err = db.Transaction(func(tx *gorm.DB) error {
		made, err := itemgen.CopyItems(tx, req.Start, req.End, copies)
		if err != nil {
			return err
		}
		res.Items = made
		group := req.LootdropID
		if group == 0 {
			var name *string
			if n := strings.TrimSpace(req.GroupName); n != "" {
				name = &n
			}
			created, err := applyEdits(tx, zone, req.Version, []Edit{{Op: "new_lootdrop", LoottableID: req.LoottableID, Name: name, Everywhere: req.Everywhere}})
			if err != nil {
				return err
			}
			group = created[0].ID
		}
		res.LootdropID = group
		var edits []Edit
		for i, m := range made {
			chance, one := req.Items[i].Chance, 1
			edits = append(edits, Edit{Op: "lootdrop_entry", LootdropID: group, ItemID: m.ID, Chance: &chance, Multiplier: &one, Everywhere: req.Everywhere})
		}
		if _, err := applyEdits(tx, zone, req.Version, edits); err != nil {
			return err
		}
		res.Zone, err = Load(tx, zone, req.Version)
		return err
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}
