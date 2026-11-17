package zonecards

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/EQEmu/spire/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Boss title cards (starfall-3 zone/boss_cards.cpp, docs/design/boss_cards.md):
// a boss NPC type in custom_boss_card announces itself to the players near it
// with the zone-card look, on the first pull of each fight or on spawn. Its
// card is always timed: it never holds the keyboard or waits for input.

const (
	MaxRadius = 5000
	MaxNotes  = 255
)

var (
	triggers    = map[string]bool{"engage": true, "spawn": true}
	bossReveals = map[string]bool{"fade": true, "rise": true, "letters": true}
	errNoBoss   = errors.New("there is no custom_boss_card table: the server makes it (Starfall custom migration 74) when world starts on a build with boss cards")
)

// BossNPC is a boss's NPC, as the editor shows it
type BossNPC struct {
	ID    uint32 `json:"id"`
	Name  string `json:"name"`
	Level int    `json:"level"`
	Race  int    `json:"race"`
	Zones string `json:"zones"` // where it spawns
	Found bool   `json:"found"` // npc_types has it
}

type Boss struct {
	models.CustomBossCard
	NPC BossNPC `json:"npc"`
}

type BossList struct {
	Bosses []Boss `json:"bosses"`
}

// npcInfo is each NPC's name and where it spawns
func npcInfo(content *gorm.DB, ids []uint32) map[uint32]BossNPC {
	out := map[uint32]BossNPC{}
	if len(ids) == 0 {
		return out
	}
	var rows []BossNPC
	content.Raw(`SELECT n.id, n.name, n.level, n.race, COALESCE(GROUP_CONCAT(DISTINCT s.zone ORDER BY s.zone SEPARATOR ', '), '') AS zones
		FROM npc_types n LEFT JOIN spawnentry e ON e.npcID = n.id LEFT JOIN spawn2 s ON s.spawngroupID = e.spawngroupID
		WHERE n.id IN ? GROUP BY n.id`, ids).Scan(&rows)
	for _, r := range rows {
		r.Found = true
		out[r.ID] = r
	}
	return out
}

// ReadBosses lists the boss cards with their NPCs
func ReadBosses(content, server *gorm.DB) (*BossList, error) {
	if !hasTable(server, "custom_boss_card") {
		return nil, errNoBoss
	}
	var rows []models.CustomBossCard
	if err := server.Order("npc_type_id").Find(&rows).Error; err != nil {
		return nil, err
	}
	var ids []uint32
	for _, r := range rows {
		ids = append(ids, r.NpcTypeID)
	}
	info := npcInfo(content, ids)
	out := &BossList{Bosses: []Boss{}}
	for _, r := range rows {
		n, ok := info[r.NpcTypeID]
		if !ok {
			n = BossNPC{ID: r.NpcTypeID}
		}
		out.Bosses = append(out.Bosses, Boss{CustomBossCard: r, NPC: n})
	}
	return out, nil
}

// SearchNPCs finds NPCs to give a card, by name or id
func SearchNPCs(content *gorm.DB, q string) ([]BossNPC, error) {
	q = strings.TrimSpace(q)
	out := []BossNPC{}
	if q == "" {
		return out, nil
	}
	var ids []uint32
	like := "%" + strings.ReplaceAll(q, " ", "_") + "%"
	if err := content.Raw(`SELECT id FROM npc_types WHERE id = ? OR name LIKE ?
		ORDER BY id = ? DESC, raid_target DESC, rare_spawn DESC, LENGTH(name), id LIMIT 25`, q, like, q).Scan(&ids).Error; err != nil {
		return nil, err
	}
	info := npcInfo(content, ids)
	for _, id := range ids {
		out = append(out, info[id])
	}
	return out, nil
}

// SaveBoss writes a boss's card
func SaveBoss(content, server *gorm.DB, b models.CustomBossCard) (*models.CustomBossCard, error) {
	b.Title = strings.TrimSpace(b.Title)
	b.Subtitle = strings.TrimSpace(b.Subtitle)
	b.TitleFont = strings.ToLower(strings.TrimSpace(b.TitleFont))
	b.Reveal = strings.ToLower(strings.TrimSpace(b.Reveal))
	b.Trigger = strings.ToLower(strings.TrimSpace(b.Trigger))
	b.Sound = strings.TrimSpace(b.Sound)
	b.Notes = strings.TrimSpace(b.Notes)
	if b.Reveal == "" {
		b.Reveal = "fade" // what the server reads an empty one as
	}
	if b.Trigger == "" {
		b.Trigger = "engage"
	}
	for _, c := range []*string{&b.TitleColor, &b.SubtitleColor, &b.AccentColor, &b.ShroudColor} {
		*c = strings.ToUpper(strings.TrimPrefix(strings.TrimSpace(*c), "#"))
		if *c != "" && !hexColor.MatchString(*c) {
			return nil, fmt.Errorf("%q is not a colour: hex RRGGBB, or empty for the default", *c)
		}
	}
	switch {
	case b.NpcTypeID == 0:
		return nil, errors.New("pick the boss's NPC")
	case len(b.Title) > MaxTitle:
		return nil, fmt.Errorf("the title is %d bytes: players see at most %d", len(b.Title), MaxTitle)
	case len(b.Subtitle) > MaxSubtitle:
		return nil, fmt.Errorf("the epithet is %d bytes: players see at most %d", len(b.Subtitle), MaxSubtitle)
	case !utf8.ValidString(b.Title) || !utf8.ValidString(b.Subtitle) || strings.ContainsAny(b.Title+b.Subtitle, "\r\n\t"):
		return nil, errors.New("a card is one line of title and one of epithet")
	case !fonts[b.TitleFont]:
		return nil, fmt.Errorf("%q is not a font: cinzel, pirata or arimo", b.TitleFont)
	case !bossReveals[b.Reveal]:
		return nil, fmt.Errorf("%q is not a reveal: fade, rise or letters", b.Reveal)
	case !triggers[b.Trigger]:
		return nil, fmt.Errorf("%q is not a trigger: engage or spawn", b.Trigger)
	case b.DisplayMs > MaxDisplayMs:
		return nil, fmt.Errorf("a card stays up at most %d ms", MaxDisplayMs)
	case b.Shroud > MaxShroud:
		return nil, fmt.Errorf("the shroud dims the scene at most %d%%", MaxShroud)
	case b.Radius == 0 || b.Radius > MaxRadius:
		return nil, fmt.Errorf("the radius is from 1 to %d", MaxRadius)
	case b.Sound != "" && (len(b.Sound) > MaxSound || !soundPath.MatchString(b.Sound) || strings.Contains(b.Sound, "..")):
		return nil, fmt.Errorf("%q is not an MP3 in the client folder (at most %d characters)", b.Sound, MaxSound)
	case len(b.Notes) > MaxNotes:
		return nil, fmt.Errorf("the notes are at most %d characters", MaxNotes)
	}
	if b.TitleFont == "cinzel" {
		b.TitleFont = ""
	}
	if b.Enabled > 1 {
		b.Enabled = 1
	}
	var n int64
	content.Raw("SELECT COUNT(*) FROM npc_types WHERE id = ?", b.NpcTypeID).Scan(&n)
	if n == 0 {
		return nil, fmt.Errorf("there is no NPC %d", b.NpcTypeID)
	}
	if !hasTable(server, "custom_boss_card") {
		return nil, errNoBoss
	}
	if err := server.Clauses(clause.OnConflict{UpdateAll: true}).Create(&b).Error; err != nil {
		return nil, err
	}
	return &b, nil
}

// DeleteBoss removes a boss's card
func DeleteBoss(server *gorm.DB, npcTypeID uint32) error {
	if !hasTable(server, "custom_boss_card") {
		return errNoBoss
	}
	res := server.Where("npc_type_id = ?", npcTypeID).Delete(&models.CustomBossCard{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("NPC %d has no boss card", npcTypeID)
	}
	return nil
}
