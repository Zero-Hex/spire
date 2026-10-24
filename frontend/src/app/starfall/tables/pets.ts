// Starfall pet progression (custom DB migrations 6-18, 54). Field meanings
// follow zone/pet_progression_manager.h in the server repository. The zone
// caches these tables: changes apply after `#reload pet_species global`.

const GENDERS = [
  { value: 0, text: "Male" },
  { value: 1, text: "Female" },
  { value: 2, text: "Neuter" },
]

const STAT = (name, label, def = 75) => ({ name: name, label: label, type: "int", default: def, col: 3 })
const GROWTH = (name, label) => ({ name: name, label: label, type: "float", col: 3 })

export const PET_SPECIES = {
  title: "Species",
  singular: "species",
  list: "custom_pet_species",
  entity: "custom_pet_specie",
  key: "id",
  autoIncrement: true,
  orderBy: ["sort_order", "id"],
  nameField: "name",
  columns: ["id", "name", "required_class", "level_group_id", "unlock_flag"],
  deleteWarning: "Characters who have this species keep their rows in custom_character_pets.",
  fields: [
    { name: "name", label: "Name", type: "text", max: 64, required: true, col: 6 },
    { name: "sort_order", label: "Sort order", type: "int", col: 3 },
    { name: "enabled", label: "Enabled", type: "bool", default: 1, col: 3 },
    { name: "description", label: "Description", type: "text", max: 255, col: 12 },
    {
      name: "unlock_flag", label: "Unlock flag", type: "text", max: 80, col: 6,
      help: "Blank = every character has it. Anything else locks it until a script grants it with <code>$client->GrantPetSpecies(id)</code>; the text is only a label.",
    },
    { name: "unlock_hint", label: "Unlock hint", type: "text", max: 160, nullable: true, col: 6, help: "Shown instead of the description while locked. Blank = a generic hint." },
    { name: "required_class", label: "Required owner class", type: "ref", ref: "class", col: 6, help: "0 = any class. Only checked when summoning with #pet." },
    { name: "level_group_id", label: "Level group", type: "int", col: 6, help: "Species with the same non-zero group share one level and exp (eg the four elementals)." },
    { name: "pet_class", label: "Pet class", type: "ref", ref: "class", default: 1, col: 6, help: "The class the pet fights as: its special attacks (Rogue backstab, Monk kicks)." },
    { name: "npc_type_id", label: "NPC template", type: "ref", ref: "npc", col: 6, help: "0 = built from the columns here. Otherwise that npc_types row is the pet at its base level: appearance, class, spells and base stats come from it, and growth still applies." },
    { name: "npc_spells_id", label: "Spell list", type: "ref", ref: "npc_spells", col: 6, help: "0 = no spells" },
    { name: "bodytype", label: "Body type", type: "ref", ref: "bodytype", default: 1, col: 6 },
    { name: "race_id", label: "Race", type: "ref", ref: "race", col: 6 },
    { name: "gender", label: "Gender", type: "select", options: GENDERS, default: 2, col: 3 },
    { name: "size", label: "Size", type: "float", col: 3, help: "0 = race default" },
    { name: "texture", label: "Texture", type: "int", col: 3 },
    { name: "helmtexture", label: "Helm texture", type: "int", col: 3 },
    {
      name: "use_beastlord_race_table", label: "Beastlord appearance", type: "bool", col: 6,
      help: "Take race, texture and size from pets_beastlord_data by the owner's race, as Beastlord warders do. The columns above become the fallback.",
    },
    { name: "base_level", label: "Base level", type: "int", default: 1, col: 3 },
    { name: "max_level", label: "Max level", type: "int", default: 60, col: 3 },
    STAT("base_hp", "Base HP", 100),
    STAT("base_mana", "Base mana", 0),
    STAT("base_ac", "Base AC", 10),
    STAT("base_atk", "Base attack", 10),
    STAT("base_min_dmg", "Base min damage", 1),
    STAT("base_max_dmg", "Base max damage", 4),
    STAT("base_resist", "Base resists", 10),
    STAT("base_str", "STR"),
    STAT("base_sta", "STA"),
    STAT("base_dex", "DEX"),
    STAT("base_agi", "AGI"),
    STAT("base_int", "INT"),
    STAT("base_wis", "WIS"),
    STAT("base_cha", "CHA"),
  ],
}

export const PET_SPECIES_GROWTH = {
  title: "Growth per level",
  singular: "growth row",
  list: "custom_pet_species_growths",
  entity: "custom_pet_species_growth",
  key: "species_id",
  keyReadonly: true,
  autoIncrement: false,
  maxRows: 1,
  columns: ["hp_per_level", "ac_per_level", "atk_per_level", "max_dmg_per_level"],
  help: "Added per level above the base level, times the owner's class multiplier. A species without a row does not grow.",
  fields: [
    { name: "species_id", label: "Species", type: "int", col: 12 },
    GROWTH("hp_per_level", "HP"),
    GROWTH("mana_per_level", "Mana"),
    GROWTH("ac_per_level", "AC"),
    GROWTH("atk_per_level", "Attack"),
    GROWTH("min_dmg_per_level", "Min damage"),
    GROWTH("max_dmg_per_level", "Max damage"),
    GROWTH("resist_per_level", "Resists"),
    GROWTH("str_per_level", "STR"),
    GROWTH("sta_per_level", "STA"),
    GROWTH("dex_per_level", "DEX"),
    GROWTH("agi_per_level", "AGI"),
    GROWTH("int_per_level", "INT"),
    GROWTH("wis_per_level", "WIS"),
    GROWTH("cha_per_level", "CHA"),
  ],
}

export const PET_CLASS_GROWTH = {
  title: "Owner class growth",
  singular: "class multiplier",
  list: "custom_pet_class_growths",
  entity: "custom_pet_class_growth",
  key: "class_id",
  keyReadonly: true,
  chooseKey: true,
  autoIncrement: false,
  orderBy: ["class_id"],
  columns: ["class_id", "growth_multiplier"],
  help: "Multiplies every species' growth for owners of that class. Classes without a row use 1.0.",
  fields: [
    { name: "class_id", label: "Owner class", type: "ref", ref: "class", col: 8 },
    { name: "growth_multiplier", label: "Multiplier", type: "float", default: 1, col: 4 },
  ],
}
