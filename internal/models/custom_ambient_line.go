package models

// CustomAmbientLine is a Starfall ambient line (custom migrations 71-73,
// plugins/ambient.pl): idle chatter or a greeting for NPCs near players.
// 0 / empty in a filter means "any".
type CustomAmbientLine struct {
	ID            uint32 `json:"id" gorm:"Column:id;primaryKey;autoIncrement"`
	ZoneShortName string `json:"zone_short_name" gorm:"Column:zone_short_name"` // empty: every zone
	NpcTypeID     uint32 `json:"npc_type_id" gorm:"Column:npc_type_id"`
	NpcRace       uint32 `json:"npc_race" gorm:"Column:npc_race"`
	NpcBodytype   uint32 `json:"npc_bodytype" gorm:"Column:npc_bodytype"`
	NpcName       string `json:"npc_name" gorm:"Column:npc_name"` // part of the NPC's name
	Kind          string `json:"kind" gorm:"Column:kind"`         // idle, approach
	Mode          string `json:"mode" gorm:"Column:mode"`         // say, emote
	Audience      string `json:"audience" gorm:"Column:audience"` // nearby, player; empty: by kind (72)
	TimeOfDay     string `json:"time_of_day" gorm:"Column:time_of_day"`
	PlayerRace    uint32 `json:"player_race" gorm:"Column:player_race"`
	PlayerClass   uint32 `json:"player_class" gorm:"Column:player_class"`
	MinLevel      uint32 `json:"min_level" gorm:"Column:min_level"`
	MaxLevel      uint32 `json:"max_level" gorm:"Column:max_level"`
	FactionFrom   string `json:"faction_from" gorm:"Column:faction_from"` // ally .. scowls; empty: any (73)
	FactionTo     string `json:"faction_to" gorm:"Column:faction_to"`
	Weight        uint32 `json:"weight" gorm:"Column:weight"`
	Cooldown      uint32 `json:"cooldown" gorm:"Column:cooldown"` // seconds; 0: 120
	Text          string `json:"text" gorm:"Column:text"`         // parts split by | play as a scene; *part: an emote
	Enabled       uint8  `json:"enabled" gorm:"Column:enabled"`
	Notes         string `json:"notes" gorm:"Column:notes"`
}

func (CustomAmbientLine) TableName() string {
	return "custom_ambient_line"
}

func (CustomAmbientLine) Relationships() []string {
	return []string{}
}

func (CustomAmbientLine) Connection() string {
	return ""
}
