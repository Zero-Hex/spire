package models

type CustomAchievementCategory struct {
	ID          uint   `json:"id" gorm:"Column:id"`
	ParentId    uint   `json:"parent_id" gorm:"Column:parent_id"`
	Name        string `json:"name" gorm:"Column:name"`
	Description string `json:"description" gorm:"Column:description"`
	SortOrder   int    `json:"sort_order" gorm:"Column:sort_order"`
	IconId      uint   `json:"icon_id" gorm:"Column:icon_id"`
	Enabled     int8   `json:"enabled" gorm:"Column:enabled"`
}

func (CustomAchievementCategory) TableName() string {
    return "custom_achievement_categories"
}

func (CustomAchievementCategory) Relationships() []string {
    return []string{}
}

func (CustomAchievementCategory) Connection() string {
    return ""
}
