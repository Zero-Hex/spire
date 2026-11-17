package ambient

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/EQEmu/spire/internal/models"
	"gorm.io/gorm"
)

// Ambient lines (starfall-3 plugins/ambient.pl, docs/design/ambient_lines.md):
// NPCs near players say or emote a line now and then (idle), or greet a player
// who walks up (approach). A line goes to everyone nearby (the NPC's own say
// or emote) or only to the player it's for; empty audience is by kind.

const (
	MaxText    = 255
	MaxNpcName = 64
	MaxNotes   = 255
)

var (
	kinds     = map[string]bool{"idle": true, "approach": true}
	modes     = map[string]bool{"say": true, "emote": true}
	audiences = map[string]bool{"": true, "nearby": true, "player": true}
	times     = map[string]bool{"any": true, "day": true, "night": true}
	zoneName  = regexp.MustCompile(`^[a-z0-9_]+$`)
	braces    = regexp.MustCompile(`\{[^}]*\}`)
	fills     = map[string]bool{"{player}": true, "{race}": true, "{class}": true, "{npc}": true}
)

// Standings: the faction con an NPC has toward a player (common/faction.h
// FACTION_VALUE), with the other spellings plugins/ambient.pl accepts
var Standings = []string{"ally", "warmly", "kindly", "amiably", "indifferent", "apprehensive", "dubious", "threatening", "scowls"}

var standingAliases = map[string]string{"amiable": "amiably", "indifferently": "indifferent", "apprehensively": "apprehensive",
	"dubiously": "dubious", "threateningly": "threatening", "scowling": "scowls"}

func standing(name string) (string, int, bool) {
	n := strings.ToLower(strings.TrimSpace(name))
	if a, ok := standingAliases[n]; ok {
		n = a
	}
	if n == "" {
		return "", 0, true
	}
	for i, s := range Standings {
		if s == n {
			return s, i + 1, true
		}
	}
	return "", 0, false
}

// the columns later migrations add
var laterColumns = []struct {
	name      string
	migration int
	isDefault func(l models.CustomAmbientLine) bool
}{
	{"audience", 72, func(l models.CustomAmbientLine) bool { return l.Audience == "" }},
	{"faction_from", 73, func(l models.CustomAmbientLine) bool { return l.FactionFrom == "" }},
	{"faction_to", 73, func(l models.CustomAmbientLine) bool { return l.FactionTo == "" }},
}

var errNoTable = errors.New("there is no custom_ambient_line table: the server makes it (Starfall custom migration 71) when world starts on a build with ambient lines")

func hasTable(db *gorm.DB, table string) bool {
	var n int64
	db.Raw("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?", table).Scan(&n)
	return n > 0
}

func hasColumn(db *gorm.DB, table, column string) bool {
	var n int64
	db.Raw("SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?", table, column).Scan(&n)
	return n > 0
}

type Zone struct {
	ShortName string `json:"short_name"`
	LongName  string `json:"long_name"`
}

// NPC is one the zone spawns, for seeing which lines fit whom
type NPC struct {
	ID       uint32 `json:"id"`
	Name     string `json:"name"`
	Clean    string `json:"clean"` // as players see it (GetCleanName)
	Race     uint32 `json:"race"`
	Bodytype uint32 `json:"bodytype"`
	Level    uint32 `json:"level"`
	Silent   bool   `json:"silent"` // never speaks: invisible-man / zone-controller race, untargetable body type
}

type List struct {
	Zones    []Zone                     `json:"zones"`
	Counts   map[string]int             `json:"counts"`   // lines per zone, "" = every zone
	Lines    []models.CustomAmbientLine `json:"lines"`    // the zone's and every zone's
	NPCs     []NPC                      `json:"npcs"`     // the zone's, every version
	Audience bool                       `json:"audience"` // migration 72
	Faction  bool                       `json:"faction"`  // migration 73
}

// CleanName is EQEmu's CleanMobName: _ to a space, letters and ` kept
func CleanName(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r == '_':
			b.WriteByte(' ')
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '`':
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Silent says whether the plugin never lets an NPC of a race and body type speak
func Silent(race, bodytype uint32) bool {
	return race == 127 || race == 240 || bodytype == 11 || bodytype == 66 || bodytype == 67
}

// Read lists the zones and line counts, and a zone's lines and NPCs (zone ""
// lists the lines for every zone)
func Read(content, server *gorm.DB, zone string) (*List, error) {
	if !hasTable(server, "custom_ambient_line") {
		return nil, errNoTable
	}
	out := &List{Zones: []Zone{}, Counts: map[string]int{}, Lines: []models.CustomAmbientLine{}, NPCs: []NPC{},
		Audience: hasColumn(server, "custom_ambient_line", "audience"), Faction: hasColumn(server, "custom_ambient_line", "faction_from")}
	if err := content.Raw(`SELECT z.short_name, z.long_name FROM zone z
		JOIN (SELECT short_name, MIN(version) AS version FROM zone WHERE short_name <> '' GROUP BY short_name) v
		ON v.short_name = z.short_name AND v.version = z.version GROUP BY z.short_name ORDER BY z.long_name`).Scan(&out.Zones).Error; err != nil {
		return nil, err
	}
	var counts []struct {
		ZoneShortName string
		N             int
	}
	server.Raw("SELECT zone_short_name, COUNT(*) AS n FROM custom_ambient_line GROUP BY zone_short_name").Scan(&counts)
	for _, c := range counts {
		out.Counts[c.ZoneShortName] = c.N
	}
	zone = strings.ToLower(strings.TrimSpace(zone))
	if zone != "" && !zoneName.MatchString(zone) {
		return nil, fmt.Errorf("%q is not a zone short name", zone)
	}
	cols := []string{"id", "zone_short_name", "npc_type_id", "npc_race", "npc_bodytype", "npc_name", "kind", "mode", "time_of_day",
		"player_race", "player_class", "min_level", "max_level", "weight", "cooldown", "text", "enabled", "notes"}
	for _, c := range laterColumns {
		if hasColumn(server, "custom_ambient_line", c.name) {
			cols = append(cols, c.name)
		}
	}
	if err := server.Select(cols).Where("zone_short_name = ? OR zone_short_name = ''", zone).Order("zone_short_name DESC, kind, id").Find(&out.Lines).Error; err != nil {
		return nil, err
	}
	if zone != "" {
		if err := content.Raw(`SELECT DISTINCT n.id, n.name, n.race, n.bodytype, n.level FROM npc_types n
			JOIN spawnentry e ON e.npcID = n.id JOIN spawn2 s ON s.spawngroupID = e.spawngroupID
			WHERE s.zone = ? ORDER BY n.name`, zone).Scan(&out.NPCs).Error; err != nil {
			return nil, err
		}
		for i := range out.NPCs {
			out.NPCs[i].Clean = CleanName(out.NPCs[i].Name)
			out.NPCs[i].Silent = Silent(out.NPCs[i].Race, out.NPCs[i].Bodytype)
		}
	}
	return out, nil
}

// Save writes a line: a new one when its id is 0
func Save(content, server *gorm.DB, l models.CustomAmbientLine) (*models.CustomAmbientLine, error) {
	l.ZoneShortName = strings.ToLower(strings.TrimSpace(l.ZoneShortName))
	l.NpcName = strings.TrimSpace(l.NpcName)
	l.Kind = strings.ToLower(strings.TrimSpace(l.Kind))
	l.Mode = strings.ToLower(strings.TrimSpace(l.Mode))
	l.Audience = strings.ToLower(strings.TrimSpace(l.Audience))
	l.TimeOfDay = strings.ToLower(strings.TrimSpace(l.TimeOfDay))
	l.Text = strings.TrimSpace(l.Text)
	l.Notes = strings.TrimSpace(l.Notes)
	if l.TimeOfDay == "" {
		l.TimeOfDay = "any"
	}
	if l.Weight == 0 {
		l.Weight = 1 // the plugin reads 0 as 1
	}
	if l.Enabled > 1 {
		l.Enabled = 1
	}
	switch {
	case l.ZoneShortName != "" && !zoneName.MatchString(l.ZoneShortName):
		return nil, fmt.Errorf("%q is not a zone short name", l.ZoneShortName)
	case !kinds[l.Kind]:
		return nil, fmt.Errorf("%q is not a kind: idle or approach", l.Kind)
	case !modes[l.Mode]:
		return nil, fmt.Errorf("%q is not a mode: say or emote", l.Mode)
	case !audiences[l.Audience]:
		return nil, fmt.Errorf("%q is not an audience: nearby, player, or empty for by kind", l.Audience)
	case !times[l.TimeOfDay]:
		return nil, fmt.Errorf("%q is not a time: any, day or night", l.TimeOfDay)
	case l.Text == "":
		return nil, errors.New("a line needs its text")
	case len(l.Text) > MaxText:
		return nil, fmt.Errorf("the text is %d bytes: at most %d", len(l.Text), MaxText)
	case !utf8.ValidString(l.Text) || strings.ContainsAny(l.Text, "\r\n\t"):
		return nil, errors.New("the text is one line of plain text")
	case len(l.NpcName) > MaxNpcName:
		return nil, fmt.Errorf("the NPC name is at most %d characters", MaxNpcName)
	case len(l.Notes) > MaxNotes:
		return nil, fmt.Errorf("the notes are at most %d characters", MaxNotes)
	case l.MinLevel > 255 || l.MaxLevel > 255:
		return nil, errors.New("levels are from 0 to 255")
	case l.MinLevel > 0 && l.MaxLevel > 0 && l.MinLevel > l.MaxLevel:
		return nil, errors.New("the lowest level is above the highest")
	}
	// a scene: parts split by |, the plugin dropping empty ones
	parts := 0
	for _, p := range strings.Split(l.Text, "|") {
		if p = strings.TrimSpace(p); p != "" && strings.TrimSpace(strings.TrimPrefix(p, "*")) != "" {
			parts++
		}
	}
	if parts == 0 {
		return nil, errors.New("a line needs its text")
	}
	// the faction band: canonical names, the better standing first
	from, f1, ok1 := standing(l.FactionFrom)
	to, f2, ok2 := standing(l.FactionTo)
	switch {
	case !ok1:
		return nil, fmt.Errorf("%q is not a standing: %v", l.FactionFrom, strings.Join(Standings, ", "))
	case !ok2:
		return nil, fmt.Errorf("%q is not a standing: %v", l.FactionTo, strings.Join(Standings, ", "))
	case f1 > 0 && f2 > 0 && f2 < f1:
		from, to = to, from
	}
	l.FactionFrom, l.FactionTo = from, to
	for _, b := range braces.FindAllString(l.Text, -1) {
		if !fills[strings.ToLower(b)] {
			return nil, fmt.Errorf("%v isn't filled in: the text can use {player}, {race}, {class} and {npc}", b)
		}
	}
	// the plugin fills only lower-case names in
	l.Text = braces.ReplaceAllStringFunc(l.Text, strings.ToLower)

	if l.ZoneShortName != "" {
		var n int64
		content.Raw("SELECT COUNT(*) FROM zone WHERE short_name = ?", l.ZoneShortName).Scan(&n)
		if n == 0 {
			return nil, fmt.Errorf("there is no zone %q", l.ZoneShortName)
		}
	}
	if l.NpcTypeID > 0 {
		var n int64
		content.Raw("SELECT COUNT(*) FROM npc_types WHERE id = ?", l.NpcTypeID).Scan(&n)
		if n == 0 {
			return nil, fmt.Errorf("there is no NPC %d", l.NpcTypeID)
		}
	}
	if !hasTable(server, "custom_ambient_line") {
		return nil, errNoTable
	}
	cols := []string{"zone_short_name", "npc_type_id", "npc_race", "npc_bodytype", "npc_name", "kind", "mode", "time_of_day",
		"player_race", "player_class", "min_level", "max_level", "weight", "cooldown", "text", "enabled", "notes"}
	for _, c := range laterColumns {
		if hasColumn(server, "custom_ambient_line", c.name) {
			cols = append(cols, c.name)
		} else if !c.isDefault(l) {
			return nil, fmt.Errorf("this server's custom_ambient_line has no %v column yet (Starfall custom migration %d, run when world starts on a build that has it)", c.name, c.migration)
		}
	}
	if l.ID == 0 {
		if err := server.Select(cols).Create(&l).Error; err != nil {
			return nil, err
		}
		return &l, nil
	}
	res := server.Model(&models.CustomAmbientLine{}).Where("id = ?", l.ID).Select(cols).Updates(&l)
	if res.Error != nil {
		return nil, res.Error
	}
	var n int64
	server.Model(&models.CustomAmbientLine{}).Where("id = ?", l.ID).Count(&n)
	if n == 0 {
		return nil, fmt.Errorf("there is no line %d", l.ID)
	}
	return &l, nil
}

// Delete removes a line
func Delete(server *gorm.DB, id uint32) error {
	if !hasTable(server, "custom_ambient_line") {
		return errNoTable
	}
	res := server.Where("id = ?", id).Delete(&models.CustomAmbientLine{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("there is no line %d", id)
	}
	return nil
}
