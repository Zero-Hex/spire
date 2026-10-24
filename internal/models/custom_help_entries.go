package models

type CustomHelpEntry struct {
	ID          uint   `json:"id" gorm:"Column:id"`
	CategoryId  uint   `json:"category_id" gorm:"Column:category_id"`
	Title       string `json:"title" gorm:"Column:title"`
	Body        string `json:"body" gorm:"Column:body"`
	SortOrder   int    `json:"sort_order" gorm:"Column:sort_order"`
	Enabled     int8   `json:"enabled" gorm:"Column:enabled"`
}

func (CustomHelpEntry) TableName() string {
    return "custom_help_entries"
}

func (CustomHelpEntry) Relationships() []string {
    return []string{}
}

func (CustomHelpEntry) Connection() string {
    return ""
}
