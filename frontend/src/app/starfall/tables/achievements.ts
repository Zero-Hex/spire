// Starfall achievements (custom DB migrations 2-5). Field meanings follow
// zone/achievement_manager.cpp and docs/design/achievement_system_reference.md
// in the server repository.

// objective_type is matched against these literals; anything else is
// stored but never completes
const OBJECTIVE_TYPES = [
  { value: "level", text: "level - reach a level", ref: null, help: "Completes when the character's level is at least <b>Required count</b>. Also fires on a persona switch." },
  { value: "zone_visit", text: "zone_visit - enter a zone", ref: null, help: "Completes on zoning into <b>Zone</b>." },
  { value: "task_complete", text: "task_complete - finish a task", ref: "task", help: "<b>Target</b> is the task id." },
  { value: "skill", text: "skill - raise a skill", ref: "skill", help: "<b>Target</b> is the skill id; <b>Required count</b> is the skill value." },
  { value: "quest_signal", text: "quest_signal - signal from a script", ref: null, help: "Completes when a quest script signals <b>Target name</b> (case-insensitive), eg <code>$client->AchievementSignal(\"name\")</code>. Collections fire their <b>Achievement signal</b> this way when completed." },
  { value: "npc_kill", text: "npc_kill - kill an NPC type", ref: "npc", help: "<b>Target</b> is the npc_types id." },
  { value: "npc_name_kill", text: "npc_name_kill - kill by name in a zone", ref: null, help: "Matches <b>Target name</b> (the NPC's clean name, lowercased) in <b>Zone</b>." },
  { value: "zone_kill", text: "zone_kill - kills in a zone", ref: null, help: "Counts kills in <b>Zone</b>." },
  { value: "race_kill", text: "race_kill - kill a race", ref: "race", help: "<b>Target</b> is the race id." },
  { value: "bodytype_kill", text: "bodytype_kill - kill a body type", ref: "bodytype", help: "<b>Target</b> is the body type id." },
  { value: "achievement_complete", text: "achievement_complete - finish another achievement", ref: "achievement", help: "<b>Target</b> is another achievement's id. This is how meta achievements chain." },
]

const REWARD_TYPES = [
  { value: "item", text: "item", help: "<b>Reward id</b> is the item; <b>Amount</b> above 1 is the charge count." },
  { value: "coin", text: "coin", help: "<b>Amount</b> is in copper (1000 = 1 platinum)." },
  { value: "currency", text: "currency", help: "<b>Reward id</b> is the alternate_currency id; <b>Amount</b> is how many." },
  { value: "title_set", text: "title_set", help: "<b>Reward id</b> is the title set to enable." },
  { value: "title_text", text: "title_text", help: "Grants <b>Data text</b> as a title (falls back to <b>Preview text</b>)." },
  { value: "title_suffix", text: "title_suffix", help: "Grants <b>Data text</b> as a suffix (falls back to <b>Preview text</b>)." },
  { value: "live_item_request", text: "live_item_request", help: "Queues a leveled-item request: <b>Amount</b> is the level band, <b>Reward id</b> the item slot, <b>Tier</b> and <b>Data text</b> (theme) are passed along." },
]

function objectiveType(row) {
  return OBJECTIVE_TYPES.find((t) => t.value === row.objective_type)
}

function rewardType(row) {
  return REWARD_TYPES.find((t) => t.value === row.reward_type)
}

export const ACHIEVEMENT_CATEGORIES = {
  title: "Categories",
  singular: "category",
  list: "custom_achievement_categories",
  entity: "custom_achievement_category",
  refType: "achievement_category",
  key: "id",
  autoIncrement: false,
  orderBy: ["sort_order", "id"],
  tree: { parent: "parent_id", label: "name" },
  nameField: "name",
  columns: ["id", "name", "sort_order"],
  deleteWarning: "Achievements in this category stay in the database but no longer show to players.",
  fields: [
    { name: "id", label: "ID", type: "int", col: 3 },
    { name: "name", label: "Name", type: "text", max: 64, required: true, col: 9 },
    { name: "description", label: "Description", type: "text", max: 255, col: 12 },
    { name: "parent_id", label: "Parent category", type: "ref", ref: "achievement_category", col: 12, help: "0 = top level" },
    { name: "sort_order", label: "Sort order", type: "int", col: 4 },
    { name: "icon_id", label: "Icon", type: "int", col: 4 },
    { name: "enabled", label: "Enabled", type: "bool", default: 1, col: 4 },
  ],
}

export const ACHIEVEMENTS = {
  title: "Achievements",
  singular: "achievement",
  list: "custom_achievements",
  entity: "custom_achievement",
  refType: "achievement",
  key: "id",
  autoIncrement: false,
  orderBy: ["sort_order", "id"],
  nameField: "name",
  columns: ["id", "name", "points"],
  deleteWarning: "Its objectives and rewards are not deleted with it, and characters who completed it keep their record.",
  validate: (row, rows) => {
    if (!row.slug) {
      return "Slug is required (it must be unique)"
    }
    return ""
  },
  fields: [
    { name: "id", label: "ID", type: "int", col: 3, help: "Opaque; only needs to be unique" },
    { name: "name", label: "Name", type: "text", max: 96, required: true, col: 5 },
    { name: "slug", label: "Slug", type: "text", max: 64, col: 4, help: "Unique key, never shown" },
    { name: "description", label: "Description", type: "text", max: 255, col: 12 },
    { name: "category_id", label: "Category", type: "ref", ref: "achievement_category", col: 6 },
    { name: "points", label: "Points", type: "int", col: 3 },
    { name: "sort_order", label: "Sort order", type: "int", col: 3 },
    { name: "enabled", label: "Enabled", type: "bool", default: 1, col: 3, help: "Disabled never shows or completes" },
    { name: "hidden", label: "Hidden", type: "bool", col: 3, help: "Not listed to players" },
    { name: "repeatable", label: "Repeatable", type: "bool", col: 3 },
    { name: "reward_item_id", label: "Legacy reward item", type: "ref", ref: "item", col: 6, help: "Legacy columns still pay out; prefer the Rewards table" },
    { name: "reward_title_set", label: "Legacy title set", type: "int", col: 4 },
    { name: "reward_currency_id", label: "Legacy currency id", type: "int", col: 4 },
    { name: "reward_currency_amount", label: "Legacy currency amount", type: "int", col: 4 },
    { name: "created_at", label: "Created (unix time)", type: "int", col: 4, default: () => Math.floor(Date.now() / 1000) },
  ],
}

export const ACHIEVEMENT_OBJECTIVES = {
  title: "Objectives",
  singular: "objective",
  list: "custom_achievement_objectives",
  entity: "custom_achievement_objective",
  key: "id",
  autoIncrement: false,
  orderBy: ["objective_index", "id"],
  nameField: "target_name",
  columns: ["objective_index", "objective_type", "target_name", "required_count", "optional"],
  help: "An achievement completes when every objective that is not <b>Optional</b> is done.",
  fields: [
    { name: "id", label: "ID", type: "int", col: 3 },
    { name: "achievement_id", label: "Achievement", type: "ref", ref: "achievement", col: 5 },
    { name: "objective_index", label: "Order", type: "int", col: 4 },
    {
      name: "objective_type", label: "Type", type: "select", col: 12, default: "npc_kill",
      options: OBJECTIVE_TYPES.map((t) => ({ value: t.value, text: t.text })),
      help: (row) => (objectiveType(row) || { help: "<span class='text-warning'>Not a type the server checks: this objective can never complete.</span>" }).help,
    },
    {
      name: "target_id", label: "Target", col: 6,
      type: "ref", ref: (row) => (objectiveType(row) || {}).ref || null,
      showIf: (row) => !!(objectiveType(row) || {}).ref,
    },
    { name: "target_name", label: "Target name", type: "text", max: 128, col: 6, help: "Shown to players as the objective; the signal for quest_signal, the NPC name for npc_name_kill" },
    { name: "target_type", label: "Target label", type: "text", max: 32, col: 4, help: "Cosmetic" },
    { name: "required_count", label: "Required count", type: "int", default: 1, col: 4 },
    { name: "zone_id", label: "Zone", type: "ref", ref: "zone", col: 4 },
    { name: "class_mask", label: "Class mask", type: "int", col: 4 },
    { name: "optional", label: "Optional", type: "bool", col: 4, help: "Does not count toward completion" },
  ],
}

export const ACHIEVEMENT_REWARDS = {
  title: "Rewards",
  singular: "reward",
  list: "custom_achievement_rewards",
  entity: "custom_achievement_reward",
  key: "id",
  autoIncrement: true,
  orderBy: ["sort_order", "id"],
  nameField: "preview_text",
  columns: ["reward_type", "reward_id", "amount", "chance", "auto_claim"],
  help: "Rolled on completion. Unless <b>Auto claim</b> is set, rewards wait for the player to claim them.",
  fields: [
    { name: "achievement_id", label: "Achievement", type: "ref", ref: "achievement", col: 6 },
    {
      name: "reward_type", label: "Type", type: "select", col: 6, default: "item",
      options: REWARD_TYPES.map((t) => ({ value: t.value, text: t.text })),
      help: (row) => (rewardType(row) || { help: "" }).help,
    },
    {
      name: "reward_id", label: "Reward id", col: 6,
      type: "ref", ref: (row) => (row.reward_type === "item" ? "item" : null),
    },
    { name: "amount", label: "Amount", type: "int", default: 1, col: 3 },
    {
      name: "chance", label: "Chance", type: "int", default: 10000, col: 3,
      help: (row) => "Out of 10000: " + ((Number(row.chance) || 0) / 100) + "%",
    },
    { name: "tier", label: "Tier", type: "text", max: 32, col: 4 },
    { name: "preview_text", label: "Preview text", type: "text", max: 255, col: 8, help: "What the player sees; blank = generated from the type" },
    { name: "data_text", label: "Data text", type: "text", max: 255, col: 12 },
    { name: "auto_claim", label: "Auto claim", type: "bool", col: 3 },
    { name: "claim_once", label: "Claim once", type: "bool", default: 1, col: 3 },
    { name: "enabled", label: "Enabled", type: "bool", default: 1, col: 3 },
    { name: "sort_order", label: "Sort order", type: "int", col: 3 },
    { name: "created_at", label: "Created (unix time)", type: "int", col: 4, default: () => Math.floor(Date.now() / 1000) },
  ],
}
