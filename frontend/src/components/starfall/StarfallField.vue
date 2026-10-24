<template>
  <div class="starfall-field">
    <div class="font-weight-bold" style="font-size: 13px">
      {{ field.label }}
      <span class="text-muted font-weight-normal" v-if="field.name !== field.label">({{ field.name }})</span>
    </div>

    <!-- checkbox -->
    <eq-checkbox
      v-if="field.type === 'bool'"
      :id="'sf-' + uid + '-' + field.name"
      class="d-inline-block mt-1"
      :true-value="1"
      :false-value="0"
      :value="row[field.name]"
      @input="set($event)"
    />

    <!-- fixed choices -->
    <b-form-select
      v-else-if="field.type === 'select'"
      class="form-control form-control-sm mt-1"
      :value="row[field.name]"
      @change="set($event)"
    >
      <option v-for="o in options()" :key="String(o.value)" :value="o.value">{{ o.text }}</option>
    </b-form-select>

    <!-- long text -->
    <div v-else-if="field.type === 'textarea'">
      <b-form-textarea
        class="mt-1"
        rows="4"
        max-rows="16"
        :value="row[field.name] === null ? '' : row[field.name]"
        @input="set($event)"
      />
      <small v-if="field.max" :class="length() > field.max ? 'text-danger' : 'text-muted'">
        {{ length() }} / {{ field.max }}
      </small>
    </div>

    <!-- reference to another table or a constant list -->
    <div v-else-if="refType()">
      <div class="d-flex align-items-center mt-1">
        <b-form-input
          type="number"
          size="sm"
          style="max-width: 130px"
          :value="row[field.name]"
          @input="set(toNumber($event))"
          :readonly="readonly"
        />
        <a
          v-if="refName && refLink()"
          class="ml-2 text-truncate"
          :href="refLink()"
          target="_blank"
          :title="'Open in Spire'"
        >{{ refName }}</a>
        <span v-else-if="refName" class="ml-2 text-truncate">{{ refName }}</span>
        <span v-else-if="row[field.name] > 0" class="ml-2 text-warning">not found</span>
      </div>
      <b-form-input
        v-if="!readonly"
        size="sm"
        class="mt-1"
        :placeholder="'Search ' + refLabel() + ' by name or id'"
        v-model="search"
        @input="runSearch()"
      />
      <div v-if="results.length > 0" class="starfall-ref-results">
        <div
          v-for="r in results"
          :key="r.id"
          class="starfall-ref-result"
          @click="pick(r)"
        >{{ r.id }} - {{ r.name }}
        </div>
      </div>
    </div>

    <!-- numbers -->
    <b-form-input
      v-else-if="field.type === 'int' || field.type === 'float'"
      type="number"
      size="sm"
      class="mt-1"
      :step="field.type === 'float' ? 'any' : 1"
      :value="row[field.name]"
      @input="set(toNumber($event))"
      :readonly="readonly"
    />

    <!-- short text -->
    <div v-else>
      <b-form-input
        size="sm"
        class="mt-1"
        :value="row[field.name] === null ? '' : row[field.name]"
        @input="set($event)"
        :readonly="readonly"
      />
      <small v-if="field.max && length() > field.max * 0.8" :class="length() > field.max ? 'text-danger' : 'text-muted'">
        {{ length() }} / {{ field.max }}
      </small>
    </div>

    <small class="text-muted d-block mt-1" v-if="help()" v-html="help()"></small>
  </div>
</template>

<script>
import EqCheckbox     from "@/components/eq-ui/EQCheckbox";
import {StarfallRefs} from "@/app/starfall/starfall-refs";

let nextUid = 0

export default {
  name: "StarfallField",
  components: { EqCheckbox },
  props: {
    field: { type: Object, required: true },
    row: { type: Object, required: true },
    // key fields of an existing row are read only
    readonly: { type: Boolean, default: false },
  },
  data() {
    return {
      uid: nextUid++,
      refName: "",
      search: "",
      results: [],
      searchTimer: null,
    }
  },
  watch: {
    'row': {
      handler() {
        this.loadRefName()
      },
      deep: true,
      immediate: true,
    },
  },
  methods: {
    set(value) {
      this.$set(this.row, this.field.name, value)
      this.$emit("changed", this.field.name)
    },
    toNumber(v) {
      if (v === "" || v === null || typeof v === "undefined") {
        return 0
      }
      const n = Number(v)
      return isNaN(n) ? 0 : n
    },
    length() {
      const v = this.row[this.field.name]
      return v ? String(v).length : 0
    },
    options() {
      return typeof this.field.options === "function" ? this.field.options(this.row) : (this.field.options || [])
    },
    help() {
      return typeof this.field.help === "function" ? this.field.help(this.row) : (this.field.help || "")
    },

    // the reference type can depend on another field (objective_type picks
    // what target_id points at)
    refType() {
      if (this.field.type !== "ref") {
        return null
      }
      return typeof this.field.ref === "function" ? this.field.ref(this.row) : this.field.ref
    },
    refLabel() {
      const r = StarfallRefs.get(this.refType())
      return r ? r.label.toLowerCase() : ""
    },
    refLink() {
      const r  = StarfallRefs.get(this.refType())
      const id = this.row[this.field.name]
      return r && r.link && id > 0 ? r.link(id) : ""
    },
    async loadRefName() {
      const type = this.refType()
      const id   = this.row[this.field.name]
      this.refName = type && id > 0 ? await StarfallRefs.nameOf(type, id) : ""
    },
    runSearch() {
      clearTimeout(this.searchTimer)
      this.searchTimer = setTimeout(async () => {
        this.results = await StarfallRefs.search(this.refType(), this.search.trim())
      }, 300)
    },
    pick(r) {
      this.set(r.id)
      this.search  = ""
      this.results = []
    },
  },
}
</script>

<style scoped>
.starfall-ref-results {
  max-height: 200px;
  overflow-y: auto;
  border: 1px solid rgba(255, 255, 255, .2);
  border-radius: 4px;
  margin-top: 2px;
}

.starfall-ref-result {
  padding: 3px 8px;
  cursor: pointer;
  font-size: 13px;
}

.starfall-ref-result:hover {
  background: rgba(255, 255, 255, .1);
}
</style>
