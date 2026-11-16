package models

// CustomZoneCard is a Starfall zone title card (custom migrations 68 and 69,
// zone/zone_cards.cpp): every zone gets a card with its long name; a row sets
// a different title and/or a subtitle, hides the zone's card, or styles it.
// Empty strings and zeros are the defaults.
type CustomZoneCard struct {
	ZoneShortName string `json:"zone_short_name" gorm:"Column:zone_short_name;primaryKey"`
	Title         string `json:"title" gorm:"Column:title"`
	Subtitle      string `json:"subtitle" gorm:"Column:subtitle"`
	TitleFont     string `json:"title_font" gorm:"Column:title_font"`         // cinzel (default), pirata, arimo
	TitleColor    string `json:"title_color" gorm:"Column:title_color"`       // RRGGBB
	SubtitleColor string `json:"subtitle_color" gorm:"Column:subtitle_color"` // RRGGBB
	AccentColor   string `json:"accent_color" gorm:"Column:accent_color"`     // RRGGBB: the rules
	EmblemIcon    uint32 `json:"emblem_icon" gorm:"Column:emblem_icon"`       // an item icon above the title
	Reveal        string `json:"reveal" gorm:"Column:reveal"`                 // fade (default), rise, letters
	DisplayMs     uint32 `json:"display_ms" gorm:"Column:display_ms"`         // 0: 4500; at most 15000
	ReturnCard    uint8  `json:"return_card" gorm:"Column:return_card"`       // 0: no small card on return visits
	Sound         string `json:"sound" gorm:"Column:sound"`                   // an MP3 in the client folder
	SoundOnReturn uint8  `json:"sound_on_return" gorm:"Column:sound_on_return"`
	Enabled       uint8  `json:"enabled" gorm:"Column:enabled"`
}

func (CustomZoneCard) TableName() string {
	return "custom_zone_card"
}

func (CustomZoneCard) Relationships() []string {
	return []string{}
}

func (CustomZoneCard) Connection() string {
	return ""
}
