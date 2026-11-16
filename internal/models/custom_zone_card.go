package models

// CustomZoneCard is a Starfall zone title card's text (custom migration 68,
// zone/zone_cards.cpp): every zone gets a card with its long name; a row sets
// a different title and/or a subtitle, or hides the zone's card.
type CustomZoneCard struct {
	ZoneShortName string `json:"zone_short_name" gorm:"Column:zone_short_name;primaryKey"`
	Title         string `json:"title" gorm:"Column:title"`
	Subtitle      string `json:"subtitle" gorm:"Column:subtitle"`
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
