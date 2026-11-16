package zonecards

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/EQEmu/spire/internal/clientassets"
	"github.com/EQEmu/spire/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Zone title cards (starfall-3 zone/zone_cards.cpp, docs/design/zone_cards.md):
// every zone gets a card with its long name; a custom_zone_card row sets a
// different title and/or a subtitle line, or hides the zone's card. The
// server sends the DLL at most 63 bytes of title and 127 of subtitle
// (common/starfall_link_wire.h kMaxTitle / kMaxSubtitle, less the end).

const (
	MaxTitle     = 63
	MaxSubtitle  = 127
	MaxDisplayMs = 15000 // kMaxDisplayMs
	MaxSound     = 64    // the sound column
	MaxShroud    = 90    // kMaxShroudPct
	// a row's shroud when it doesn't set one (the column default); zones with
	// no row use Custom:Zone_Card_Default_Shroud
	RowShroud = 45
)

// the columns later migrations add, their defaults, and the migration
var laterColumns = []struct {
	name      string
	migration int
	isDefault func(c models.CustomZoneCard) bool
}{
	{"title_font", 69, func(c models.CustomZoneCard) bool { return c.TitleFont == "" }},
	{"title_color", 69, func(c models.CustomZoneCard) bool { return c.TitleColor == "" }},
	{"subtitle_color", 69, func(c models.CustomZoneCard) bool { return c.SubtitleColor == "" }},
	{"accent_color", 69, func(c models.CustomZoneCard) bool { return c.AccentColor == "" }},
	{"emblem_icon", 69, func(c models.CustomZoneCard) bool { return c.EmblemIcon == 0 }},
	{"reveal", 69, func(c models.CustomZoneCard) bool { return c.Reveal == "" }},
	{"display_ms", 69, func(c models.CustomZoneCard) bool { return c.DisplayMs == 0 }},
	{"return_card", 69, func(c models.CustomZoneCard) bool { return c.ReturnCard == 1 }},
	{"sound", 69, func(c models.CustomZoneCard) bool { return c.Sound == "" }},
	{"sound_on_return", 69, func(c models.CustomZoneCard) bool { return c.SoundOnReturn == 0 }},
	{"shroud", 70, func(c models.CustomZoneCard) bool { return c.Shroud == RowShroud }},
	{"shroud_color", 70, func(c models.CustomZoneCard) bool { return c.ShroudColor == "" }},
}

var (
	fonts     = map[string]bool{"": true, "cinzel": true, "pirata": true, "arimo": true}
	reveals   = map[string]bool{"": true, "fade": true, "rise": true, "letters": true}
	hexColor  = regexp.MustCompile(`^[0-9A-Fa-f]{6}$`)
	soundPath = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_ ./\\-]*\.[Mm][Pp]3$`)
)

type Zone struct {
	ShortName string `json:"short_name"`
	LongName  string `json:"long_name"`
	ID        int    `json:"zoneidnumber"`
}

type List struct {
	Zones []Zone                  `json:"zones"`
	Cards []models.CustomZoneCard `json:"cards"`
	// characters who have entered each zone (custom_zone_discovered): they
	// have had its first-entry card
	Discovered map[string]int `json:"discovered"`
	// the server has the style columns (custom migration 69)
	Styles bool `json:"styles"`
	// and the shroud ones (70)
	Shroud bool `json:"shroud"`
	// Custom:Zone_Card_Default_Shroud: zones with no row
	DefaultShroud int `json:"default_shroud"`
	// MP3s Spire keeps for the client patch, by their path in the client folder
	Sounds []string `json:"sounds"`
}

var zoneName = regexp.MustCompile(`^[a-z0-9_]+$`)

func hasColumn(db *gorm.DB, table, column string) bool {
	var n int64
	db.Raw("SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?", table, column).Scan(&n)
	return n > 0
}

func hasTable(db *gorm.DB, table string) bool {
	var n int64
	db.Raw("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?", table).Scan(&n)
	return n > 0
}

var errNoTable = errors.New("there is no custom_zone_card table: the server makes it (Starfall custom migration 68) when world starts on a build with zone cards")

// Read lists every zone (content) with the cards and discovery counts
// (server database)
func Read(content, server *gorm.DB) (*List, error) {
	if !hasTable(server, "custom_zone_card") {
		return nil, errNoTable
	}
	out := &List{Zones: []Zone{}, Cards: []models.CustomZoneCard{}, Discovered: map[string]int{}, Sounds: []string{},
		Styles: hasColumn(server, "custom_zone_card", "title_font"), Shroud: hasColumn(server, "custom_zone_card", "shroud"),
		DefaultShroud: defaultShroud(server)}
	// one row per zone: version 0's name where there is one
	if err := content.Raw(`SELECT z.short_name, z.long_name, z.zoneidnumber AS id FROM zone z
		JOIN (SELECT short_name, MIN(version) AS version FROM zone WHERE short_name <> '' GROUP BY short_name) v
		ON v.short_name = z.short_name AND v.version = z.version
		GROUP BY z.short_name ORDER BY z.long_name`).Scan(&out.Zones).Error; err != nil {
		return nil, err
	}
	if err := server.Order("zone_short_name").Find(&out.Cards).Error; err != nil {
		return nil, err
	}
	// before migrations 69 and 70 a row has no return_card or shroud
	// columns: it gets their defaults
	for i := range out.Cards {
		if !out.Styles {
			out.Cards[i].ReturnCard = 1
		}
		if !out.Shroud {
			out.Cards[i].Shroud = RowShroud
		}
	}
	if hasTable(server, "custom_zone_discovered") {
		var counts []struct {
			ZoneShortName string
			N             int
		}
		server.Raw("SELECT zone_short_name, COUNT(*) AS n FROM custom_zone_discovered GROUP BY zone_short_name").Scan(&counts)
		for _, c := range counts {
			out.Discovered[c.ZoneShortName] = c.N
		}
	}
	return out, nil
}

// Save writes a zone's card
func Save(content, server *gorm.DB, card models.CustomZoneCard) (*models.CustomZoneCard, error) {
	card.ZoneShortName = strings.ToLower(strings.TrimSpace(card.ZoneShortName))
	card.Title = strings.TrimSpace(card.Title)
	card.Subtitle = strings.TrimSpace(card.Subtitle)
	switch {
	case !zoneName.MatchString(card.ZoneShortName):
		return nil, fmt.Errorf("%q is not a zone short name", card.ZoneShortName)
	case len(card.Title) > MaxTitle:
		return nil, fmt.Errorf("the title is %d bytes: players see at most %d", len(card.Title), MaxTitle)
	case len(card.Subtitle) > MaxSubtitle:
		return nil, fmt.Errorf("the subtitle is %d bytes: players see at most %d", len(card.Subtitle), MaxSubtitle)
	case !utf8.ValidString(card.Title) || !utf8.ValidString(card.Subtitle):
		return nil, errors.New("the text isn't valid UTF-8")
	case strings.ContainsAny(card.Title+card.Subtitle, "\r\n\t"):
		return nil, errors.New("a card is one line of title and one of subtitle")
	}
	card.TitleFont = strings.ToLower(strings.TrimSpace(card.TitleFont))
	card.Reveal = strings.ToLower(strings.TrimSpace(card.Reveal))
	card.Sound = strings.TrimSpace(card.Sound)
	for _, c := range []*string{&card.TitleColor, &card.SubtitleColor, &card.AccentColor, &card.ShroudColor} {
		*c = strings.ToUpper(strings.TrimPrefix(strings.TrimSpace(*c), "#"))
		if *c != "" && !hexColor.MatchString(*c) {
			return nil, fmt.Errorf("%q is not a colour: hex RRGGBB, or empty for the default", *c)
		}
	}
	switch {
	case !fonts[card.TitleFont]:
		return nil, fmt.Errorf("%q is not a font: cinzel, pirata or arimo", card.TitleFont)
	case !reveals[card.Reveal]:
		return nil, fmt.Errorf("%q is not a reveal: fade, rise or letters", card.Reveal)
	case card.DisplayMs > MaxDisplayMs:
		return nil, fmt.Errorf("a card stays up at most %d ms", MaxDisplayMs)
	case card.Shroud > MaxShroud:
		return nil, fmt.Errorf("the shroud dims the scene at most %d%%", MaxShroud)
	case card.Sound != "" && (len(card.Sound) > MaxSound || !soundPath.MatchString(card.Sound) || strings.Contains(card.Sound, "..")):
		return nil, fmt.Errorf("%q is not an MP3 in the client folder (at most %d characters)", card.Sound, MaxSound)
	}
	if card.TitleFont == "cinzel" {
		card.TitleFont = "" // the default
	}
	if card.Reveal == "fade" {
		card.Reveal = ""
	}
	if card.Enabled > 1 {
		card.Enabled = 1
	}
	if card.ReturnCard > 1 {
		card.ReturnCard = 1
	}
	if card.SoundOnReturn > 1 {
		card.SoundOnReturn = 1
	}
	var n int64
	content.Raw("SELECT COUNT(*) FROM zone WHERE short_name = ?", card.ZoneShortName).Scan(&n)
	if n == 0 {
		return nil, fmt.Errorf("there is no zone %q", card.ZoneShortName)
	}
	if !hasTable(server, "custom_zone_card") {
		return nil, errNoTable
	}
	// the columns this server has: one a later migration adds is written
	// only once it's there, and can't be set before
	cols := []string{"title", "subtitle", "enabled"}
	for _, c := range laterColumns {
		if hasColumn(server, "custom_zone_card", c.name) {
			cols = append(cols, c.name)
		} else if !c.isDefault(card) {
			return nil, fmt.Errorf("this server's custom_zone_card has no %v column yet (Starfall custom migration %d, run when world starts on a build that has it)", c.name, c.migration)
		}
	}
	if err := server.Select(append([]string{"zone_short_name"}, cols...)).
		Clauses(clause.OnConflict{DoUpdates: clause.AssignmentColumns(cols)}).Create(&card).Error; err != nil {
		return nil, err
	}
	return &card, nil
}

// Delete removes a zone's row: its card goes back to its long name, no
// subtitle, shown
func Delete(server *gorm.DB, zone string) error {
	if !zoneName.MatchString(zone) {
		return fmt.Errorf("%q is not a zone short name", zone)
	}
	if !hasTable(server, "custom_zone_card") {
		return errNoTable
	}
	return server.Where("zone_short_name = ?", zone).Delete(&models.CustomZoneCard{}).Error
}

// Sounds is the MP3s Spire keeps for the client patch (not ones it deletes
// from players' clients), by their path in the client folder
func Sounds(store *clientassets.Store) []string {
	out := []string{}
	if store == nil {
		return out
	}
	for _, f := range store.Index().Files {
		if f.Size > 0 && !f.Remove && strings.HasSuffix(strings.ToLower(f.Path), ".mp3") {
			out = append(out, f.Path)
		}
	}
	sort.Strings(out)
	return out
}

// defaultShroud is Custom:Zone_Card_Default_Shroud (45 unless set)
func defaultShroud(server *gorm.DB) int {
	v := RowShroud
	if hasTable(server, "rule_values") {
		var rows []struct{ RuleValue string }
		server.Raw("SELECT rule_value FROM rule_values WHERE ruleset_id = 1 AND rule_name = 'Custom:Zone_Card_Default_Shroud' LIMIT 1").Scan(&rows)
		if len(rows) == 1 {
			if n, err := strconv.Atoi(strings.TrimSpace(rows[0].RuleValue)); err == nil {
				v = n
			}
		}
	}
	return min(max(v, 0), MaxShroud)
}
