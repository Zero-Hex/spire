import {SpireApi}          from "@/app/api/spire-api";
import {SpireQueryBuilder} from "@/app/api/spire-query-builder";
import {ROUTE}             from "@/routes";
import util                from "util";
import {RACES}             from "@/app/constants/eq-race-constants";
import {BODYTYPES}         from "@/app/constants/eq-bodytype-constants";
import {DB_SKILLS}         from "@/app/constants/eq-skill-constants";
import {DB_PLAYER_CLASSES} from "@/app/constants/eq-classes-constants";

/**
 * References a Starfall editor field can point at: another table looked up
 * through the API (an item, an NPC), or a fixed list of constants (a race).
 * A field shows the referenced name beside its id and can search by name.
 */
export type StarfallRef = {
  label: string
  list: string                      // API list route, eg "items" ("" for constants)
  id: string                        // id column in that table
  name: string                      // name column, shown and searched
  link?: (id: number) => string     // Spire page for the referenced row
  constants?: { [key: string]: string }
}

function classNames() {
  const names = {}
  for (const [id, c] of Object.entries(DB_PLAYER_CLASSES)) {
    // @ts-ignore
    names[id] = c.class
  }
  return names
}

export const STARFALL_REFS: { [key: string]: StarfallRef } = {
  item: {label: "Item", list: "items", id: "id", name: "name", link: (id) => util.format(ROUTE.ITEM_EDIT, id)},
  npc: {label: "NPC", list: "npc_types", id: "id", name: "name", link: (id) => ROUTE.NPC_EDIT.replace(":npc", String(id))},
  faction: {label: "Faction", list: "faction_lists", id: "id", name: "name"},
  task: {label: "Task", list: "tasks", id: "id", name: "title", link: (id) => util.format(ROUTE.TASK_EDIT, id)},
  zone: {label: "Zone", list: "zones", id: "zoneidnumber", name: "long_name"},
  npc_spells: {label: "NPC spell set", list: "npc_spells", id: "id", name: "name", link: (id) => util.format(ROUTE.NPC_SPELL_EDIT, id)},
  achievement: {label: "Achievement", list: "custom_achievements", id: "id", name: "name"},
  achievement_category: {label: "Achievement category", list: "custom_achievement_categories", id: "id", name: "name"},
  race: {label: "Race", list: "", id: "", name: "", constants: RACES},
  bodytype: {label: "Body type", list: "", id: "", name: "", constants: BODYTYPES},
  skill: {label: "Skill", list: "", id: "", name: "", constants: DB_SKILLS},
  class: {label: "Class", list: "", id: "", name: "", constants: classNames()},
}

// names already looked up, per ref type
const nameCache: { [key: string]: { [id: string]: string } } = {}

export class StarfallRefs {
  static get(type: string): StarfallRef | undefined {
    return STARFALL_REFS[type]
  }

  /**
   * Drops cached names after the referenced table was edited
   */
  static forget(type: string) {
    delete nameCache[type]
  }

  /**
   * Name of the referenced row, or "" when there is none
   */
  static async nameOf(type: string, id: number): Promise<string> {
    const ref = STARFALL_REFS[type]
    if (!ref || !id) {
      return ""
    }

    if (ref.constants) {
      return ref.constants[String(id)] || ""
    }

    nameCache[type] = nameCache[type] || {}
    if (typeof nameCache[type][id] !== "undefined") {
      return nameCache[type][id]
    }

    try {
      const r = await SpireApi.v1().get(ref.list, {
        params: (new SpireQueryBuilder())
          .where(ref.id, "=", id)
          .select([ref.id, ref.name])
          .limit(1)
          .get()
      })
      const name = r.data && r.data.length > 0 ? (r.data[0][ref.name] || "") : ""
      nameCache[type][id] = name
      return name
    } catch (e) {
      return ""
    }
  }

  /**
   * Names for many ids at once, for list columns. Uses the bulk endpoint
   * when the reference is keyed by its primary key, one lookup per id
   * otherwise (zones are keyed by zoneidnumber)
   */
  static async names(type: string, ids: number[]): Promise<{ [id: string]: string }> {
    const ref    = STARFALL_REFS[type]
    const wanted = Array.from(new Set(ids.filter((id) => id > 0)))
    const out    = {}
    if (!ref || wanted.length === 0) {
      return out
    }

    nameCache[type] = nameCache[type] || {}
    const missing   = wanted.filter((id) => typeof nameCache[type][id] === "undefined")

    if (missing.length > 0 && !ref.constants) {
      if (ref.id === "id") {
        try {
          const r = await SpireApi.v1().post(ref.list + "/bulk", {ids: missing}, {
            params: (new SpireQueryBuilder()).select([ref.id, ref.name]).get()
          })
          for (const row of (r.data || [])) {
            nameCache[type][row[ref.id]] = row[ref.name] || ""
          }
        } catch (e) {
          // names are a convenience; the ids still show
        }
      } else {
        await Promise.all(missing.map((id) => StarfallRefs.nameOf(type, id)))
      }
    }

    for (const id of wanted) {
      out[id] = ref.constants ? (ref.constants[String(id)] || "") : (nameCache[type][id] || "")
    }

    return out
  }

  /**
   * Up to 15 rows whose name contains the search text (or whose id is it)
   */
  static async search(type: string, text: string): Promise<{ id: number, name: string }[]> {
    const ref = STARFALL_REFS[type]
    if (!ref || !text) {
      return []
    }

    if (ref.constants) {
      const needle = text.toLowerCase()
      return Object.entries(ref.constants)
        .filter(([id, name]) => id === text || String(name).toLowerCase().includes(needle))
        .slice(0, 15)
        .map(([id, name]) => ({id: parseInt(id), name: String(name)}))
    }

    const q = (new SpireQueryBuilder()).select([ref.id, ref.name]).limit(15)
    if (/^\d+$/.test(text)) {
      q.where(ref.id, "=", text)
    } else {
      // "." and "_" are delimiters in the where param, so search on the
      // longest run of text between them (NPC names use "_" for spaces)
      const words = text.split(/[._\s]+/).filter((w) => w.length > 0)
      if (words.length === 0) {
        return []
      }
      q.where(ref.name, "like", words.reduce((a, b) => (b.length > a.length ? b : a)))
    }

    try {
      const r = await SpireApi.v1().get(ref.list, {params: q.get()})
      return (r.data || []).map((row) => ({id: row[ref.id], name: row[ref.name] || ""}))
    } catch (e) {
      return []
    }
  }
}
