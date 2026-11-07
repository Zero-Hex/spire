<template>
  <div>
    <eq-window-simple title="Zone Loot">
      <div>
        What a zone's NPCs drop, worked out the way the server rolls loot: each NPC's loot table, its drop groups and
        their items, as the expected number of each item per kill. Snapshots keep every spawn, NPC and loot row the zone
        is made of, so you can put them back after editing.
      </div>
      <div class="mt-3 d-flex align-items-center flex-wrap">
        <span class="mr-2">Zone</span>
        <b-form-input size="sm" v-model="zoneInput" placeholder="zone short name" style="max-width: 170px" class="mr-2"
                      @keyup.enter="load()"/>
        <b-button size="sm" variant="outline-warning" class="mr-3" @click="load()" :disabled="!zoneInput.trim() || loading">
          <i :class="'fa ' + (loading ? 'fa-cog fa-spin' : 'fa-search') + ' mr-1'"></i> Load
        </b-button>
        <span v-if="data" class="text-muted">
          <b>{{ data.long_name || data.zone }}</b>:
          {{ data.npcs.length }} NPCs, {{ count(data.tables) }} loot tables, {{ count(data.dropgroups) }} drop groups,
          {{ count(data.items) }} items
          <span v-if="data.global_multiplier !== 1">(Zone:GlobalLootMultiplier {{ data.global_multiplier }})</span>
        </span>
      </div>
      <info-error-banner class="mt-2" :notification="notification" :error="error"
                         @dismiss-error="error = ''" @dismiss-notification="notification = ''"/>

      <div v-if="data && (sharedTables().length || sharedGroups().length)" class="mt-2 text-warning hint zone-loot-shared">
        <div v-if="sharedTables().length">
          <i class="fa fa-exclamation-triangle mr-1"></i>
          {{ sharedTables().length }} loot table(s) are also used by NPCs outside {{ data.zone }}: changing them changes those NPCs too.
          <span v-for="t in sharedTables()" :key="'st' + t.id" class="mr-2">
            <code>{{ t.id }} {{ t.name }}</code>
            ({{ outsideNames(t.shared_by) }})
          </span>
        </div>
        <div v-if="sharedGroups().length">
          <i class="fa fa-exclamation-triangle mr-1"></i>
          {{ sharedGroups().length }} drop group(s) are also used by loot tables outside {{ data.zone }}:
          <span v-for="d in sharedGroups()" :key="'sd' + d.id" class="mr-2">
            <code>{{ d.id }} {{ d.name }}</code>
            ({{ outsideNames(d.shared_by) }})
          </span>
        </div>
      </div>
    </eq-window-simple>

    <eq-window-simple class="mt-3" v-if="data">
      <eq-tabs :selected="tab" @on-selected="tab = $event">
        <!-- by NPC -->
        <eq-tab name="By NPC" :selected="tab === 'By NPC'">
          <div class="d-flex align-items-center flex-wrap mb-2">
            <b-form-input size="sm" v-model="npcFilter" placeholder="Search NPCs or items" style="max-width: 260px"
                          class="mr-3"/>
            <b-form-radio-group size="sm" v-model="role" :options="roleOptions()" buttons button-variant="outline-secondary"
                                class="mr-3"/>
            <b-form-checkbox v-model="lootOnly" class="mr-3">Only NPCs with loot</b-form-checkbox>
            <span class="text-muted">{{ npcs().length }} shown</span>
          </div>
          <table class="eq-table eq-highlight-rows zone-loot-npcs">
            <thead>
            <tr>
              <th>NPC</th>
              <th>Role</th>
              <th class="text-center">Level</th>
              <th class="text-center" title="How many are up at once, on average, over its spawn points">Up</th>
              <th>Loot table</th>
              <th>Drops per kill</th>
            </tr>
            </thead>
            <tbody>
            <template v-for="n in npcs()">
              <tr :key="'n' + n.id" @click="toggle(n.id)" class="zone-loot-clickable">
                <td>
                  <i :class="'fa fa-caret-' + (open[n.id] ? 'down' : 'right') + ' mr-1 text-muted'"></i>
                  <router-link :to="npcRoute(n)" @click.native.stop>{{ cleanName(n.name) }}</router-link>
                  <span class="text-muted ml-1">{{ n.id }}</span>
                </td>
                <td><span :class="'zone-loot-role zone-loot-role-' + n.role">{{ n.role }}</span></td>
                <td class="text-center">{{ levels(n) }}</td>
                <td class="text-center">{{ round(n.up, 2) }}</td>
                <td>
                  <span v-if="!n.loottable_id" class="text-muted">none</span>
                  <span v-else>
                    {{ n.loottable_id }}
                    <span v-if="isShared(table(n))" class="text-warning ml-1"
                          :title="'Also used by ' + outsideNames(table(n).shared_by)"><i class="fa fa-share-alt"></i> shared</span>
                  </span>
                </td>
                <td>
                  <span v-for="d in n.drops.slice(0, 4)" :key="n.id + '-' + d.item_id" class="mr-3 zone-loot-drop">
                    <span :class="'item-' + icon(d.item_id) + '-sm'" class="zone-loot-icon"></span>
                    {{ itemName(d.item_id) }} <span class="text-muted">{{ rate(d.expected) }}</span>
                  </span>
                  <span v-if="n.drops.length > 4" class="text-muted">+{{ n.drops.length - 4 }} more</span>
                </td>
              </tr>
              <tr v-if="open[n.id]" :key="'d' + n.id" class="zone-loot-detail">
                <td colspan="6">
                  <div class="text-muted hint mb-1">
                    Spawns in
                    <span v-for="s in n.spawns" :key="'sp' + s.spawngroup_id" class="mr-2">
                      group {{ s.spawngroup_id }} <code v-if="s.name" class="ml-1">{{ s.name }}</code>
                      ({{ s.points }} point(s), {{ percent(s.chance) }} of the group)
                    </span>
                  </div>
                  <div v-if="table(n)">
                    <div class="mb-1">
                      Loot table <b>{{ table(n).id }}</b> <code class="ml-1">{{ table(n).name }}</code>
                      <span v-if="table(n).maxcash" class="text-muted ml-2">cash {{ table(n).mincash }}–{{ table(n).maxcash }} copper</span>
                      <span v-if="isShared(table(n))" class="text-warning ml-2">
                        also used by {{ outsideNames(table(n).shared_by) }}
                      </span>
                    </div>
                    <div v-for="e in table(n).entries" :key="'e' + e.lootdrop_id" class="zone-loot-group">
                      <div>
                        Drop group <b>{{ e.lootdrop_id }}</b> <code class="ml-1">{{ groupName(e.lootdrop_id) }}</code>
                        <span class="text-muted ml-2">{{ entryRolls(e) }}</span>
                        <span v-if="isShared(group(e.lootdrop_id))" class="text-warning ml-2">
                          also in {{ outsideNames(group(e.lootdrop_id).shared_by) }}
                        </span>
                      </div>
                      <table class="zone-loot-items">
                        <tr v-for="it in groupItems(e.lootdrop_id)" :key="e.lootdrop_id + '-' + it.item_id"
                            :class="levelOK(it, n) ? '' : 'text-muted'">
                          <td style="width: 26px"><span :class="'item-' + icon(it.item_id) + '-sm'" class="zone-loot-icon"></span></td>
                          <td>
                            <router-link :to="itemRoute(it.item_id)">{{ itemName(it.item_id) }}</router-link>
                            <span class="text-muted ml-1">{{ it.item_id }}</span>
                          </td>
                          <td class="text-right pr-3">{{ round(it.chance, 2) }}%</td>
                          <td class="text-muted">
                            <span v-if="it.multiplier > 1">×{{ it.multiplier }}</span>
                            <span v-if="!levelOK(it, n)">only for NPC levels {{ it.npc_min_level }}–{{ it.npc_max_level || "any" }}</span>
                          </td>
                          <td class="text-right">{{ rate(expectedFor(n, it.item_id)) }}</td>
                        </tr>
                      </table>
                    </div>
                  </div>
                </td>
              </tr>
            </template>
            </tbody>
          </table>
        </eq-tab>

        <!-- by item -->
        <eq-tab name="By Item" :selected="tab === 'By Item'">
          <div class="d-flex align-items-center flex-wrap mb-2">
            <b-form-input size="sm" v-model="itemFilter" placeholder="Search items or NPCs" style="max-width: 260px"
                          class="mr-3"/>
            <span class="text-muted">
              {{ items().length }} items. <i>Per clear</i> is how many drop if everything up in the zone is killed once.
            </span>
          </div>
          <table class="eq-table eq-highlight-rows zone-loot-by-item">
            <thead>
            <tr>
              <th>Item</th>
              <th class="text-center">Req / rec</th>
              <th class="text-center">Flags</th>
              <th>Dropped by</th>
              <th class="text-right">Best per kill</th>
              <th class="text-right">Per clear</th>
            </tr>
            </thead>
            <tbody>
            <template v-for="it in items()">
              <tr :key="'i' + it.id" @click="toggleItem(it.id)" class="zone-loot-clickable">
                <td>
                  <i :class="'fa fa-caret-' + (openItem[it.id] ? 'down' : 'right') + ' mr-1 text-muted'"></i>
                  <span :class="'item-' + it.icon + '-sm'" class="zone-loot-icon"></span>
                  <router-link :to="itemRoute(it.id)" @click.native.stop>{{ it.name }}</router-link>
                  <span class="text-muted ml-1">{{ it.id }}</span>
                </td>
                <td class="text-center">{{ it.reqlevel || "–" }} / {{ it.reclevel || "–" }}</td>
                <td class="text-center">
                  <span v-if="it.magic" class="mr-1" title="Magic">M</span>
                  <span v-if="it.nodrop" class="mr-1 text-warning" title="No Drop">ND</span>
                  <span v-if="it.lore" title="Lore">L</span>
                </td>
                <td>
                  {{ it.droppers.length }} NPC(s):
                  <span class="text-muted">{{ it.droppers.slice(0, 3).map(d => cleanName(d.npc.name)).join(", ") }}<span
                    v-if="it.droppers.length > 3">, …</span></span>
                </td>
                <td class="text-right">{{ rate(it.best) }}</td>
                <td class="text-right">{{ round(it.perClear, 2) }}</td>
              </tr>
              <tr v-if="openItem[it.id]" :key="'id' + it.id" class="zone-loot-detail">
                <td colspan="6">
                  <table class="zone-loot-items">
                    <tr v-for="d in it.droppers" :key="it.id + '-' + d.npc.id">
                      <td><router-link :to="npcRoute(d.npc)">{{ cleanName(d.npc.name) }}</router-link>
                        <span class="text-muted ml-1">{{ d.npc.id }}</span></td>
                      <td><span :class="'zone-loot-role zone-loot-role-' + d.npc.role">{{ d.npc.role }}</span></td>
                      <td class="text-muted pl-3">level {{ levels(d.npc) }}, {{ round(d.npc.up, 2) }} up</td>
                      <td class="text-right pl-3">{{ rate(d.expected) }} per kill</td>
                    </tr>
                  </table>
                </td>
              </tr>
            </template>
            </tbody>
          </table>
        </eq-tab>

        <!-- snapshots -->
        <eq-tab name="Snapshots" :selected="tab === 'Snapshots'">
          <div class="mb-2">
            A snapshot keeps {{ data.zone }}'s spawn points, spawn groups and entries, its NPCs, their loot tables and
            drop groups with their items. Restoring puts those rows back as they were and removes spawn points and
            loot rows added under them since; it snapshots the zone first, so a restore can be undone too.
            Restart the zone (or the server) after restoring for it to take effect.
          </div>
          <div v-if="sharedTables().length || sharedGroups().length" class="text-warning hint mb-2">
            Shared loot tables and drop groups are in the snapshot too: restoring one puts it back for the NPCs outside
            {{ data.zone }} that use it as well.
          </div>
          <div class="d-flex align-items-center mb-3">
            <b-form-input size="sm" v-model="note" placeholder="Note (what you are about to change)" style="max-width: 360px"
                          class="mr-2" @keyup.enter="take()"/>
            <b-button size="sm" variant="outline-warning" @click="take()" :disabled="busy">
              <i class="fa fa-camera mr-1"></i> Take snapshot
            </b-button>
          </div>
          <div v-if="!snapshots.length" class="text-muted">No snapshots of {{ data.zone }} yet.</div>
          <table v-else class="eq-table eq-highlight-rows zone-loot-snapshots">
            <thead>
            <tr>
              <th>Taken</th>
              <th>Note</th>
              <th>Rows</th>
              <th></th>
            </tr>
            </thead>
            <tbody>
            <template v-for="s in snapshots">
              <tr :key="'s' + s.id">
                <td style="white-space: nowrap">{{ when(s.taken) }}</td>
                <td>{{ s.note }}</td>
                <td class="text-muted hint">{{ rowCounts(s) }}</td>
                <td class="text-right" style="white-space: nowrap">
                  <b-button size="sm" variant="outline-secondary" class="mr-1" @click="preview(s)" :disabled="busy">
                    {{ diffs[s.id] ? "Hide changes" : "What would change" }}
                  </b-button>
                  <b-button size="sm" variant="outline-danger" class="mr-1" @click="restore(s)" :disabled="busy">
                    <i class="fa fa-undo mr-1"></i> Restore
                  </b-button>
                  <b-button size="sm" variant="outline-secondary" @click="remove(s)" :disabled="busy" title="Delete snapshot">
                    <i class="fa fa-trash"></i>
                  </b-button>
                </td>
              </tr>
              <tr v-if="diffs[s.id]" :key="'sd' + s.id" class="zone-loot-detail">
                <td colspan="4">
                  <div v-if="!diffTotal(diffs[s.id])" class="text-success">Nothing has changed since this snapshot.</div>
                  <div v-for="t in diffTables(diffs[s.id])" :key="s.id + t.table" class="zone-loot-diff">
                    <b>{{ t.table }}</b> <span class="text-muted hint">({{ keyNames[t.table] }})</span>:
                    <span v-if="t.diff.changed.length" class="mr-2">{{ t.diff.changed.length }} put back</span>
                    <span v-if="t.diff.added.length" class="mr-2 text-success">{{ t.diff.added.length }} written back</span>
                    <span v-if="t.diff.removed.length" class="mr-2 text-danger">{{ t.diff.removed.length }} deleted</span>
                    <div class="text-muted hint">
                      <span v-for="c in t.diff.changed.slice(0, 8)" :key="'c' + c.key" class="mr-3">
                        {{ c.key }} <i>({{ c.columns.join(", ") }})</i>
                      </span>
                      <span v-for="c in t.diff.added.slice(0, 8)" :key="'a' + c.key" class="mr-3 text-success">+{{ c.key }}</span>
                      <span v-for="c in t.diff.removed.slice(0, 8)" :key="'r' + c.key" class="mr-3 text-danger">−{{ c.key }}</span>
                      <span v-if="t.total > 24">…</span>
                    </div>
                  </div>
                </td>
              </tr>
            </template>
            </tbody>
          </table>
        </eq-tab>
      </eq-tabs>
    </eq-window-simple>
  </div>
</template>

<script>
import EqWindowSimple  from "@/components/eq-ui/EQWindowSimple";
import EqTabs          from "@/components/eq-ui/EQTabs";
import EqTab           from "@/components/eq-ui/EQTab";
import InfoErrorBanner from "@/components/InfoErrorBanner";
import {SpireApi}      from "@/app/api/spire-api";
import {ROUTE}         from "@/routes";
import util            from "util";

const ZONE_KEY = "zone-loot:zone"

// what a changed row's key is made of, by table
const KEY_NAMES = {
  spawn2: "spawn point",
  spawngroup: "spawn group",
  spawnentry: "spawn group / NPC",
  npc_types: "NPC",
  loottable: "loot table",
  loottable_entries: "loot table / drop group",
  lootdrop: "drop group",
  lootdrop_entries: "drop group / item",
}

function stored(key, fallback) {
  try {
    return localStorage.getItem(key) || fallback
  } catch (e) {
    return fallback
  }
}

export default {
  name: "ZoneLoot",
  components: { EqWindowSimple, EqTabs, EqTab, InfoErrorBanner },
  data() {
    return {
      zoneInput: this.$route.query.zone || stored(ZONE_KEY, "warrens"),
      data: null,
      loading: false,
      busy: false,
      error: "",
      notification: "",
      tab: "By NPC",

      npcFilter: "",
      role: "",
      lootOnly: true,
      open: {},

      itemFilter: "",
      openItem: {},

      snapshots: [],
      note: "",
      diffs: {},
      keyNames: KEY_NAMES,
    }
  },
  mounted() {
    this.load()
  },
  methods: {
    async load() {
      const zone = this.zoneInput.trim().toLowerCase()
      if (!zone) {
        return
      }
      this.loading = true
      this.error   = ""
      try {
        const r   = await SpireApi.v1().get(`zone-loot/${zone}`)
        this.data = r.data
        this.open = {}
        this.openItem = {}
        this.diffs = {}
        try {
          localStorage.setItem(ZONE_KEY, zone)
        } catch (e) {
        }
        if (this.$route.query.zone !== zone) {
          this.$router.replace({ query: { zone: zone } }).catch(() => {})
        }
        if (!this.data.npcs.length) {
          this.notification = `${zone} has no spawned NPCs`
        }
        await this.loadSnapshots()
      } catch (e) {
        this.error = this.message(e)
      } finally {
        this.loading = false
      }
    },
    async loadSnapshots() {
      const r        = await SpireApi.v1().get(`zone-loot/${this.data.zone}/snapshots`)
      this.snapshots = r.data
    },
    message(e) {
      return (e.response && e.response.data && e.response.data.error) || e.message || String(e)
    },

    count(m) {
      return Object.keys(m || {}).length
    },
    roleOptions() {
      const by = { raid: 0, named: 0, trash: 0 }
      this.data.npcs.forEach(n => by[n.role]++)
      return [
        { text: `All ${this.data.npcs.length}`, value: "" },
        { text: `Raid ${by.raid}`, value: "raid" },
        { text: `Named ${by.named}`, value: "named" },
        { text: `Trash ${by.trash}`, value: "trash" },
      ]
    },
    npcs() {
      const q = this.npcFilter.trim().toLowerCase()
      const order = { raid: 0, named: 1, trash: 2 }
      return this.data.npcs
        .filter(n => !this.role || n.role === this.role)
        .filter(n => !this.lootOnly || n.drops.length)
        .filter(n => !q || n.name.toLowerCase().replace(/_/g, " ").includes(q) || String(n.id) === q ||
          n.drops.some(d => this.itemName(d.item_id).toLowerCase().includes(q)))
        .sort((a, b) => order[a.role] - order[b.role] || b.level - a.level || a.name.localeCompare(b.name))
    },
    items() {
      const by = {}
      for (const n of this.data.npcs) {
        for (const d of n.drops) {
          const it = this.data.items[d.item_id] || { id: d.item_id, name: `item ${d.item_id}`, icon: 0 }
          const e  = by[d.item_id] || (by[d.item_id] = { ...it, droppers: [], best: 0, perClear: 0 })
          e.droppers.push({ npc: n, expected: d.expected })
          e.best = Math.max(e.best, d.expected)
          e.perClear += d.expected * n.up
        }
      }
      const q = this.itemFilter.trim().toLowerCase()
      return Object.values(by)
        .filter(it => !q || it.name.toLowerCase().includes(q) || String(it.id) === q ||
          it.droppers.some(d => d.npc.name.toLowerCase().replace(/_/g, " ").includes(q)))
        .map(it => (it.droppers.sort((a, b) => b.expected - a.expected), it))
        .sort((a, b) => (b.reqlevel || b.reclevel) - (a.reqlevel || a.reclevel) || b.best - a.best || a.name.localeCompare(b.name))
    },

    table(n) {
      return this.data.tables[n.loottable_id]
    },
    group(id) {
      return this.data.dropgroups[id]
    },
    groupName(id) {
      return (this.group(id) || {}).name || ""
    },
    groupItems(id) {
      return ((this.group(id) || {}).entries || []).slice().sort((a, b) => b.chance - a.chance)
    },
    isShared(x) {
      return x && x.shared_by && x.shared_by.length > 0
    },
    sharedTables() {
      return Object.values(this.data.tables).filter(this.isShared)
    },
    sharedGroups() {
      return Object.values(this.data.dropgroups).filter(this.isShared)
    },
    outsideNames(list) {
      const names = list.slice(0, 4).map(o => this.cleanName(o.name) + (o.zone ? ` in ${o.zone}` : ""))
      return names.join(", ") + (list.length > 4 ? ` and ${list.length - 4} more` : "")
    },
    entryRolls(e) {
      const parts = []
      if (e.multiplier !== 1) {
        parts.push(`rolled ${e.multiplier}×`)
      }
      if (e.probability < 100) {
        parts.push(`${this.round(e.probability, 2)}% to roll`)
      }
      if (e.droplimit || e.mindrop) {
        parts.push(e.mindrop ? `${e.mindrop}–${Math.max(e.droplimit, e.mindrop)} items` : `up to ${e.droplimit} item(s)`)
      } else {
        parts.push("every item rolls")
      }
      return parts.join(", ")
    },
    levelOK(it, n) {
      return (!it.npc_min_level || n.level >= it.npc_min_level) && (!it.npc_max_level || n.level <= it.npc_max_level)
    },
    expectedFor(n, itemId) {
      const d = n.drops.find(d => d.item_id === itemId)
      return d ? d.expected : 0
    },

    item(id) {
      return this.data.items[id]
    },
    itemName(id) {
      return (this.item(id) || {}).name || `item ${id}`
    },
    icon(id) {
      return (this.item(id) || {}).icon || 0
    },
    cleanName(name) {
      return (name || "").replace(/^#/, "").replace(/_/g, " ")
    },
    levels(n) {
      return n.maxlevel > n.level ? `${n.level}–${n.maxlevel}` : n.level
    },
    round(v, places) {
      return Number(v.toFixed(places)).toString()
    },
    percent(v) {
      return this.round(v * 100, 1) + "%"
    },
    rate(v) {
      return v >= 1 ? `×${this.round(v, 2)}` : v >= 0.001 ? this.percent(v) : "<0.1%"
    },
    npcRoute(n) {
      return ROUTE.NPC_EDIT.replace(":npc", n.id)
    },
    itemRoute(id) {
      return util.format(ROUTE.ITEM_EDIT, id)
    },
    toggle(id) {
      this.$set(this.open, id, !this.open[id])
    },
    toggleItem(id) {
      this.$set(this.openItem, id, !this.openItem[id])
    },

    // snapshots
    when(unix) {
      return new Date(unix * 1000).toLocaleString()
    },
    rowCounts(s) {
      const c = s.counts || {}
      return `${c.spawn2 || 0} spawn points, ${c.npc_types || 0} NPCs, ${c.loottable || 0} loot tables, ` +
        `${c.lootdrop || 0} drop groups, ${c.lootdrop_entries || 0} drop items`
    },
    async take() {
      this.busy = true
      this.error = ""
      try {
        const r = await SpireApi.v1().post(`zone-loot/${this.data.zone}/snapshots`, { note: this.note })
        this.notification = `Snapshot ${r.data.id} taken`
        this.note = ""
        await this.loadSnapshots()
      } catch (e) {
        this.error = this.message(e)
      } finally {
        this.busy = false
      }
    },
    async preview(s) {
      if (this.diffs[s.id]) {
        this.$delete(this.diffs, s.id)
        return
      }
      this.busy = true
      this.error = ""
      try {
        const r = await SpireApi.v1().get(`zone-loot/${this.data.zone}/snapshots/${s.id}/diff`)
        this.$set(this.diffs, s.id, r.data)
      } catch (e) {
        this.error = this.message(e)
      } finally {
        this.busy = false
      }
    },
    diffTables(d) {
      return Object.keys(d).map(t => ({
        table: t, diff: d[t], total: d[t].changed.length + d[t].added.length + d[t].removed.length,
      })).filter(t => t.total > 0)
    },
    diffTotal(d) {
      return this.diffTables(d).reduce((a, t) => a + t.total, 0)
    },
    async restore(s) {
      if (!confirm(`Restore ${this.data.zone} to the snapshot from ${this.when(s.taken)}${s.note ? ` (${s.note})` : ""}?\n\n` +
        `The zone is snapshotted first, so this can be undone.`)) {
        return
      }
      this.busy = true
      this.error = ""
      try {
        const r = await SpireApi.v1().post(`zone-loot/${this.data.zone}/snapshots/${s.id}/restore`)
        const changed = this.diffTotal(r.data.diff)
        await this.load()
        this.notification = `Restored ${s.id}: ${changed} row(s) put back. How it was before is kept as snapshot ` +
          `${r.data.before.id}. Restart the zone for it to take effect.`
      } catch (e) {
        this.error = this.message(e)
      } finally {
        this.busy = false
      }
    },
    async remove(s) {
      if (!confirm(`Delete the snapshot from ${this.when(s.taken)}${s.note ? ` (${s.note})` : ""}?`)) {
        return
      }
      this.busy = true
      try {
        await SpireApi.v1().delete(`zone-loot/${this.data.zone}/snapshots/${s.id}`)
        this.$delete(this.diffs, s.id)
        await this.loadSnapshots()
      } catch (e) {
        this.error = this.message(e)
      } finally {
        this.busy = false
      }
    },
  },
}
</script>

<style scoped>
.hint {
  font-size: 0.85em;
}

.zone-loot-clickable {
  cursor: pointer;
}

.zone-loot-icon {
  display: inline-block;
  vertical-align: middle;
}

.zone-loot-drop {
  white-space: nowrap;
}

.zone-loot-role {
  font-size: 0.8em;
  padding: 1px 6px;
  border-radius: 3px;
  text-transform: uppercase;
}

.zone-loot-role-raid {
  background: rgba(220, 53, 69, 0.35);
}

.zone-loot-role-named {
  background: rgba(255, 193, 7, 0.3);
}

.zone-loot-role-trash {
  background: rgba(255, 255, 255, 0.08);
}

.zone-loot-detail > td {
  background: rgba(0, 0, 0, 0.25);
  padding: 8px 12px 10px 28px;
}

.zone-loot-group {
  margin-bottom: 8px;
}

.zone-loot-items td {
  padding: 1px 6px;
  border: none;
}

.zone-loot-diff {
  margin-bottom: 4px;
}
</style>
