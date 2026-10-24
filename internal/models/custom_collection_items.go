package models

type CustomCollectionItem struct {
	ID             uint `json:"id" gorm:"Column:id"`
	CollectionId   uint `json:"collection_id" gorm:"Column:collection_id"`
	ItemId         uint `json:"item_id" gorm:"Column:item_id"`
	RequiredCount  uint `json:"required_count" gorm:"Column:required_count"`
	SortOrder      int  `json:"sort_order" gorm:"Column:sort_order"`
}

func (CustomCollectionItem) TableName() string {
    return "custom_collection_items"
}

func (CustomCollectionItem) Relationships() []string {
    return []string{}
}

func (CustomCollectionItem) Connection() string {
    return ""
}
