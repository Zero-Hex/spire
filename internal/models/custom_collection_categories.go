package models

type CustomCollectionCategory struct {
	ID         uint   `json:"id" gorm:"Column:id"`
	ParentId   uint   `json:"parent_id" gorm:"Column:parent_id"`
	Name       string `json:"name" gorm:"Column:name"`
	SortOrder  int    `json:"sort_order" gorm:"Column:sort_order"`
	Enabled    int8   `json:"enabled" gorm:"Column:enabled"`
}

func (CustomCollectionCategory) TableName() string {
    return "custom_collection_categories"
}

func (CustomCollectionCategory) Relationships() []string {
    return []string{}
}

func (CustomCollectionCategory) Connection() string {
    return ""
}
