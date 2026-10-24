// Starfall help window (custom DB migration 20), read by zone/help_window.cpp
// each time a player opens or browses it.

export const HELP_CATEGORIES = {
  title: "Categories",
  singular: "category",
  list: "custom_help_categories",
  entity: "custom_help_category",
  refType: "help_category",
  key: "id",
  autoIncrement: false,
  orderBy: ["sort_order", "id"],
  tree: { parent: "parent_id", label: "name" },
  nameField: "name",
  columns: ["id", "name", "sort_order"],
  deleteWarning: "Entries in this category stay in the database but no longer show to players.",
  fields: [
    { name: "id", label: "ID", type: "int", col: 3 },
    { name: "name", label: "Name", type: "text", max: 64, required: true, col: 9 },
    { name: "parent_id", label: "Parent category", type: "ref", ref: "help_category", col: 12, help: "0 = top level" },
    { name: "sort_order", label: "Sort order", type: "int", col: 6 },
    { name: "enabled", label: "Enabled", type: "bool", default: 1, col: 6 },
  ],
}

export const HELP_ENTRIES = {
  title: "Entries",
  singular: "entry",
  list: "custom_help_entries",
  entity: "custom_help_entry",
  key: "id",
  autoIncrement: false,
  orderBy: ["sort_order", "id"],
  nameField: "title",
  columns: ["id", "title"],
  fields: [
    { name: "id", label: "ID", type: "int", col: 3 },
    { name: "title", label: "Title", type: "text", max: 96, required: true, col: 9 },
    {
      name: "body", label: "Body", type: "textarea", max: 900, col: 12,
      help: "Line breaks show as written. The window sends at most 900 characters and cuts the rest with \"...\".",
    },
    { name: "category_id", label: "Category", type: "ref", ref: "help_category", col: 12 },
    { name: "sort_order", label: "Sort order", type: "int", col: 6 },
    { name: "enabled", label: "Enabled", type: "bool", default: 1, col: 6 },
  ],
}
