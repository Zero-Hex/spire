package models

type CustomQuestBoard struct {
	ID               uint    `json:"id" gorm:"Column:id"`
	Board            string  `json:"board" gorm:"Column:board"`
	Category         string  `json:"category" gorm:"Column:category"`
	Title            string  `json:"title" gorm:"Column:title"`
	NpcName          string  `json:"npc_name" gorm:"Column:npc_name"`
	NpcZone          string  `json:"npc_zone" gorm:"Column:npc_zone"`
	X                float32 `json:"x" gorm:"Column:x"`
	Y                float32 `json:"y" gorm:"Column:y"`
	Z                float32 `json:"z" gorm:"Column:z"`
	Location         string  `json:"location" gorm:"Column:location"`
	StartText        string  `json:"start_text" gorm:"Column:start_text"`
	Summary          string  `json:"summary" gorm:"Column:summary"`
	Reward           string  `json:"reward" gorm:"Column:reward"`
	RewardItemId     uint    `json:"reward_item_id" gorm:"Column:reward_item_id"`
	MinLevel         uint16  `json:"min_level" gorm:"Column:min_level"`
	MaxLevel         uint16  `json:"max_level" gorm:"Column:max_level"`
	NeedBucket       string  `json:"need_bucket" gorm:"Column:need_bucket"`
	NeedTask         uint    `json:"need_task" gorm:"Column:need_task"`
	NeedFactionId    uint    `json:"need_faction_id" gorm:"Column:need_faction_id"`
	NeedFactionMin   int     `json:"need_faction_min" gorm:"Column:need_faction_min"`
	NeedClasses      string  `json:"need_classes" gorm:"Column:need_classes"`
	NeedRaces        string  `json:"need_races" gorm:"Column:need_races"`
	NeedItem         uint    `json:"need_item" gorm:"Column:need_item"`
	LockedMode       uint8   `json:"locked_mode" gorm:"Column:locked_mode"`
	LockedText       string  `json:"locked_text" gorm:"Column:locked_text"`
	Tier             uint8   `json:"tier" gorm:"Column:tier"`
	SortOrder        int     `json:"sort_order" gorm:"Column:sort_order"`
	Enabled          uint8   `json:"enabled" gorm:"Column:enabled"`
}

func (CustomQuestBoard) TableName() string {
    return "custom_quest_board"
}

func (CustomQuestBoard) Relationships() []string {
    return []string{}
}

func (CustomQuestBoard) Connection() string {
    return ""
}
