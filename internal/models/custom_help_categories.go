package models

type CustomHelpCategory struct {
	ID         uint   `json:"id" gorm:"Column:id"`
	ParentId   uint   `json:"parent_id" gorm:"Column:parent_id"`
	Name       string `json:"name" gorm:"Column:name"`
	SortOrder  int    `json:"sort_order" gorm:"Column:sort_order"`
	Enabled    int8   `json:"enabled" gorm:"Column:enabled"`
}

func (CustomHelpCategory) TableName() string {
    return "custom_help_categories"
}

func (CustomHelpCategory) Relationships() []string {
    return []string{}
}

func (CustomHelpCategory) Connection() string {
    return ""
}
