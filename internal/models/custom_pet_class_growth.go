package models

type CustomPetClassGrowth struct {
	ClassId           uint    `json:"class_id" gorm:"Column:class_id"`
	GrowthMultiplier  float32 `json:"growth_multiplier" gorm:"Column:growth_multiplier"`
}

func (CustomPetClassGrowth) TableName() string {
    return "custom_pet_class_growth"
}

func (CustomPetClassGrowth) Relationships() []string {
    return []string{}
}

func (CustomPetClassGrowth) Connection() string {
    return ""
}
