package models

type CustomAchievementReward struct {
	ID             uint64 `json:"id" gorm:"Column:id"`
	AchievementId  uint   `json:"achievement_id" gorm:"Column:achievement_id"`
	RewardType     string `json:"reward_type" gorm:"Column:reward_type"`
	RewardId       uint   `json:"reward_id" gorm:"Column:reward_id"`
	Amount         uint   `json:"amount" gorm:"Column:amount"`
	Chance         uint   `json:"chance" gorm:"Column:chance"`
	Tier           string `json:"tier" gorm:"Column:tier"`
	ClaimOnce      int8   `json:"claim_once" gorm:"Column:claim_once"`
	AutoClaim      int8   `json:"auto_claim" gorm:"Column:auto_claim"`
	PreviewText    string `json:"preview_text" gorm:"Column:preview_text"`
	DataText       string `json:"data_text" gorm:"Column:data_text"`
	Enabled        int8   `json:"enabled" gorm:"Column:enabled"`
	SortOrder      int    `json:"sort_order" gorm:"Column:sort_order"`
	CreatedAt      uint   `json:"created_at" gorm:"Column:created_at"`
}

func (CustomAchievementReward) TableName() string {
    return "custom_achievement_rewards"
}

func (CustomAchievementReward) Relationships() []string {
    return []string{}
}

func (CustomAchievementReward) Connection() string {
    return ""
}
