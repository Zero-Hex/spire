package models

type CustomPetSpeciesGrowth struct {
	SpeciesId         uint    `json:"species_id" gorm:"Column:species_id"`
	HpPerLevel        float32 `json:"hp_per_level" gorm:"Column:hp_per_level"`
	ManaPerLevel      float32 `json:"mana_per_level" gorm:"Column:mana_per_level"`
	AcPerLevel        float32 `json:"ac_per_level" gorm:"Column:ac_per_level"`
	AtkPerLevel       float32 `json:"atk_per_level" gorm:"Column:atk_per_level"`
	MinDmgPerLevel    float32 `json:"min_dmg_per_level" gorm:"Column:min_dmg_per_level"`
	MaxDmgPerLevel    float32 `json:"max_dmg_per_level" gorm:"Column:max_dmg_per_level"`
	ResistPerLevel    float32 `json:"resist_per_level" gorm:"Column:resist_per_level"`
	StrPerLevel       float32 `json:"str_per_level" gorm:"Column:str_per_level"`
	StaPerLevel       float32 `json:"sta_per_level" gorm:"Column:sta_per_level"`
	DexPerLevel       float32 `json:"dex_per_level" gorm:"Column:dex_per_level"`
	AgiPerLevel       float32 `json:"agi_per_level" gorm:"Column:agi_per_level"`
	IntPerLevel       float32 `json:"int_per_level" gorm:"Column:int_per_level"`
	WisPerLevel       float32 `json:"wis_per_level" gorm:"Column:wis_per_level"`
	ChaPerLevel       float32 `json:"cha_per_level" gorm:"Column:cha_per_level"`
}

func (CustomPetSpeciesGrowth) TableName() string {
    return "custom_pet_species_growth"
}

func (CustomPetSpeciesGrowth) Relationships() []string {
    return []string{}
}

func (CustomPetSpeciesGrowth) Connection() string {
    return ""
}
