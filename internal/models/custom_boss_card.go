package models

// CustomBossCard is a Starfall boss title card (custom migration 74,
// zone/boss_cards.cpp): a boss NPC type announces itself to nearby players
// with the zone-card look, on its first pull or on spawn.
type CustomBossCard struct {
	NpcTypeID     uint32 `json:"npc_type_id" gorm:"Column:npc_type_id;primaryKey;autoIncrement:false"`
	Title         string `json:"title" gorm:"Column:title"` // empty: the NPC's own name
	Subtitle      string `json:"subtitle" gorm:"Column:subtitle"`
	Trigger       string `json:"trigger" gorm:"Column:trigger"` // engage, spawn
	Radius        uint32 `json:"radius" gorm:"Column:radius"`
	TitleFont     string `json:"title_font" gorm:"Column:title_font"`
	TitleColor    string `json:"title_color" gorm:"Column:title_color"`
	SubtitleColor string `json:"subtitle_color" gorm:"Column:subtitle_color"`
	AccentColor   string `json:"accent_color" gorm:"Column:accent_color"`
	EmblemIcon    uint32 `json:"emblem_icon" gorm:"Column:emblem_icon"`
	Reveal        string `json:"reveal" gorm:"Column:reveal"` // fade, rise, letters ('' reads as fade)
	DisplayMs     uint32 `json:"display_ms" gorm:"Column:display_ms"`
	CameraMs      uint32 `json:"camera_ms" gorm:"Column:camera_ms"`       // 0: no shot (custom migration 75)
	CameraMode    string `json:"camera_mode" gorm:"Column:camera_mode"`   // '' auto, keep, first, top, close, side, far, front (76)
	CameraStyle   string `json:"camera_style" gorm:"Column:camera_style"` // snap (portrait), live (77)
	Shroud        uint8  `json:"shroud" gorm:"Column:shroud"`
	ShroudColor   string `json:"shroud_color" gorm:"Column:shroud_color"`
	Sound         string `json:"sound" gorm:"Column:sound"`
	RepeatSeconds uint32 `json:"repeat_seconds" gorm:"Column:repeat_seconds"`
	Enabled       uint8  `json:"enabled" gorm:"Column:enabled"`
	Notes         string `json:"notes" gorm:"Column:notes"`
}

func (CustomBossCard) TableName() string {
	return "custom_boss_card"
}

func (CustomBossCard) Relationships() []string {
	return []string{}
}

func (CustomBossCard) Connection() string {
	return ""
}
