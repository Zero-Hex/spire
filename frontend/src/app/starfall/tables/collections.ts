// Starfall collections (custom DB migration 6 on). Progress is per account:
// players deposit items from the cursor and every character on the account
// shares it. Field meanings follow zone/collection_manager.cpp.

export const COLLECTION_CATEGORIES = {
  title: "Categories",
  singular: "category",
  list: "custom_collection_categories",
  entity: "custom_collection_category",
  refType: "collection_category",
  key: "id",
  autoIncrement: true,
  orderBy: ["sort_order", "id"],
  tree: { parent: "parent_id", label: "name" },
  nameField: "name",
  columns: ["id", "name", "sort_order"],
  deleteWarning: "Collections in this category stay in the database but no longer show to players.",
  fields: [
    { name: "name", label: "Name", type: "text", max: 80, required: true, col: 12 },
    { name: "parent_id", label: "Parent category", type: "ref", ref: "collection_category", col: 12, help: "0 = top level" },
    { name: "sort_order", label: "Sort order", type: "int", col: 6 },
    { name: "enabled", label: "Enabled", type: "bool", default: 1, col: 6 },
  ],
}

export const COLLECTIONS = {
  title: "Collections",
  singular: "collection",
  list: "custom_collections",
  entity: "custom_collection",
  refType: "collection",
  key: "id",
  autoIncrement: true,
  orderBy: ["sort_order", "id"],
  nameField: "name",
  columns: ["id", "name", "points", "achievement_signal"],
  deleteWarning: "Its items are not deleted with it, and accounts that completed it keep their record.",
  fields: [
    { name: "name", label: "Name", type: "text", max: 128, required: true, col: 8 },
    { name: "points", label: "Points", type: "int", col: 4 },
    { name: "description", label: "Description", type: "text", max: 255, col: 12 },
    { name: "category_id", label: "Category", type: "ref", ref: "collection_category", col: 6 },
    {
      name: "achievement_signal", label: "Achievement signal", type: "text", max: 80, col: 6,
      help: "Fired on completion as a quest_signal: an achievement objective of type quest_signal with this Target name completes. Blank = none.",
    },
    { name: "sort_order", label: "Sort order", type: "int", col: 4 },
    { name: "enabled", label: "Enabled", type: "bool", default: 1, col: 4 },
    { name: "hidden", label: "Hidden", type: "bool", col: 4, help: "Not listed to players" },
  ],
}

export const COLLECTION_ITEMS = {
  title: "Items",
  singular: "item",
  list: "custom_collection_items",
  entity: "custom_collection_item",
  key: "id",
  autoIncrement: true,
  orderBy: ["sort_order", "id"],
  columns: ["item_id", "required_count"],
  help: "Players deposit these from their cursor. One item can count toward several collections at once.",
  validate: (row, rows, isNew) => {
    const dupe = rows.find((r) => r.item_id === Number(row.item_id) && (isNew || r.id !== row.id))
    return dupe ? "That item is already in this collection" : ""
  },
  fields: [
    { name: "item_id", label: "Item", type: "ref", ref: "item", required: true, col: 12 },
    { name: "collection_id", label: "Collection", type: "ref", ref: "collection", col: 12 },
    { name: "required_count", label: "Required count", type: "int", default: 1, col: 6 },
    { name: "sort_order", label: "Sort order", type: "int", col: 6 },
  ],
}

