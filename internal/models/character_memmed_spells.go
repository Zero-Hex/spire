package models

type CharacterMemmedSpell struct {
	ID       uint   `json:"id" gorm:"Column:id"`
	ClassId  uint8  `json:"class_id" gorm:"Column:class_id"`
	SlotId   uint16 `json:"slot_id" gorm:"Column:slot_id"`
	SpellId  uint16 `json:"spell_id" gorm:"Column:spell_id"`
}

func (CharacterMemmedSpell) TableName() string {
    return "character_memmed_spells"
}

func (CharacterMemmedSpell) Relationships() []string {
    return []string{}
}

func (CharacterMemmedSpell) Connection() string {
    return ""
}
