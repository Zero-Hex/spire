package models

type CustomMobInspectVisibility struct {
	NpcId          uint   `json:"npc_id" gorm:"Column:npc_id"`
	HideAll        int8   `json:"hide_all" gorm:"Column:hide_all"`
	HideStats      int8   `json:"hide_stats" gorm:"Column:hide_stats"`
	HideAbilities  int8   `json:"hide_abilities" gorm:"Column:hide_abilities"`
	HideLoot       int8   `json:"hide_loot" gorm:"Column:hide_loot"`
	Note           string `json:"note" gorm:"Column:note"`
}

func (CustomMobInspectVisibility) TableName() string {
    return "custom_mob_inspect_visibility"
}

func (CustomMobInspectVisibility) Relationships() []string {
    return []string{}
}

func (CustomMobInspectVisibility) Connection() string {
    return ""
}
