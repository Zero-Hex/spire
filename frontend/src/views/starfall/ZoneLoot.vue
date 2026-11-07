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
        <span class="mr-2">version</span>
        <b-form-select size="sm" v-model="version" :options="versionOptions()" style="max-width: 90px" class="mr-2"
                       @change="load()"/>
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
          {{ sharedTables().length }} loot table(s) are also used by NPCs outside {{ where() }}: changing them changes those NPCs too.
          <span v-for="t in sharedTables()" :key="'st' + t.id" class="mr-2">
            <code>{{ t.id }} {{ t.name }}</code>
            ({{ outsideNames(t.shared_by) }})
          </span>
        </div>
        <div v-if="sharedGroups().length">
          <i class="fa fa-exclamation-triangle mr-1"></i>
          {{ sharedGroups().length }} drop group(s) are also used by loot tables outside {{ where() }}:
          <span v-for="d in sharedGroups()" :key="'sd' + d.id" class="mr-2">
            <code>{{ d.id }} {{ d.name }}</code>
            ({{ outsideNames(d.shared_by) }})
          </span>
        </div>
      </div>
    </eq-window-simple>

    <eq-window-simple class="mt-3" v-if="data">
      <div v-if="editing || pendingCount()" class="zone-loot-pending">
        <div class="d-flex align-items-center flex-wrap">
          <span class="mr-3">
            <i v-if="previewing" class="fa fa-cog fa-spin mr-1"></i>
            <b>{{ pendingCount() }}</b> unsaved change(s)<span v-if="pendingCount()">: drops per kill below show them</span>
            <span v-else class="text-muted">: open an NPC and change its loot; nothing is saved until you save</span>
          </span>
          <a v-if="pendingCount()" href="#" class="mr-3" @click.prevent="showPending = !showPending">{{ showPending ? "Hide" : "Show" }} changes</a>
          <b-button size="sm" variant="success" class="mr-2" :disabled="!pendingCount() || saving" @click="save()">
            <i :class="'fa ' + (saving ? 'fa-cog fa-spin' : 'fa-save') + ' mr-1'"></i> Save
          </b-button>
          <b-button size="sm" variant="outline-secondary" :disabled="!pendingCount() || saving" @click="discard()">Discard</b-button>
          <span class="text-muted hint ml-3" v-if="!snapshotted">The zone is snapshotted before the first save.</span>
        </div>
        <ul v-if="showPending && pendingCount()" class="mt-2 mb-0 hint">
          <li v-for="(e, key) in pending" :key="'p' + key">
            {{ describe(e) }}
            <a href="#" class="ml-2" @click.prevent="undo(key)">undo</a>
          </li>
        </ul>
      </div>
      <eq-tabs :selected="tab" @on-selected="tab = $event">
        <!-- by NPC -->
        <eq-tab name="By NPC" :selected="tab === 'By NPC'">
          <div class="d-flex align-items-center flex-wrap mb-2">
            <b-form-input size="sm" v-model="npcFilter" placeholder="Search NPCs or items" style="max-width: 260px"
                          class="mr-3"/>
            <b-form-radio-group size="sm" v-model="role" :options="roleOptions()" buttons button-variant="outline-secondary"
                                class="mr-3"/>
            <b-form-checkbox v-model="lootOnly" class="mr-3">Only NPCs with loot</b-form-checkbox>
            <b-form-checkbox v-model="editing" switch class="mr-3">Edit loot</b-form-checkbox>
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

                  <!-- no loot table -->
                  <div v-if="!table(n)">
                    <span class="text-muted">No loot table.</span>
                    <template v-if="editing">
                      <b-button size="sm" variant="outline-warning" class="ml-2" :disabled="structuralBlocked()"
                                :title="structuralTitle()" @click="act({ op: 'new_loottable', npc_ids: [n.id] }, 'New loot table for ' + cleanName(n.name))">
                        <i class="fa fa-plus mr-1"></i> New loot table
                      </b-button>
                      <span class="ml-3 mr-1">or use loot table</span>
                      <b-form-input size="sm" type="number" class="zone-loot-num d-inline-block" placeholder="id"
                                    @change="v => useTable(n, v)"/>
                    </template>
                  </div>

                  <div v-else>
                    <!-- the loot table -->
                    <div class="mb-1 d-flex align-items-center flex-wrap">
                      <span class="mr-2">Loot table <b>{{ table(n).id }}</b> <code class="ml-1">{{ table(n).name }}</code></span>
                      <template v-if="!editing">
                        <span v-if="table(n).maxcash" class="text-muted mr-2">cash {{ table(n).mincash }}–{{ table(n).maxcash }} copper</span>
                      </template>
                      <template v-else>
                        <span class="mr-1">cash</span>
                        <b-form-input size="sm" type="number" min="0" class="zone-loot-num" :disabled="locked('t', table(n).id)"
                                      :value="tableValue(table(n), 'mincash')" @change="v => setTable(table(n), 'mincash', v)"/>
                        <span class="mx-1">–</span>
                        <b-form-input size="sm" type="number" min="0" class="zone-loot-num" :disabled="locked('t', table(n).id)"
                                      :value="tableValue(table(n), 'maxcash')" @change="v => setTable(table(n), 'maxcash', v)"/>
                        <span class="ml-1 mr-3">copper</span>
                      </template>
                      <span v-if="usersHere(table(n).id).length > 1" class="text-muted mr-2"
                            :title="usersHere(table(n).id).map(u => cleanName(u.name)).join(', ')">
                        used by {{ usersHere(table(n).id).length }} NPCs here
                      </span>
                      <span v-if="isShared(table(n))" class="text-warning mr-2">
                        also used by {{ outsideNames(table(n).shared_by) }}
                        <span v-if="everywhere['t' + table(n).id]">(editing everywhere)</span>
                      </span>
                    </div>
                    <div v-if="editing" class="zone-loot-actions mb-2">
                      <template v-if="locked('t', table(n).id)">
                        <b-button size="sm" variant="outline-warning" :disabled="structuralBlocked()" :title="structuralTitle()"
                                  @click="act({ op: 'copy_loottable', loottable_id: table(n).id, deep: true }, 'Copy loot table ' + table(n).id + ' for ' + where())">
                          <i class="fa fa-clone mr-1"></i> Copy for {{ where() }}
                        </b-button>
                        <b-button size="sm" variant="outline-danger" @click="unlock('t', table(n).id)">Edit everywhere</b-button>
                      </template>
                      <b-button v-if="usersHere(table(n).id).length > 1" size="sm" variant="outline-secondary"
                                :disabled="structuralBlocked()" :title="structuralTitle()"
                                @click="act({ op: 'copy_loottable', loottable_id: table(n).id, npc_ids: [n.id], deep: true }, 'Copy loot table ' + table(n).id + ' for ' + cleanName(n.name) + ' only')">
                        <i class="fa fa-clone mr-1"></i> Copy for this NPC only
                      </b-button>
                      <template v-if="!locked('t', table(n).id)">
                        <b-button size="sm" variant="outline-success" :disabled="structuralBlocked()" :title="structuralTitle()"
                                  @click="openMaker(n, table(n), 0)">
                          <i class="fa fa-magic mr-1"></i> Make items
                        </b-button>
                        <b-button size="sm" variant="outline-secondary" :disabled="structuralBlocked()" :title="structuralTitle()"
                                  @click="act({ op: 'new_lootdrop', loottable_id: table(n).id, everywhere: !!everywhere['t' + table(n).id] }, 'New drop group in loot table ' + table(n).id)">
                          <i class="fa fa-plus mr-1"></i> New drop group
                        </b-button>
                        <span class="ml-2 mr-1">add drop group</span>
                        <b-form-input size="sm" type="number" class="zone-loot-num d-inline-block" placeholder="id"
                                      @change="v => addGroup(table(n), v)"/>
                      </template>
                      <span class="ml-3 mr-1">use loot table</span>
                      <b-form-input size="sm" type="number" class="zone-loot-num d-inline-block" placeholder="id"
                                    @change="v => useTable(n, v)"/>
                      <b-button size="sm" variant="outline-secondary" class="ml-1" @click="useTable(n, 0)">No loot</b-button>
                    </div>

                    <!-- its drop groups -->
                    <div v-for="e in table(n).entries" :key="'e' + e.lootdrop_id" class="zone-loot-group">
                      <div class="d-flex align-items-center flex-wrap">
                        <span class="mr-2">Drop group <b>{{ e.lootdrop_id }}</b> <code class="ml-1">{{ groupName(e.lootdrop_id) }}</code></span>
                        <span v-if="!editing" class="text-muted mr-2">{{ entryRolls(e) }}</span>
                        <template v-else>
                          <span class="mr-1">rolled</span>
                          <b-form-input size="sm" type="number" min="0" max="255" class="zone-loot-num" :disabled="locked('t', table(n).id)"
                                        :value="entryValue(table(n), e, 'multiplier')" @change="v => setEntry(table(n), e, 'multiplier', v)"/>
                          <span class="mx-1">×,</span>
                          <b-form-input size="sm" type="number" min="0" max="100" step="0.1" class="zone-loot-num" :disabled="locked('t', table(n).id)"
                                        :value="entryValue(table(n), e, 'probability')" @change="v => setEntry(table(n), e, 'probability', v)"/>
                          <span class="mx-1">% to roll, picks: at least</span>
                          <b-form-input size="sm" type="number" min="0" max="255" class="zone-loot-num" :disabled="locked('t', table(n).id)"
                                        :value="entryValue(table(n), e, 'mindrop')" @change="v => setEntry(table(n), e, 'mindrop', v)"/>
                          <span class="mx-1">up to</span>
                          <b-form-input size="sm" type="number" min="0" max="255" class="zone-loot-num" :disabled="locked('t', table(n).id)"
                                        :value="entryValue(table(n), e, 'droplimit')" @change="v => setEntry(table(n), e, 'droplimit', v)"
                                        title="0 with no minimum: every item rolls on its own"/>
                          <b-button size="sm" variant="outline-danger" class="ml-2 mr-2" :disabled="locked('t', table(n).id)"
                                    title="Take this drop group out of the loot table" @click="removeEntry(table(n), e)">
                            <i class="fa fa-times"></i>
                          </b-button>
                        </template>
                        <span v-if="editing && tablesHere(e.lootdrop_id).length > 1" class="text-muted mr-2"
                              :title="tablesHere(e.lootdrop_id).map(t => t.id + ' ' + t.name).join(', ')">
                          in {{ tablesHere(e.lootdrop_id).length }} loot tables here
                        </span>
                        <span v-if="isShared(group(e.lootdrop_id))" class="text-warning mr-2">
                          also in {{ outsideNames(group(e.lootdrop_id).shared_by) }}
                          <span v-if="everywhere['d' + e.lootdrop_id]">(editing everywhere)</span>
                        </span>
                      </div>
                      <div v-if="editing" class="zone-loot-actions my-1">
                        <template v-if="locked('d', e.lootdrop_id)">
                          <b-button size="sm" variant="outline-warning" :disabled="structuralBlocked()" :title="structuralTitle()"
                                    @click="act({ op: 'copy_lootdrop', lootdrop_id: e.lootdrop_id }, 'Copy drop group ' + e.lootdrop_id + ' for ' + where())">
                            <i class="fa fa-clone mr-1"></i> Copy for {{ where() }}
                          </b-button>
                          <b-button size="sm" variant="outline-danger" @click="unlock('d', e.lootdrop_id)">Edit everywhere</b-button>
                        </template>
                        <b-button v-if="!locked('d', e.lootdrop_id)" size="sm" variant="outline-success"
                                  :disabled="structuralBlocked()" :title="structuralTitle()" @click="openMaker(n, table(n), e.lootdrop_id)">
                          <i class="fa fa-magic mr-1"></i> Make items for this group
                        </b-button>
                        <b-button v-if="tablesHere(e.lootdrop_id).length > 1 && !locked('t', table(n).id)" size="sm" variant="outline-secondary"
                                  :disabled="structuralBlocked()" :title="structuralTitle()"
                                  @click="act({ op: 'copy_lootdrop', lootdrop_id: e.lootdrop_id, loottable_ids: [table(n).id] }, 'Copy drop group ' + e.lootdrop_id + ' for loot table ' + table(n).id + ' only')">
                          <i class="fa fa-clone mr-1"></i> Copy for this loot table only
                        </b-button>
                      </div>
                      <table class="zone-loot-items">
                        <tr v-if="editing" class="text-muted hint">
                          <td></td><td></td><td class="text-right pr-2">chance %</td><td>×</td><td>NPC levels</td><td>equip</td><td></td>
                          <td class="text-right">per kill</td>
                        </tr>
                        <tr v-for="it in groupItems(e.lootdrop_id)" :key="e.lootdrop_id + '-' + it.item_id"
                            :class="levelOK(it, n) ? '' : 'text-muted'">
                          <td style="width: 26px"><span :class="'item-' + icon(it.item_id) + '-sm'" class="zone-loot-icon"></span></td>
                          <td>
                            <router-link :to="itemRoute(it.item_id)">{{ itemName(it.item_id) }}</router-link>
                            <span class="text-muted ml-1">{{ it.item_id }}</span>
                          </td>
                          <template v-if="!editing">
                            <td class="text-right pr-3">{{ round(it.chance, 2) }}%</td>
                            <td class="text-muted">
                              <span v-if="it.multiplier > 1">×{{ it.multiplier }}</span>
                              <span v-if="!levelOK(it, n)">only for NPC levels {{ it.npc_min_level }}–{{ it.npc_max_level || "any" }}</span>
                            </td>
                          </template>
                          <template v-else>
                            <td class="pr-2"><b-form-input size="sm" type="number" min="0" max="100" step="0.1" class="zone-loot-num"
                                                           :disabled="locked('d', e.lootdrop_id)" :value="itemValue(e.lootdrop_id, it, 'chance')"
                                                           @change="v => setItem(e.lootdrop_id, it, 'chance', v)"/></td>
                            <td><b-form-input size="sm" type="number" min="0" max="255" class="zone-loot-num zone-loot-num-sm"
                                              :disabled="locked('d', e.lootdrop_id)" :value="itemValue(e.lootdrop_id, it, 'multiplier')"
                                              @change="v => setItem(e.lootdrop_id, it, 'multiplier', v)"/></td>
                            <td style="white-space: nowrap">
                              <b-form-input size="sm" type="number" min="0" class="zone-loot-num zone-loot-num-sm d-inline-block"
                                            :disabled="locked('d', e.lootdrop_id)" :value="itemValue(e.lootdrop_id, it, 'npc_min_level')"
                                            @change="v => setItem(e.lootdrop_id, it, 'npc_min_level', v)" title="0: any"/>
                              –
                              <b-form-input size="sm" type="number" min="0" class="zone-loot-num zone-loot-num-sm d-inline-block"
                                            :disabled="locked('d', e.lootdrop_id)" :value="itemValue(e.lootdrop_id, it, 'npc_max_level')"
                                            @change="v => setItem(e.lootdrop_id, it, 'npc_max_level', v)" title="0: any"/>
                            </td>
                            <td class="text-center">
                              <b-form-checkbox :disabled="locked('d', e.lootdrop_id)" :checked="!!itemValue(e.lootdrop_id, it, 'equip_item')"
                                               @change="v => setItem(e.lootdrop_id, it, 'equip_item', v)" title="The NPC wears it"/>
                            </td>
                            <td>
                              <b-button size="sm" variant="outline-danger" :disabled="locked('d', e.lootdrop_id)" title="Remove the item"
                                        @click="removeItem(e.lootdrop_id, it)"><i class="fa fa-times"></i></b-button>
                            </td>
                          </template>
                          <td class="text-right pl-3" style="white-space: nowrap">
                            {{ rate(expectedFor(n, it.item_id)) }}
                            <span v-if="changedRate(n, it.item_id)" class="text-muted hint">({{ baseExpected(n, it.item_id) ? "was " + rate(baseExpected(n, it.item_id)) : "new" }})</span>
                          </td>
                        </tr>
                      </table>
                      <div v-if="editing && !locked('d', e.lootdrop_id)" class="zone-loot-add">
                        <b-form-input size="sm" placeholder="Add an item: name or id" style="max-width: 260px"
                                      :value="search.drop === e.lootdrop_id ? search.q : ''"
                                      @input="v => findItems(e.lootdrop_id, v)"/>
                        <div v-if="search.drop === e.lootdrop_id && search.results.length" class="zone-loot-results">
                          <div v-for="r in search.results" :key="'r' + r.id" class="zone-loot-result" @click="addItem(e.lootdrop_id, r)">
                            <span :class="'item-' + r.icon + '-sm'" class="zone-loot-icon mr-1"></span>
                            {{ r.name }} <span class="text-muted">{{ r.id }}<span v-if="r.reqlevel || r.reclevel">, level {{ r.reqlevel || r.reclevel }}</span></span>
                          </div>
                        </div>
                      </div>
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
            A snapshot keeps {{ data.zone }}'s spawn points (every version's), spawn groups and entries, its NPCs, their loot tables and
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
    <!-- make items -->
    <b-modal v-model="maker.show" size="xl" scrollable :title="makerTitle()" hide-footer>
      <div v-if="maker.npc">
        <div class="d-flex align-items-center flex-wrap mb-2">
          <span class="mr-1">Level</span>
          <b-form-input size="sm" type="number" min="1" max="255" v-model.number="maker.level" class="zone-loot-num mr-3"/>
          <span class="mr-2">Tier</span>
          <b-form-radio-group size="sm" v-model="maker.tier" :options="['trash', 'named', 'raid']" buttons
                              button-variant="outline-secondary" class="mr-3"/>
          <span class="mr-1">Names start</span>
          <b-form-input size="sm" v-model="maker.prefix" style="max-width: 220px"/>
        </div>
        <div class="hint text-muted mb-1">
          Stats come from what NPCs near level {{ maker.level }} drop for each piece, at the {{ tierWord() }} of those items.
          Each item is a copy of the comparable item nearest the suggestion (its type, slots, look and classes), without its effects.
        </div>
        <div class="mb-2">
          <b-form-checkbox-group v-model="maker.pieces" :options="(maker.catalog.pieces || []).map(p => ({ value: p.key, text: p.label }))"
                                 buttons button-variant="outline-secondary" size="sm" class="zone-loot-pieces"/>
        </div>
        <b-button size="sm" variant="outline-warning" class="mb-3" :disabled="!maker.pieces.length || maker.busy" @click="suggest()">
          <i :class="'fa ' + (maker.busy ? 'fa-cog fa-spin' : 'fa-lightbulb-o') + ' mr-1'"></i>
          {{ maker.rows.length ? "Suggest again" : "Suggest stats" }}
        </b-button>
        <info-error-banner :error="maker.error" @dismiss-error="maker.error = ''"/>

        <div v-for="(r, i) in maker.rows" :key="'mk' + i" class="zone-loot-maker-row">
          <div class="d-flex align-items-center flex-wrap">
            <span :class="'item-' + r.icon + '-sm'" class="zone-loot-icon mr-2"></span>
            <b-form-input size="sm" v-model="r.name" style="max-width: 280px" class="mr-2" maxlength="64"/>
            <span class="text-muted mr-2">{{ r.piece.label }}</span>
            <span class="mr-1">chance</span>
            <b-form-input size="sm" type="number" min="0" max="100" step="0.1" v-model.number="r.chance" class="zone-loot-num mr-1"/>
            <span class="mr-3">%</span>
            <b-form-checkbox v-model="r.nodrop" class="mr-2">No Drop</b-form-checkbox>
            <b-form-checkbox v-model="r.lore" class="mr-2">Lore</b-form-checkbox>
            <b-form-checkbox v-model="r.allClasses" class="mr-2">All classes</b-form-checkbox>
            <span v-if="!r.allClasses" class="text-muted hint mr-2">{{ classNames(r.template.classes) }}</span>
            <b-button size="sm" variant="outline-danger" class="ml-auto" title="Don't make this one" @click="maker.rows.splice(i, 1)">
              <i class="fa fa-times"></i>
            </b-button>
          </div>
          <div class="d-flex align-items-center flex-wrap mt-1 hint">
            <span class="mr-1">copy of</span>
            <b-form-select size="sm" :value="r.template.id" style="max-width: 300px" class="mr-2"
                           :options="r.examples.map(x => ({ value: x.id, text: x.name + ' (' + x.id + ')' }))"
                           @change="id => pickTemplate(r, id)"/>
            <span class="mr-1">icon</span>
            <b-form-input size="sm" type="number" min="0" v-model.number="r.icon" class="zone-loot-num mr-2"/>
            <span class="mr-1">model</span>
            <b-form-input size="sm" v-model="r.idfile" class="zone-loot-num mr-2" maxlength="30"/>
            <span class="mr-1">req / rec level</span>
            <b-form-input size="sm" type="number" min="0" max="255" v-model.number="r.reqlevel" class="zone-loot-num zone-loot-num-sm mr-1"/>
            <b-form-input size="sm" type="number" min="0" max="255" v-model.number="r.reclevel" class="zone-loot-num zone-loot-num-sm mr-2"/>
            <span class="text-muted">from {{ r.sample }} items NPCs level {{ r.levels[0] }}–{{ r.levels[1] }} drop</span>
          </div>
          <div class="d-flex align-items-center flex-wrap mt-1">
            <span v-for="k in statOrder(r.stats)" :key="'st' + i + k" class="zone-loot-stat">
              <span class="text-muted mr-1">{{ statLabel(k) }}</span>
              <b-form-input size="sm" type="number" :value="r.stats[k]" class="zone-loot-num zone-loot-num-sm"
                            @change="v => setStat(r, k, v)"/>
              <a href="#" class="ml-1 text-muted" title="Remove the stat" @click.prevent="$delete(r.stats, k)">×</a>
            </span>
            <b-form-select size="sm" :value="null" style="max-width: 130px" @change="k => k && setStat(r, k, 1)"
                           :key="'add' + i + '-' + Object.keys(r.stats).join()"
                           :options="[{ value: null, text: '+ stat' }].concat(maker.catalog.stats.filter(k => r.stats[k] === undefined).map(k => ({ value: k, text: statLabel(k) })))"/>
          </div>
        </div>

        <div v-if="maker.rows.length" class="d-flex align-items-center flex-wrap mt-3">
          <template v-if="!maker.group">
            <span class="mr-1">New drop group</span>
            <b-form-input size="sm" v-model="maker.groupName" style="max-width: 240px" class="mr-3"/>
          </template>
          <span class="mr-1">item ids from</span>
          <b-form-input size="sm" type="number" v-model.number="maker.start" class="zone-loot-num mr-1" style="width: 100px"/>
          <span class="mr-1">to</span>
          <b-form-input size="sm" type="number" v-model.number="maker.end" class="zone-loot-num mr-3" style="width: 100px"/>
          <b-button size="sm" variant="success" :disabled="maker.busy || !maker.rows.every(r => r.name.trim())" @click="make()">
            <i :class="'fa ' + (maker.busy ? 'fa-cog fa-spin' : 'fa-magic') + ' mr-1'"></i>
            Make {{ maker.rows.length }} item(s) and add them
          </b-button>
        </div>
      </div>
    </b-modal>
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

const PIECE_NOUNS = {
  head: "Helm", face: "Mask", ears: "Earring", neck: "Amulet", shoulders: "Pauldrons", arms: "Sleeves", back: "Cloak",
  wrist: "Bracer", hands: "Gauntlets", chest: "Breastplate", legs: "Greaves", feet: "Boots", waist: "Belt", fingers: "Ring",
  range: "Talisman", charm: "Charm", "1hs": "Blade", "2hs": "Greatsword", "1hp": "Dagger", "2hp": "Spear", "1hb": "Mace",
  "2hb": "Maul", h2h: "Knuckles", bow: "Bow", shield: "Shield",
}

const STAT_LABELS = {
  ac: "AC", hp: "HP", mana: "Mana", endur: "End", astr: "STR", asta: "STA", aagi: "AGI", adex: "DEX", awis: "WIS", aint: "INT",
  acha: "CHA", mr: "MR", fr: "FR", cr: "CR", dr: "DR", pr: "PR", damage: "Dmg", delay: "Delay", haste: "Haste", attack: "Atk",
  regen: "Regen", manaregen: "Mana regen", enduranceregen: "End regen", damageshield: "DS", shielding: "Shielding",
  spellshield: "Spell shield", avoidance: "Avoidance", accuracy: "Accuracy", strikethrough: "Strikethrough",
  stunresist: "Stun resist", dotshielding: "DoT shield", spelldmg: "Spell dmg", healamt: "Heal amt", clairvoyance: "Clairvoyance",
  heroic_str: "H.STR", heroic_sta: "H.STA", heroic_agi: "H.AGI", heroic_dex: "H.DEX", heroic_wis: "H.WIS", heroic_int: "H.INT",
  heroic_cha: "H.CHA",
}

const CLASS_NAMES = ["WAR", "CLR", "PAL", "RNG", "SHD", "DRU", "MNK", "BRD", "ROG", "SHM", "NEC", "WIZ", "MAG", "ENC", "BST", "BER"]

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
      version: parseInt(this.$route.query.version || "0", 10) || 0,
      data: null, // what's shown: the saved zone, or a preview of it with the unsaved changes
      base: null, // the saved zone
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

      // editing
      editing: false,
      pending: {},     // unsaved edits, by row
      everywhere: {},  // shared tables (t<id>) and drop groups (d<id>) unlocked for editing everywhere
      snapshotted: false,
      previewing: false,
      previewSeq: 0,
      previewTimer: null,
      saving: false,
      showPending: false,
      search: { drop: 0, q: "", results: [], timer: null },

      // making items
      maker: {
        show: false, npc: null, table: 0, group: 0, level: 1, tier: "named", prefix: "", pieces: [], rows: [],
        catalog: { pieces: [], stats: [] }, busy: false, error: "", groupName: "",
        start: Number(stored("spire-item-gen-start", 300000)), end: Number(stored("spire-item-gen-end", 999999)),
      },
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
      if (this.pendingCount() && !confirm("Discard your unsaved loot changes?")) {
        if (this.data) {
          this.version = this.data.version
        }
        return
      }
      this.loading = true
      this.error   = ""
      try {
        const r   = await SpireApi.v1().get(`zone-loot/${zone}`, { params: { version: this.version } })
        if (!this.data || this.data.zone !== zone) {
          this.snapshotted = false
          this.open = {}
          this.openItem = {}
        }
        this.data = r.data
        this.base = r.data
        this.pending = {}
        this.everywhere = {}
        this.diffs = {}
        try {
          localStorage.setItem(ZONE_KEY, zone)
        } catch (e) {
        }
        if (this.$route.query.zone !== zone || String(this.$route.query.version || 0) !== String(this.version)) {
          this.$router.replace({ query: { zone: zone, version: String(this.version) } }).catch(() => {})
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

    versionOptions() {
      const vs = this.data && this.data.zone === this.zoneInput.trim().toLowerCase() ? this.data.versions : [this.version]
      return vs.map(v => ({ value: v, text: String(v) }))
    },
    where() {
      return this.data.version ? `${this.data.zone} v${this.data.version}` : this.data.zone
    },

    // --- making items ---
    async openMaker(n, t, group) {
      const name = this.cleanName(n.name).replace(/^(a|an|the)\s+/i, "")
      Object.assign(this.maker, {
        show: true, npc: n, table: t.id, group: group, level: n.level, tier: n.role === "trash" ? "trash" : n.role,
        prefix: n.role === "trash" ? "" : name + "'s", rows: [], error: "",
        groupName: `${t.id}_${n.name.replace(/^#/, "")}_made`,
      })
      if (!this.maker.catalog.pieces.length) {
        try {
          const r = await SpireApi.v1().get(`zone-loot/${this.data.zone}/pieces`)
          this.maker.catalog = r.data
        } catch (e) {
          this.maker.error = this.message(e)
        }
      }
    },
    makerTitle() {
      const m = this.maker
      if (!m.npc) {
        return "Make items"
      }
      return `Make items for ${this.cleanName(m.npc.name)}: ` + (m.group ? `into drop group ${m.group}` : `a new drop group in loot table ${m.table}`)
    },
    tierWord() {
      return { trash: "median", named: "75th percentile", raid: "90th percentile" }[this.maker.tier]
    },
    async suggest() {
      const m = this.maker
      m.busy  = true
      m.error = ""
      try {
        const r = await SpireApi.v1().post(`zone-loot/${this.data.zone}/suggest`, { pieces: m.pieces, level: m.level, tier: m.tier })
        const chance = { trash: 2, named: 15, raid: 25 }[m.tier]
        const missing = r.data.filter(s => !s.template).map(s => s.piece.label)
        if (missing.length) {
          m.error = `Nothing near level ${m.level} drops to go on for: ${missing.join(", ")}`
        }
        m.rows = r.data.filter(s => s.template).map(s => ({
          piece: s.piece, sample: s.sample, levels: s.levels, examples: s.examples, template: s.template,
          name: ((m.prefix ? m.prefix.trim() + " " : "") + (PIECE_NOUNS[s.piece.key] || s.piece.label)).slice(0, 64),
          icon: s.template.icon, idfile: s.template.idfile, stats: { ...s.stats }, reqlevel: 0, reclevel: 0,
          chance: chance, nodrop: m.tier === "raid", lore: m.tier === "raid", allClasses: false,
        }))
      } catch (e) {
        m.error = this.message(e)
      } finally {
        m.busy = false
      }
    },
    pickTemplate(r, id) {
      const t = r.examples.find(x => x.id === id)
      if (t) {
        r.template = t
        r.icon     = t.icon
        r.idfile   = t.idfile
      }
    },
    statOrder(stats) {
      return this.maker.catalog.stats.filter(k => stats[k] !== undefined)
    },
    statLabel(k) {
      return STAT_LABELS[k] || k
    },
    setStat(r, k, v) {
      this.$set(r.stats, k, Math.round(parseFloat(v) || 0))
    },
    classNames(mask) {
      if ((mask & 65535) === 65535) {
        return "all classes"
      }
      return CLASS_NAMES.filter((c, i) => mask & (1 << i)).join(" ") || "no classes"
    },
    async make() {
      const m = this.maker
      m.busy  = true
      m.error = ""
      try {
        try {
          localStorage.setItem("spire-item-gen-start", String(m.start))
          localStorage.setItem("spire-item-gen-end", String(m.end))
        } catch (e) {
        }
        const r = await SpireApi.v1().post(`zone-loot/${this.data.zone}/make`, {
          version: this.data.version, start: m.start, end: m.end, loottable_id: m.table, lootdrop_id: m.group,
          group_name: m.groupName, everywhere: !!this.everywhere["t" + m.table], snapshot: !this.snapshotted,
          comment: `Spire Zone Loot for ${this.cleanName(m.npc.name)}`,
          items: m.rows.map(row => ({
            template_id: row.template.id, name: row.name, icon: row.icon, idfile: row.idfile, stats: row.stats,
            reqlevel: row.reqlevel, reclevel: row.reclevel, classes: row.allClasses ? 65535 : 0,
            nodrop: row.nodrop, lore: row.lore, chance: row.chance,
          })),
        })
        this.base = this.data = r.data.zone
        this.snapshotted = true
        m.show = false
        this.notification = `Made ${r.data.items.map(x => `${x.name} (${x.id})`).join(", ")} in drop group ${r.data.lootdrop_id}` +
          (r.data.snapshot ? `; how the zone was is kept as snapshot ${r.data.snapshot.id}` : "") +
          ". Restart the zone for them to drop."
        await this.loadSnapshots()
      } catch (e) {
        m.error = "Nothing was made: " + this.message(e)
      } finally {
        m.busy = false
      }
    },

    // --- editing ---
    pendingCount() {
      return Object.keys(this.pending).length
    },
    locked(kind, id) {
      const x = kind === "t" ? this.data.tables[id] : this.data.dropgroups[id]
      return this.isShared(x) && !this.everywhere[kind + id]
    },
    unlock(kind, id) {
      const what = kind === "t" ? `loot table ${id}` : `drop group ${id}`
      if (confirm(`Change ${what} everywhere? Its changes reach every NPC using it, outside ${this.where()} too.`)) {
        this.$set(this.everywhere, kind + id, true)
      }
    },
    usersHere(tableId) {
      return this.data.npcs.filter(n => n.loottable_id === tableId)
    },
    tablesHere(dropId) {
      return Object.values(this.data.tables).filter(t => t.entries.some(e => e.lootdrop_id === dropId))
    },
    num(v) {
      if (typeof v === "boolean") {
        return v
      }
      const x = parseFloat(v)
      return isNaN(x) ? 0 : x
    },
    stage(key, base, field, value) {
      const cur = this.pending[key] || base
      this.$set(this.pending, key, field ? { ...cur, [field]: value } : { ...cur })
      this.schedulePreview()
    },
    pendingValue(key, field, fallback) {
      const p = this.pending[key]
      return p && p[field] !== undefined ? p[field] : fallback
    },
    tableValue(t, field) {
      return this.pendingValue(`lt:${t.id}`, field, t[field])
    },
    setTable(t, field, v) {
      this.stage(`lt:${t.id}`, { op: "loottable", loottable_id: t.id, everywhere: !!this.everywhere["t" + t.id] }, field, Math.round(this.num(v)))
    },
    entryValue(t, e, field) {
      return this.pendingValue(`te:${t.id}:${e.lootdrop_id}`, field, e[field])
    },
    setEntry(t, e, field, v) {
      const value = field === "probability" ? this.num(v) : Math.round(this.num(v))
      this.stage(`te:${t.id}:${e.lootdrop_id}`, { op: "loottable_entry", loottable_id: t.id, lootdrop_id: e.lootdrop_id, everywhere: !!this.everywhere["t" + t.id] }, field, value)
    },
    removeEntry(t, e) {
      const key = `te:${t.id}:${e.lootdrop_id}`
      if (this.pending[key] && this.pending[key].added) {
        this.undo(key)
        return
      }
      this.$set(this.pending, key, { op: "loottable_entry", loottable_id: t.id, lootdrop_id: e.lootdrop_id, remove: true, everywhere: !!this.everywhere["t" + t.id] })
      this.schedulePreview()
    },
    addGroup(t, v) {
      const id = Math.round(this.num(v))
      if (id <= 0) {
        return
      }
      this.$set(this.pending, `te:${t.id}:${id}`, {
        op: "loottable_entry", loottable_id: t.id, lootdrop_id: id, multiplier: 1, droplimit: 1, mindrop: 0, probability: 100,
        added: true, everywhere: !!this.everywhere["t" + t.id],
      })
      this.schedulePreview()
    },
    useTable(n, v) {
      const id = Math.round(this.num(v))
      if (id < 0) {
        return
      }
      this.$set(this.pending, `nt:${n.id}`, { op: "npc_loottable", npc_id: n.id, loottable_id: id, name: n.name })
      this.schedulePreview()
    },
    itemValue(dropId, it, field) {
      return this.pendingValue(`le:${dropId}:${it.item_id}`, field, it[field])
    },
    setItem(dropId, it, field, v) {
      const value = field === "equip_item" ? !!v : field === "chance" ? this.num(v) : Math.round(this.num(v))
      this.stage(`le:${dropId}:${it.item_id}`, { op: "lootdrop_entry", lootdrop_id: dropId, item_id: it.item_id, everywhere: !!this.everywhere["d" + dropId] }, field, value)
    },
    removeItem(dropId, it) {
      const key = `le:${dropId}:${it.item_id}`
      if (this.pending[key] && this.pending[key].added) {
        this.undo(key)
        return
      }
      this.$set(this.pending, key, { op: "lootdrop_entry", lootdrop_id: dropId, item_id: it.item_id, remove: true, name: this.itemName(it.item_id), everywhere: !!this.everywhere["d" + dropId] })
      this.schedulePreview()
    },
    findItems(dropId, q) {
      this.search.drop = dropId
      this.search.q = q
      clearTimeout(this.search.timer)
      if (!q.trim()) {
        this.search.results = []
        return
      }
      this.search.timer = setTimeout(async () => {
        try {
          const r = await SpireApi.v1().get(`zone-loot/${this.data.zone}/items`, { params: { q: q } })
          if (this.search.q === q) {
            this.search.results = r.data
          }
        } catch (e) {
          this.error = this.message(e)
        }
      }, 250)
    },
    addItem(dropId, item) {
      this.$set(this.pending, `le:${dropId}:${item.id}`, {
        op: "lootdrop_entry", lootdrop_id: dropId, item_id: item.id, chance: 10, multiplier: 1,
        added: true, name: item.name, everywhere: !!this.everywhere["d" + dropId],
      })
      this.search = { drop: 0, q: "", results: [], timer: null }
      this.schedulePreview()
    },
    undo(key) {
      this.$delete(this.pending, key)
      this.schedulePreview()
    },
    discard() {
      this.pending = {}
      this.data = this.base
      this.error = ""
    },
    // the order they're applied in: an NPC's table first, then tables, then drop groups
    editList() {
      const order = { npc_loottable: 0, loottable: 1, loottable_entry: 2, lootdrop: 3, lootdrop_entry: 4 }
      return Object.values(this.pending)
        .map(e => {
          const { added, name, ...edit } = e
          return edit
        })
        .sort((a, b) => order[a.op] - order[b.op])
    },
    schedulePreview() {
      clearTimeout(this.previewTimer)
      this.previewTimer = setTimeout(() => this.previewEdits(), 300)
    },
    async previewEdits() {
      const edits = this.editList()
      const seq   = ++this.previewSeq
      if (!edits.length) {
        this.data = this.base
        this.previewing = false
        return
      }
      this.previewing = true
      try {
        const r = await SpireApi.v1().post(`zone-loot/${this.data.zone}/edit`, { version: this.data.version, edits: edits, preview: true })
        if (seq === this.previewSeq) {
          this.data  = r.data.zone
          this.error = ""
        }
      } catch (e) {
        if (seq === this.previewSeq) {
          this.error = "Can't preview: " + this.message(e)
        }
      } finally {
        if (seq === this.previewSeq) {
          this.previewing = false
        }
      }
    },
    async save() {
      const edits = this.editList()
      this.saving = true
      this.error  = ""
      try {
        const r = await SpireApi.v1().post(`zone-loot/${this.data.zone}/edit`, {
          version: this.data.version, edits: edits, snapshot: !this.snapshotted,
          note: `before editing loot (${edits.length} change${edits.length === 1 ? "" : "s"})`,
        })
        this.base = this.data = r.data.zone
        this.pending = {}
        this.showPending = false
        this.notification = `Saved ${edits.length} change(s)` +
          (r.data.snapshot ? `; how it was is kept as snapshot ${r.data.snapshot.id}` : "") + ". Restart the zone for them to take effect."
        this.snapshotted = true
        await this.loadSnapshots()
      } catch (e) {
        this.error = "Nothing was saved: " + this.message(e)
      } finally {
        this.saving = false
      }
    },
    structuralBlocked() {
      return this.pendingCount() > 0 || this.saving
    },
    structuralTitle() {
      return this.pendingCount() ? "Save or discard your changes first" : ""
    },
    // copies and new tables/groups are saved at once
    async act(edit, what) {
      if (!confirm(`${what}? This is saved at once.`)) {
        return
      }
      this.saving = true
      this.error  = ""
      try {
        const r = await SpireApi.v1().post(`zone-loot/${this.data.zone}/edit`, {
          version: this.data.version, edits: [edit], snapshot: !this.snapshotted, note: `before: ${what}`,
        })
        this.base = this.data = r.data.zone
        const made = r.data.created.map(c => `${c.table === "loottable" ? "loot table" : "drop group"} ${c.id}`).join(", ")
        this.notification = `${what}: made ${made}` + (r.data.snapshot ? `; how it was is kept as snapshot ${r.data.snapshot.id}` : "")
        this.snapshotted = true
        await this.loadSnapshots()
      } catch (e) {
        this.error = this.message(e)
      } finally {
        this.saving = false
      }
    },
    describe(e) {
      const item = id => e.name || this.itemName(id)
      const fields = { chance: "chance %", multiplier: "multiplier", item_charges: "charges", equip_item: "equipped", npc_min_level: "NPC min level",
        npc_max_level: "NPC max level", droplimit: "up to", mindrop: "at least", probability: "probability %", mincash: "min cash", maxcash: "max cash" }
      const vals = Object.keys(fields).filter(f => e[f] !== undefined).map(f => `${fields[f]} ${e[f]}`).join(", ")
      const all = e.everywhere ? " (everywhere)" : ""
      switch (e.op) {
        case "lootdrop_entry":
          if (e.remove) {
            return `Remove ${item(e.item_id)} from drop group ${e.lootdrop_id}${all}`
          }
          return `${e.added ? "Add" : "Change"} ${item(e.item_id)} in drop group ${e.lootdrop_id}: ${vals}${all}`
        case "loottable_entry":
          if (e.remove) {
            return `Take drop group ${e.lootdrop_id} out of loot table ${e.loottable_id}${all}`
          }
          return `${e.added ? "Add" : "Change"} drop group ${e.lootdrop_id} in loot table ${e.loottable_id}: ${vals}${all}`
        case "loottable":
          return `Loot table ${e.loottable_id}: ${vals}${all}`
        case "npc_loottable":
          return `${this.cleanName(e.name)} uses ${e.loottable_id ? "loot table " + e.loottable_id : "no loot table"}`
      }
      return e.op
    },
    baseExpected(n, itemId) {
      const b = this.base && this.base.npcs.find(x => x.id === n.id)
      return b ? this.expectedFor(b, itemId) : 0
    },
    changedRate(n, itemId) {
      return this.data !== this.base && Math.abs(this.expectedFor(n, itemId) - this.baseExpected(n, itemId)) > 1e-9
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
      const it = this.item(id) || (this.base && this.base.items[id])
      return (it || {}).name || `item ${id}`
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

.zone-loot-pending {
  position: sticky;
  top: 0;
  z-index: 5;
  background: rgba(20, 24, 34, 0.96);
  border: 1px solid rgba(255, 193, 7, 0.4);
  border-radius: 4px;
  padding: 8px 12px;
  margin-bottom: 12px;
}

.zone-loot-num {
  width: 78px;
  display: inline-block;
}

.zone-loot-num-sm {
  width: 58px;
}

.zone-loot-actions .btn {
  margin-right: 6px;
}

.zone-loot-add {
  position: relative;
  margin: 4px 0 4px 32px;
}

.zone-loot-results {
  position: absolute;
  z-index: 10;
  background: #1b1f2a;
  border: 1px solid rgba(255, 255, 255, 0.2);
  max-height: 300px;
  overflow-y: auto;
  min-width: 340px;
}

.zone-loot-result {
  padding: 3px 8px;
  cursor: pointer;
}

.zone-loot-maker-row {
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 4px;
  padding: 8px 10px;
  margin-bottom: 8px;
}

.zone-loot-stat {
  white-space: nowrap;
  margin: 2px 12px 2px 0;
}

.zone-loot-pieces {
  display: flex;
  flex-wrap: wrap;
}

.zone-loot-pieces >>> .btn {
  flex: 0 0 auto;
  white-space: nowrap;
  margin: 0 4px 4px 0;
  border-radius: 3px !important;
}

.zone-loot-result:hover {
  background: rgba(255, 255, 255, 0.1);
}
</style>
