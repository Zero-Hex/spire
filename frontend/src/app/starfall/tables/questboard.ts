// Starfall task board (custom DB migrations 26, 27, 49), shown by board NPCs
// through plugins/quest_board.pl. Field meanings follow the #questboard GM
// command (zone/gm_commands/questboard.cpp). Nothing is cached: changes show
// the next time a player opens or pages the board.

import {DB_PLAYER_CLASSES} from "@/app/constants/eq-classes-constants";
import {RACES}             from "@/app/constants/eq-race-constants";

const TIERS = [
  { value: 0, text: "None" },
  { value: 1, text: "Common" },
  { value: 2, text: "Uncommon" },
  { value: 3, text: "Rare" },
  { value: 4, text: "Epic" },
  { value: 5, text: "Legendary" },
  { value: 6, text: "Glitch" },
]

const LOCKED_MODES = [
  { value: 0, text: "Hidden until met" },
  { value: 1, text: "Greyed out with a Requires: line" },
]

// "2,6" -> "Cleric, Druid"; unknown ids are flagged
function idListNames(value, names) {
  const ids = String(value || "").split(/[,\s]+/).filter((x) => x.length > 0)
  if (ids.length === 0) {
    return "Any"
  }
  return ids.map((id) => names[id] || "<span class='text-warning'>unknown " + id + "</span>").join(", ")
}

function factionStanding(min) {
  const tiers = [[1100, "Ally"], [750, "Warmly"], [500, "Kindly"], [100, "Amiably"], [0, "Indifferently"], [-100, "Apprehensively"], [-500, "Dubiously"], [-750, "Threateningly"]]
  const t     = tiers.find(([v]) => Number(min) >= v)
  return t ? t[1] : "Scowls"
}

export const QUEST_BOARD = {
  title: "Quests",
  singular: "quest",
  list: "custom_quest_boards",
  entity: "custom_quest_board",
  key: "id",
  autoIncrement: true,
  orderBy: ["board", "category", "sort_order", "id"],
  nameField: "title",
  columns: ["id", "board", "category", "title", "min_level", "max_level", "tier"],
  fields: [
    { name: "title", label: "Title", type: "text", max: 64, required: true, col: 6 },
    { name: "board", label: "Board", type: "text", max: 32, required: true, col: 3, help: "Which board lists it, eg qeynos" },
    { name: "category", label: "Category", type: "text", max: 32, default: "General", col: 3, help: "Heading it is grouped under" },
    { name: "npc_name", label: "Quest giver", type: "text", max: 64, col: 4 },
    { name: "npc_zone", label: "Zone (short name)", type: "text", max: 32, col: 4 },
    { name: "location", label: "Location", type: "text", max: 128, col: 4, help: "Where to find them, in words" },
    { name: "x", label: "X", type: "float", col: 4 },
    { name: "y", label: "Y", type: "float", col: 4 },
    { name: "z", label: "Z", type: "float", col: 4 },
    { name: "start_text", label: "How to start", type: "text", max: 255, col: 12, help: "eg Hail him and ask about [rats]" },
    { name: "summary", label: "Summary", type: "textarea", max: 2000, col: 12 },
    { name: "reward", label: "Reward (in words)", type: "text", max: 255, col: 6 },
    { name: "reward_item_id", label: "Reward item", type: "ref", ref: "item", col: 6 },
    { name: "min_level", label: "Min level", type: "int", default: 1, col: 3 },
    { name: "max_level", label: "Max level", type: "int", col: 3, help: "0 = no cap" },
    { name: "tier", label: "Tier", type: "select", options: TIERS, col: 3, help: "Colours the title and styles the detail window" },
    { name: "sort_order", label: "Sort order", type: "int", col: 2 },
    { name: "enabled", label: "Enabled", type: "bool", default: 1, col: 1 },

    { name: "need_bucket", label: "Requires data bucket", type: "text", max: 100, col: 6, help: "Character data bucket: <code>key</code>, or <code>key=value</code>" },
    { name: "need_task", label: "Requires completed task", type: "ref", ref: "task", col: 6 },
    { name: "need_item", label: "Requires carried item", type: "ref", ref: "item", col: 6 },
    { name: "need_faction_id", label: "Requires faction", type: "ref", ref: "faction", col: 6 },
    {
      name: "need_faction_min", label: "Minimum faction value", type: "int", col: 6,
      help: (row) => "Standing " + factionStanding(row.need_faction_min) + " or better. Ally 1100, Warmly 750, Kindly 500, Amiably 100, Indifferently 0.",
    },
    {
      name: "need_classes", label: "Requires class", type: "text", max: 64, col: 6,
      help: (row) => "Comma-separated class ids. " + idListNames(row.need_classes, DB_PLAYER_CLASSES),
    },
    {
      name: "need_races", label: "Requires race", type: "text", max: 128, col: 6,
      help: (row) => "Comma-separated race ids. " + idListNames(row.need_races, RACES),
    },
    { name: "locked_mode", label: "Until requirements are met", type: "select", options: LOCKED_MODES, col: 6 },
    { name: "locked_text", label: "Requires: text", type: "text", max: 255, col: 12, help: "Shown when greyed out. Blank = generated from the requirements" },
  ],
}
