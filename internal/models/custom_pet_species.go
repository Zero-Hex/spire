package models

import (
	"github.com/volatiletech/null/v8"
)

type CustomPetSpecie struct {
	ID                       uint        `json:"id" gorm:"Column:id"`
	Name                     string      `json:"name" gorm:"Column:name"`
	Description              string      `json:"description" gorm:"Column:description"`
	Enabled                  int8        `json:"enabled" gorm:"Column:enabled"`
	RaceId                   uint        `json:"race_id" gorm:"Column:race_id"`
	Gender                   uint8       `json:"gender" gorm:"Column:gender"`
	Texture                  uint8       `json:"texture" gorm:"Column:texture"`
	Helmtexture              uint8       `json:"helmtexture" gorm:"Column:helmtexture"`
	Size                     float32     `json:"size" gorm:"Column:size"`
	BaseLevel                uint        `json:"base_level" gorm:"Column:base_level"`
	MaxLevel                 uint        `json:"max_level" gorm:"Column:max_level"`
	BaseHp                   uint        `json:"base_hp" gorm:"Column:base_hp"`
	BaseMana                 uint        `json:"base_mana" gorm:"Column:base_mana"`
	BaseAc                   uint        `json:"base_ac" gorm:"Column:base_ac"`
	BaseAtk                  uint        `json:"base_atk" gorm:"Column:base_atk"`
	BaseMinDmg               uint        `json:"base_min_dmg" gorm:"Column:base_min_dmg"`
	BaseMaxDmg               uint        `json:"base_max_dmg" gorm:"Column:base_max_dmg"`
	BaseResist               uint        `json:"base_resist" gorm:"Column:base_resist"`
	Bodytype                 uint8       `json:"bodytype" gorm:"Column:bodytype"`
	UnlockFlag               string      `json:"unlock_flag" gorm:"Column:unlock_flag"`
	UnlockHint               null.String `json:"unlock_hint" gorm:"Column:unlock_hint"`
	PetClass                 uint8       `json:"pet_class" gorm:"Column:pet_class"`
	RequiredClass            uint8       `json:"required_class" gorm:"Column:required_class"`
	LevelGroupId             uint        `json:"level_group_id" gorm:"Column:level_group_id"`
	UseBeastlordRaceTable    int8        `json:"use_beastlord_race_table" gorm:"Column:use_beastlord_race_table"`
	NpcSpellsId              uint        `json:"npc_spells_id" gorm:"Column:npc_spells_id"`
	NpcTypeId                uint        `json:"npc_type_id" gorm:"Column:npc_type_id"`
	BaseStr                  uint        `json:"base_str" gorm:"Column:base_str"`
	BaseSta                  uint        `json:"base_sta" gorm:"Column:base_sta"`
	BaseDex                  uint        `json:"base_dex" gorm:"Column:base_dex"`
	BaseAgi                  uint        `json:"base_agi" gorm:"Column:base_agi"`
	BaseInt                  uint        `json:"base_int" gorm:"Column:base_int"`
	BaseWis                  uint        `json:"base_wis" gorm:"Column:base_wis"`
	BaseCha                  uint        `json:"base_cha" gorm:"Column:base_cha"`
	SortOrder                int         `json:"sort_order" gorm:"Column:sort_order"`
}

func (CustomPetSpecie) TableName() string {
    return "custom_pet_species"
}

func (CustomPetSpecie) Relationships() []string {
    return []string{}
}

func (CustomPetSpecie) Connection() string {
    return ""
}
