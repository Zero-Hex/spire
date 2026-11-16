package zonecards

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
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
)

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
		Styles: hasColumn(server, "custom_zone_card", "title_font")}
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
	// before migration 69 a row has no return_card column: it is on
	for i := range out.Cards {
		if !out.Styles {
			out.Cards[i].ReturnCard = 1
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
	for _, c := range []*string{&card.TitleColor, &card.SubtitleColor, &card.AccentColor} {
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
	if !hasColumn(server, "custom_zone_card", "title_font") {
		// before custom migration 69: the text and switch only
		def := models.CustomZoneCard{ZoneShortName: card.ZoneShortName, Title: card.Title, Subtitle: card.Subtitle, Enabled: card.Enabled, ReturnCard: 1}
		if card != def {
			return nil, errors.New("the server has no style columns yet (Starfall custom migration 69, run when world starts on a build that has it): only the title, subtitle and shown can be saved")
		}
		if err := server.Select("zone_short_name", "title", "subtitle", "enabled").
			Clauses(clause.OnConflict{DoUpdates: clause.AssignmentColumns([]string{"title", "subtitle", "enabled"})}).Create(&card).Error; err != nil {
			return nil, err
		}
		return &card, nil
	}
	if err := server.Clauses(clause.OnConflict{UpdateAll: true}).Create(&card).Error; err != nil {
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
