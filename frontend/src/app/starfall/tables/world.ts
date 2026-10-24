// Small per-NPC and per-faction Starfall tables. Both are read live by the
// zone (zone/starfall_link_mob_inspect.cpp, zone/faction_window.cpp).

export const MOB_INSPECT_VISIBILITY = {
  title: "Mob inspect visibility",
  singular: "NPC",
  list: "custom_mob_inspect_visibilities",
  entity: "custom_mob_inspect_visibility",
  key: "npc_id",
  keyReadonly: true,
  chooseKey: true,
  autoIncrement: false,
  orderBy: ["npc_id"],
  nameField: "note",
  columns: ["npc_id", "hide_all", "hide_stats", "hide_abilities", "hide_loot", "note"],
  help: "What players may see when they inspect an NPC. An NPC with no row shows everything the Custom:Mob_Inspect_* rules allow. GMs with /gm on see everything.",
  fields: [
    { name: "npc_id", label: "NPC", type: "ref", ref: "npc", col: 12 },
    { name: "hide_all", label: "Hide everything", type: "bool", col: 3, help: "Refuses the inspect with the same line as a mob with no spawn point" },
    { name: "hide_stats", label: "Hide stats", type: "bool", col: 3, help: "HP, mana, AC, damage, resists" },
    { name: "hide_abilities", label: "Hide abilities", type: "bool", col: 3, help: "Special abilities" },
    { name: "hide_loot", label: "Hide loot", type: "bool", col: 3 },
    { name: "note", label: "Note", type: "text", max: 255, col: 12, help: "For whoever maintains this; not shown in game" },
  ],
}

export const FACTION_WINDOW_PINNED = {
  title: "Pinned factions",
  singular: "pinned faction",
  list: "faction_window_pinneds",
  entity: "faction_window_pinned",
  key: "faction_id",
  keyReadonly: true,
  chooseKey: true,
  autoIncrement: false,
  orderBy: ["faction_id"],
  nameField: "label",
  columns: ["faction_id", "label"],
  help: "Listed in every player's faction window even before they have touched them, at their starting standing. Meant for a curated set such as the starting-city factions.",
  fields: [
    { name: "faction_id", label: "Faction", type: "ref", ref: "faction", col: 12 },
    { name: "label", label: "Label", type: "text", max: 64, col: 12, help: "For whoever maintains this list; the window shows the faction's own name" },
  ],
}
