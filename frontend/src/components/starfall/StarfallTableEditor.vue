<template>
  <eq-window-simple :title="title || def.title" class="starfall-table-editor">
    <small class="text-muted d-block mb-2" v-if="def.help" v-html="def.help"></small>

    <div class="d-flex align-items-center">
      <b-form-input
        size="sm"
        v-model="search"
        placeholder="Filter"
        class="mr-2"
      />
      <b-button size="sm" variant="outline-warning" @click="startNew()" :disabled="!canCreate()">
        <i class="fa fa-plus"></i> New
      </b-button>
    </div>

    <div class="text-center mt-3" v-if="loading">Loading...</div>

    <div
      class="mt-2 starfall-list"
      :style="'max-height: ' + listHeight + 'px'"
      v-if="!loading && rows.length > 0"
    >
      <table class="eq-table eq-highlight-rows" style="display: table; width: 100%">
        <thead>
        <tr>
          <th v-for="c in columns()" :key="c.name">{{ c.label }}</th>
        </tr>
        </thead>
        <tbody>
        <tr
          v-for="row in filteredRows()"
          :key="keyOf(row)"
          :class="isSelected(row) ? 'pulsate-highlight-white' : ''"
          @click="select(row)"
          style="cursor: pointer"
        >
          <td v-for="c in columns()" :key="c.name" :style="disabledStyle(row)">
            {{ display(row, c) }}
          </td>
        </tr>
        </tbody>
      </table>
    </div>

    <div class="text-center mt-3 text-muted" v-if="!loading && rows.length === 0">
      {{ emptyText || "Nothing here yet" }}
    </div>

    <!-- edit / create form -->
    <div v-if="editing" class="mt-3 starfall-form fade-in">
      <div class="font-weight-bold mb-2">
        {{ isNew ? "New " + (def.singular || "row") : "Edit " + (def.singular || "row") + " " + keyOf(original) }}
      </div>

      <div class="row">
        <div
          v-for="f in visibleFields()"
          :key="f.name"
          :class="'col-lg-' + (f.col || 6) + ' col-sm-12 mb-2'"
        >
          <starfall-field
            :field="f"
            :row="editing"
            :readonly="!isNew && f.name === def.key && def.keyReadonly"
          />
        </div>
      </div>

      <div class="mt-2">
        <b-button size="sm" variant="outline-warning" @click="save()">
          <i class="fa fa-save"></i> {{ isNew ? "Create" : "Save" }}
        </b-button>
        <b-button size="sm" class="ml-2" variant="outline-secondary" @click="cancel()">
          Cancel
        </b-button>
        <b-button size="sm" class="ml-2" variant="outline-danger" @click="remove()" v-if="!isNew">
          <i class="fa fa-trash"></i> Delete
        </b-button>
      </div>
    </div>

    <info-error-banner
      class="mt-3"
      :notification="notification"
      :error="error"
      @dismiss-error="error = ''"
      @dismiss-notification="notification = ''"
    />
  </eq-window-simple>
</template>

<script>
import EqWindowSimple      from "@/components/eq-ui/EQWindowSimple";
import InfoErrorBanner     from "@/components/InfoErrorBanner";
import StarfallField       from "@/components/starfall/StarfallField";
import {SpireApi}          from "@/app/api/spire-api";
import {SpireQueryBuilder} from "@/app/api/spire-query-builder";
import {StarfallRefs}      from "@/app/starfall/starfall-refs";

/**
 * List + form editor for one Starfall table, driven by a table definition
 * (see app/starfall/tables). `filter` pins columns to values: the list only
 * shows matching rows and new rows are created with them, which is how a
 * page nests one table under another (objectives under an achievement).
 */
export default {
  name: "StarfallTableEditor",
  components: { EqWindowSimple, InfoErrorBanner, StarfallField },
  props: {
    def: { type: Object, required: true },
    filter: { type: Object, default: () => ({}) },
    // true when the parent row the filter depends on is not chosen yet
    waiting: { type: Boolean, default: false },
    title: { type: String, default: "" },
    emptyText: { type: String, default: "" },
    listHeight: { type: Number, default: 400 },
  },
  data() {
    return {
      rows: [],
      loading: false,
      search: "",
      selectedKey: null,
      editing: null,
      original: null,
      isNew: false,
      refNames: {},
      error: "",
      notification: "",
    }
  },
  watch: {
    filter: {
      // pages pass the filter as an inline object, which is a new object on
      // every parent render; only a change of contents resets the list
      handler(now, before) {
        if (JSON.stringify(now) === JSON.stringify(before)) {
          return
        }
        this.selectedKey = null
        this.cancel()
        this.load()
      },
      deep: true,
    },
    waiting() {
      this.load()
    },
  },
  methods: {
    keyOf(row) {
      return row ? row[this.def.key] : null
    },
    isSelected(row) {
      return this.selectedKey !== null && this.keyOf(row) === this.selectedKey
    },
    canCreate() {
      return !this.waiting && !(this.def.maxRows && this.rows.length >= this.def.maxRows)
    },
    columns() {
      return this.def.columns.map((name) => {
        const f = this.def.fields.find((x) => x.name === name) || { name: name, label: name }
        return f
      })
    },
    visibleFields() {
      return this.def.fields.filter((f) => !f.showIf || f.showIf(this.editing))
    },
    disabledStyle(row) {
      return typeof row.enabled !== "undefined" && row.enabled === 0 ? "opacity: .5" : ""
    },
    display(row, c) {
      const v = row[c.name]
      if (c.type === "bool") {
        return v ? "Yes" : ""
      }
      if (c.type === "select") {
        const opts = typeof c.options === "function" ? c.options(row) : c.options
        const o    = (opts || []).find((x) => x.value === v)
        return o ? o.text : v
      }
      if (c.type === "ref" && typeof c.ref === "string" && v > 0) {
        const name = (this.refNames[c.ref] || {})[v]
        return name ? v + " - " + name : v
      }
      if (typeof c.display === "function") {
        return c.display(v, row)
      }
      if (this.def.tree && c.name === this.def.tree.label) {
        return "\u00a0\u00a0\u00a0\u00a0".repeat(this.treeDepth[this.keyOf(row)] || 0) + v
      }
      return v === null ? "" : v
    },
    filteredRows() {
      const rows   = this.def.tree ? this.treeRows() : this.rows
      const needle = this.search.trim().toLowerCase()
      if (!needle) {
        return rows
      }
      return rows.filter((row) => this.columns().some((c) => String(this.display(row, c)).toLowerCase().includes(needle)))
    },

    // def.tree = { parent: "parent_id", label: "name" }: children listed under
    // their parent, the label column indented by depth
    treeRows() {
      const parent   = this.def.tree.parent
      const byParent = {}
      for (const row of this.rows) {
        (byParent[row[parent]] = byParent[row[parent]] || []).push(row)
      }

      const keys  = new Set(this.rows.map((r) => this.keyOf(r)))
      const out   = []
      const seen  = new Set()
      const visit = (row, depth) => {
        if (seen.has(this.keyOf(row))) {
          return
        }
        seen.add(this.keyOf(row))
        this.treeDepth[this.keyOf(row)] = depth
        out.push(row)
        for (const child of (byParent[this.keyOf(row)] || [])) {
          visit(child, depth + 1)
        }
      }

      // roots: no parent, or a parent that is not in the table
      for (const row of this.rows) {
        if (!row[parent] || !keys.has(row[parent])) {
          visit(row, 0)
        }
      }
      // anything left is in a parent loop; list it rather than hide it
      for (const row of this.rows) {
        visit(row, 0)
      }

      return out
    },

    async load() {
      if (this.waiting) {
        this.rows = []
        return
      }

      this.loading = true
      try {
        const q = (new SpireQueryBuilder()).limit(10000)
        for (const [k, v] of Object.entries(this.filter)) {
          q.where(k, "=", v)
        }
        if (this.def.orderBy) {
          q.orderBy(this.def.orderBy)
        }
        const r   = await SpireApi.v1().get(this.def.list, { params: q.get() })
        this.rows = r.data || []
        this.loadRefNames()
      } catch (e) {
        this.showError(e)
      }
      this.loading = false
    },

    // names for ref columns shown in the list
    async loadRefNames() {
      const names = {}
      for (const c of this.columns()) {
        if (c.type === "ref" && typeof c.ref === "string") {
          names[c.ref] = await StarfallRefs.names(c.ref, this.rows.map((r) => r[c.name]))
        }
      }
      this.refNames = names
    },

    select(row) {
      this.selectedKey = this.keyOf(row)
      this.original    = row
      this.editing     = JSON.parse(JSON.stringify(row))
      this.isNew       = false
      this.$emit("select", row)
    },

    async startNew() {
      const row = {}
      for (const f of this.def.fields) {
        if (typeof f.default !== "undefined") {
          row[f.name] = typeof f.default === "function" ? f.default() : f.default
        } else if (f.type === "text" || f.type === "textarea") {
          row[f.name] = f.nullable ? null : ""
        } else if (f.type === "select" && Array.isArray(f.options) && f.options.length > 0) {
          row[f.name] = f.options[0].value
        } else {
          row[f.name] = 0
        }
      }
      Object.assign(row, this.filter)

      // tables without AUTO_INCREMENT need an id chosen up front, unless the
      // filter fixes it (a 1:1 row keyed by its parent) or it is a real id
      // the user picks (def.chooseKey: a class, an NPC, a faction)
      if (!this.def.autoIncrement && !this.def.chooseKey && typeof this.filter[this.def.key] === "undefined") {
        row[this.def.key] = await this.nextKey()
      }

      // a new row lands at the end of its list
      if (this.def.fields.find((f) => f.name === "sort_order")) {
        row.sort_order = this.rows.reduce((max, r) => Math.max(max, r.sort_order || 0), 0) + 10
      }

      this.original = null
      this.editing  = row
      this.isNew    = true
    },

    async nextKey() {
      try {
        const r = await SpireApi.v1().get(this.def.list, {
          params: (new SpireQueryBuilder())
            .orderBy([this.def.key])
            .orderDirection("desc")
            .select([this.def.key])
            .limit(1)
            .get()
        })
        return r.data && r.data.length > 0 ? r.data[0][this.def.key] + 1 : 1
      } catch (e) {
        return 1
      }
    },

    // starts from the whole row: the API diffs the body against the stored
    // row, so a column missing from the body (the key, or one the form does
    // not show) would be written as zero
    payload() {
      const out = JSON.parse(JSON.stringify(this.editing))
      for (const f of this.def.fields) {
        let v = this.editing[f.name]
        if (f.type === "int" || f.type === "float" || f.type === "ref" || f.type === "bool") {
          v = Number(v) || 0
        }
        if (f.nullable && v === "") {
          v = null
        }
        out[f.name] = v
      }
      if (this.def.autoIncrement && this.isNew) {
        delete out[this.def.key]
      }
      return out
    },

    validate() {
      if (this.def.chooseKey && !(Number(this.editing[this.def.key]) > 0)) {
        const f = this.def.fields.find((x) => x.name === this.def.key)
        return (f ? f.label : this.def.key) + " is required"
      }
      if (this.isNew && this.def.chooseKey && this.rows.find((r) => this.keyOf(r) === Number(this.editing[this.def.key]))) {
        return "There is already a row for " + this.editing[this.def.key]
      }
      for (const f of this.def.fields) {
        const v = this.editing[f.name]
        if (f.required && (v === "" || v === null || v === 0)) {
          return f.label + " is required"
        }
        if (f.max && v && String(v).length > f.max) {
          return f.label + " is longer than " + f.max + " characters"
        }
      }
      return typeof this.def.validate === "function" ? this.def.validate(this.editing, this.rows, this.isNew) : ""
    },

    async save() {
      this.error        = ""
      this.notification = ""
      const invalid     = this.validate()
      if (invalid) {
        this.error = invalid
        return
      }

      try {
        let saved
        if (this.isNew) {
          const r = await SpireApi.v1().put(this.def.entity, this.payload())
          saved   = r.data
        } else {
          const r = await SpireApi.v1().patch(this.def.entity + "/" + this.keyOf(this.original), this.payload())
          saved   = r.data
        }

        this.notification = this.isNew ? "Created" : "Saved"
        this.forgetNames()
        await this.load()

        const row = this.rows.find((x) => this.keyOf(x) === this.keyOf(saved))
        if (row) {
          this.select(row)
        }
        this.$emit("changed")
      } catch (e) {
        this.showError(e)
      }
    },

    async remove() {
      const name = this.def.nameField ? this.original[this.def.nameField] : ""
      const what = (this.def.singular || "row") + " " + this.keyOf(this.original) + (name ? " (" + name + ")" : "")
      const warn = this.def.deleteWarning ? "\n\n" + this.def.deleteWarning : ""
      if (!confirm("Delete " + what + "?" + warn)) {
        return
      }

      try {
        await SpireApi.v1().delete(this.def.entity + "/" + this.keyOf(this.original))
        this.notification = "Deleted " + what
        this.forgetNames()
        this.selectedKey  = null
        this.cancel()
        await this.load()
        this.$emit("deleted")
        this.$emit("changed")
      } catch (e) {
        this.showError(e)
      }
    },

    // def.refType: the ref other fields use to name rows of this table
    forgetNames() {
      if (this.def.refType) {
        StarfallRefs.forget(this.def.refType)
      }
    },

    cancel() {
      this.editing  = null
      this.original = null
      this.isNew    = false
    },

    showError(e) {
      this.error = e && e.response && e.response.data && e.response.data.error ? e.response.data.error : String(e)
    },
  },
  created() {
    // depth of each row in a tree list, filled while listing; not reactive
    this.treeDepth = {}
  },
  mounted() {
    this.load()
  },
}
</script>

<style scoped>
.starfall-list {
  overflow-y: auto;
}

.starfall-form {
  border-top: 1px solid rgba(255, 255, 255, .2);
  padding-top: 10px;
}
</style>
