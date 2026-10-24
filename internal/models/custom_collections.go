package models

type CustomCollection struct {
	ID                 uint   `json:"id" gorm:"Column:id"`
	CategoryId         uint   `json:"category_id" gorm:"Column:category_id"`
	Name               string `json:"name" gorm:"Column:name"`
	Description        string `json:"description" gorm:"Column:description"`
	Points             uint   `json:"points" gorm:"Column:points"`
	AchievementSignal  string `json:"achievement_signal" gorm:"Column:achievement_signal"`
	Enabled            int8   `json:"enabled" gorm:"Column:enabled"`
	Hidden             int8   `json:"hidden" gorm:"Column:hidden"`
	SortOrder          int    `json:"sort_order" gorm:"Column:sort_order"`
}

func (CustomCollection) TableName() string {
    return "custom_collections"
}

func (CustomCollection) Relationships() []string {
    return []string{}
}

func (CustomCollection) Connection() string {
    return ""
}
