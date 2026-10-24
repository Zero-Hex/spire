package models

type FactionWindowPinned struct {
	FactionId  uint   `json:"faction_id" gorm:"Column:faction_id"`
	Label      string `json:"label" gorm:"Column:label"`
}

func (FactionWindowPinned) TableName() string {
    return "faction_window_pinned"
}

func (FactionWindowPinned) Relationships() []string {
    return []string{}
}

func (FactionWindowPinned) Connection() string {
    return ""
}
