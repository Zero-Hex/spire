package models

type CustomAchievement struct {
	ID                     uint   `json:"id" gorm:"Column:id"`
	CategoryId             uint   `json:"category_id" gorm:"Column:category_id"`
	Slug                   string `json:"slug" gorm:"Column:slug"`
	Name                   string `json:"name" gorm:"Column:name"`
	Description            string `json:"description" gorm:"Column:description"`
	Points                 uint   `json:"points" gorm:"Column:points"`
	Hidden                 int8   `json:"hidden" gorm:"Column:hidden"`
	Repeatable             int8   `json:"repeatable" gorm:"Column:repeatable"`
	RewardTitleSet         uint   `json:"reward_title_set" gorm:"Column:reward_title_set"`
	RewardItemId           uint   `json:"reward_item_id" gorm:"Column:reward_item_id"`
	RewardCurrencyId       uint   `json:"reward_currency_id" gorm:"Column:reward_currency_id"`
	RewardCurrencyAmount   uint   `json:"reward_currency_amount" gorm:"Column:reward_currency_amount"`
	Enabled                int8   `json:"enabled" gorm:"Column:enabled"`
	SortOrder              int    `json:"sort_order" gorm:"Column:sort_order"`
	CreatedAt              uint   `json:"created_at" gorm:"Column:created_at"`
}

func (CustomAchievement) TableName() string {
    return "custom_achievements"
}

func (CustomAchievement) Relationships() []string {
    return []string{}
}

func (CustomAchievement) Connection() string {
    return ""
}
