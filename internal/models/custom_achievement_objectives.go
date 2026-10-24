package models

type CustomAchievementObjective struct {
	ID              uint   `json:"id" gorm:"Column:id"`
	AchievementId   uint   `json:"achievement_id" gorm:"Column:achievement_id"`
	ObjectiveIndex  uint   `json:"objective_index" gorm:"Column:objective_index"`
	ObjectiveType   string `json:"objective_type" gorm:"Column:objective_type"`
	TargetType      string `json:"target_type" gorm:"Column:target_type"`
	TargetId        uint   `json:"target_id" gorm:"Column:target_id"`
	TargetName      string `json:"target_name" gorm:"Column:target_name"`
	RequiredCount   uint   `json:"required_count" gorm:"Column:required_count"`
	ZoneId          uint   `json:"zone_id" gorm:"Column:zone_id"`
	ClassMask       uint   `json:"class_mask" gorm:"Column:class_mask"`
	Optional        int8   `json:"optional" gorm:"Column:optional"`
}

func (CustomAchievementObjective) TableName() string {
    return "custom_achievement_objectives"
}

func (CustomAchievementObjective) Relationships() []string {
    return []string{}
}

func (CustomAchievementObjective) Connection() string {
    return ""
}
