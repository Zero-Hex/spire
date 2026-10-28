<template>
  <b-modal
    id="item-generator"
    ref="modal"
    size="xl"
    :title="title()"
    content-class="item-gen-modal"
    hide-footer
    @hidden="$emit('closed')"
  >
    <div v-if="archive || armorSet">
      <div class="d-flex align-items-center flex-wrap mb-2">
        <span class="gen-label">Make</span>
        <div class="btn-group btn-group-sm mr-3">
          <b-button v-for="k in kinds()" :key="k.value" size="sm"
                    :variant="kind === k.value ? 'warning' : 'outline-secondary'" @click="setKind(k.value)">
            {{ k.text }}
          </b-button>
        </div>
        <span class="mr-2">{{ kind === "weapon" ? "Name" : "Set name" }}</span>
        <b-form-input size="sm" v-model="name" style="max-width: 260px" :placeholder="baseName()"
                      @input="replanSoon()"/>
      </div>

      <!-- weapon: which models, and the icon -->
      <div v-if="kind === 'weapon'" class="d-flex align-items-center flex-wrap mb-2">
        <span class="gen-label">Models</span>
        <b-form-checkbox v-for="m in archive.models" :key="m" v-model="models" :value="m" class="mr-3"
                         @change="replanSoon()">
          IT{{ m }}
        </b-form-checkbox>
      </div>
      <div v-if="kind === 'weapon'" class="d-flex align-items-center mb-2">
        <span class="gen-label">Icon</span>
        <span v-if="icon" :class="'item-' + icon + ' mr-2'" :title="icon"/>
        <b-form-input size="sm" type="number" v-model.number="icon" style="max-width: 110px" class="mr-2"
                      placeholder="template's" @input="replanSoon()"/>
        <b-button size="sm" variant="outline-secondary" @click="pickIcon(null)">Choose...</b-button>
      </div>

      <!-- sets: the Heroes' Forge set; typed only when no tag row names it -->
      <div v-if="armorSet" class="d-flex align-items-center flex-wrap mb-2">
        <span class="gen-label">Heroes' Forge set</span>
        <b v-if="armorSet.set" class="mr-2">{{ armorSet.set }}</b>
        <b-form-input v-else size="sm" type="number" v-model.number="set" style="max-width: 110px" class="mr-2"
                      @input="replanSoon()"/>
        <span class="text-muted gen-hint">
          <span v-if="armorSet.set">as NewArmorTagData.txt names it;</span>
          <span v-else>no NewArmorTagData.txt row names these models yet: the number their rows will use;</span>
          {{ armorSet.models.length }} pieces in {{ armorSet.archives.map((a) => a.file).join(", ") }}
        </span>
      </div>

      <div class="d-flex align-items-center flex-wrap mb-2">
        <span class="gen-label">Copy items</span>
        <b-form-input size="sm" v-model="templates" style="max-width: 360px" class="mr-2"
                      :placeholder="usualTemplates()" @input="replanSoon()"/>
        <span class="mr-2">IDs from</span>
        <b-form-input size="sm" type="number" v-model.number="start" style="max-width: 110px" class="mr-2"
                      @input="replanSoon()"/>
        <span class="mr-2">up to</span>
        <b-form-input size="sm" type="number" v-model.number="end" style="max-width: 110px" placeholder="no limit"
                      @input="replanSoon()"/>
      </div>
      <div class="text-muted gen-hint mb-2">
        Item IDs to copy, comma separated; empty uses {{ usualTemplates() }}. Each new item is the copy with the
        name, icon and {{ kind === "weapon" ? "item model" : "Heroes' Forge model (set × 100 + slot)" }} changed: edit
        the rest in the item editor. New IDs follow the highest ID already used between the two; ornaments and other
        augments stay at or below 1,048,573, and 1,048,574 and 1,048,575 are never used (the server's item links
        reserve them).
      </div>

      <div v-for="(w, i) in (plan ? plan.warnings : [])" :key="'w' + i" class="text-warning">
        <i class="fa fa-exclamation-triangle mr-1"></i> {{ w }}
      </div>
      <div v-if="error" class="text-danger mt-1">{{ error }}</div>

      <table v-if="plan && plan.rows.length" class="eq-table eq-highlight-rows mt-2 gen-rows">
        <thead>
        <tr>
          <th>ID</th>
          <th>{{ kind === "weapon" ? "Model" : "Slot" }}</th>
          <th>Name</th>
          <th>Icon</th>
          <th>{{ kind === "weapon" ? "Item model" : "Heroes' Forge" }}</th>
          <th>Copy of</th>
        </tr>
        </thead>
        <tbody>
        <tr v-for="(r, i) in plan.rows" :key="i">
          <td>{{ plan.ids[i] }}</td>
          <td>{{ r.slot_label || r.idfile }}</td>
          <td><b-form-input size="sm" v-model="r.name" maxlength="64"/></td>
          <td class="text-nowrap">
            <span :class="'item-' + r.icon + ' mr-1'" :title="r.icon"/>
            <b-form-input size="sm" type="number" v-model.number="r.icon" style="width: 90px; display: inline-block"/>
            <b-button size="sm" variant="outline-secondary" class="ml-1" @click="pickIcon(r)">...</b-button>
          </td>
          <td>{{ kind === "weapon" ? r.idfile : r.herosforgemodel }}</td>
          <td>{{ r.template_id }} {{ r.template_name }}</td>
        </tr>
        </tbody>
      </table>

      <div class="mt-3 d-flex align-items-center">
        <b-button variant="success" class="mr-2" @click="create()" :disabled="!plan || !plan.rows.length || busy">
          <i :class="'fa ' + (busy ? 'fa-cog fa-spin' : 'fa-magic') + ' mr-1'"></i>
          Create {{ plan ? plan.rows.length : "" }} item{{ plan && plan.rows.length === 1 ? "" : "s" }}
        </b-button>
        <b-button variant="outline-secondary" class="mr-3" @click="replan()" :disabled="busy">Refresh</b-button>
        <span v-if="planning" class="text-muted"><i class="fa fa-cog fa-spin mr-1"></i> Working it out...</span>
      </div>

      <div v-if="created.length" class="mt-3">
        <div class="text-success mb-1">
          Created {{ created.length }} item{{ created.length === 1 ? "" : "s" }}. The server sees them once it reloads
          its items.
        </div>
        <div v-for="c in created" :key="c.id">
          <router-link :to="itemRoute(c.id)" target="_blank">{{ c.id }} {{ c.name }}</router-link>
        </div>
      </div>

      <!-- the item editor's icon picker -->
      <div v-if="picking" class="mt-3">
        <div class="d-flex align-items-center mb-1">
          <b class="mr-auto">Choose an icon{{ picking.row ? " for " + picking.row.name : "" }}</b>
          <b-button size="sm" variant="outline-secondary" @click="picking = null">Close</b-button>
        </div>
        <item-icon-selector :selected-icon="picking.row ? picking.row.icon : (icon || 0)"
                            :find-by-model="kind === 'weapon' && models.length ? 'IT' + models[0] : ''"
                            @input="iconPicked"/>
      </div>
    </div>
  </b-modal>
</template>

<script>
import {SpireApi}       from "@/app/api/spire-api";
import {ROUTE}          from "@/routes";
import ItemIconSelector from "@/views/items/components/ItemIconSelector";
import util             from "util";

const START_KEY = "spire-item-gen-start"
const END_KEY   = "spire-item-gen-end"

function stored(key, otherwise) {
  try {
    return parseInt(localStorage.getItem(key)) || otherwise
  } catch (e) {
    return otherwise
  }
}

export default {
  name: "StarfallItemGenerator",
  components: { ItemIconSelector },
  data() {
    return {
      archive: null,  // weapon: the archive with the models
      armorSet: null, // armor, ornament: the Heroes' Forge set
      kind: "weapon",
      name: "",
      models: [],
      icon: null,
      set: null,
      templates: "",
      start: stored(START_KEY, 300000),
      end: stored(END_KEY, 999999), // the custom block; empty: as far as IDs go
      plan: null,
      planning: false,
      busy: false,
      error: "",
      created: [],
      picking: null,
      timer: null,
    }
  },
  methods: {
    // open makes weapons from an archive's item models
    open(archive, kind) {
      this.reset(archive, null, kind)
      this.models = archive.models.length === 1 ? [archive.models[0]] : []
      this.$refs.modal.show()
      this.replan()
    },
    // openSet makes "armor" or "ornament" pieces of a Heroes' Forge set
    openSet(set, kind) {
      this.reset(null, set, kind)
      this.set = set.set || set.guess || null
      this.$refs.modal.show()
      this.replan()
    },
    reset(archive, set, kind) {
      this.archive   = archive
      this.armorSet  = set
      this.kind      = kind
      this.name      = ""
      this.models    = []
      this.icon      = null
      this.set       = null
      this.templates = ""
      this.plan      = null
      this.created   = []
      this.picking   = null
      this.error     = ""
    },
    title() {
      if (this.armorSet) {
        return "Create items for " + (this.armorSet.set ? "set " + this.armorSet.set : this.armorSet.texture || "untagged armor")
      }
      return this.archive ? "Create items from " + this.archive.file : "Create items"
    },
    kinds() {
      if (this.armorSet) {
        return [{ value: "armor", text: "Armor set" }, { value: "ornament", text: "Ornament set" }]
      }
      return [{ value: "weapon", text: "Weapon" }]
    },
    setKind(kind) {
      this.kind      = kind
      this.templates = ""
      this.replan()
    },
    baseName() {
      if (this.armorSet) {
        return "Set " + (this.set || "")
      }
      return this.archive ? this.archive.file.replace(/\.[^.]*$/, "") : ""
    },
    usualTemplates() {
      return {
        weapon: "Fine Steel Short Sword",
        armor: "the Cloth Cap, Shirt, Sleeves, Wristband, Gloves, Pants and Sandals",
        ornament: "the Old Hero's Forge set (67983-67989)",
      }[this.kind]
    },
    request() {
      return {
        kind: this.kind,
        archive: this.archive ? this.archive.file : "",
        models: this.kind === "weapon" ? this.models : [],
        set: this.kind === "weapon" ? 0 : (parseInt(this.set) || 0),
        templates: this.templates.split(/[\s,]+/).map((t) => parseInt(t)).filter((t) => t > 0),
        name: this.name,
        icon: parseInt(this.icon) || 0,
        start: parseInt(this.start) || 0,
        end: parseInt(this.end) || 0,
      }
    },
    replanSoon() {
      clearTimeout(this.timer)
      this.timer = setTimeout(() => this.replan(), 500)
    },
    async replan() {
      clearTimeout(this.timer)
      this.planning = true
      this.error    = ""
      try {
        const r   = await SpireApi.v1().post("item-gen/plan", this.request())
        this.plan = r.data
      } catch (e) {
        this.plan  = null
        this.error = this.message(e)
      }
      this.planning = false
    },
    message(e) {
      return e && e.response && e.response.data && e.response.data.error ? e.response.data.error : String(e)
    },
    pickIcon(row) {
      this.picking = { row: row }
    },
    iconPicked(icon) {
      if (this.picking.row) {
        this.picking.row.icon = icon
      } else {
        this.icon = icon
        this.replanSoon()
      }
      this.picking = null
    },
    async create() {
      this.busy  = true
      this.error = ""
      try {
        const r = await SpireApi.v1().post("item-gen/create", {
          kind: this.kind,
          archive: this.archive ? this.archive.file : "",
          set: this.plan.set,
          start: parseInt(this.start) || 0,
          end: parseInt(this.end) || 0,
          rows: this.plan.rows,
        })
        this.created = r.data.created
        try {
          localStorage.setItem(START_KEY, String(parseInt(this.start) || 300000))
          localStorage.setItem(END_KEY, String(parseInt(this.end) || ""))
        } catch (e) {
          // the start stays as typed
        }
        this.$emit("created", this.created)
        // made: Refresh plans more, numbered after these
        this.plan = null
      } catch (e) {
        this.error = this.message(e)
      }
      this.busy = false
    },
    itemRoute(id) {
      return util.format(ROUTE.ITEM_EDIT, id)
    },
  },
  beforeDestroy() {
    clearTimeout(this.timer)
  },
}
</script>

<style scoped>
.gen-label {
  min-width: 130px;
}

.gen-hint {
  font-size: 13px;
}

.gen-rows td {
  vertical-align: middle;
}
</style>

<style>
/* Spire's tables have light text: the dialog is dark in light mode too */
.item-gen-modal {
  background-color: #0b0d12 !important;
  color: #d3dae6;
  border: 1px solid rgba(255, 255, 255, .2);
}

.item-gen-modal .modal-header {
  border-bottom-color: rgba(255, 255, 255, .15);
}

.item-gen-modal .modal-title,
.item-gen-modal .close {
  color: #d3dae6;
}
</style>
