<template>
  <div>
    <eq-window-simple title="Boss Cards">
      <div>
        A boss announces itself to the players near it with the zone-card look: the scene shrouds, its name rises
        in a display face with its epithet beneath, an emblem and a sting. It shows on the first pull of each fight
        (or when the boss spawns), to everyone within its radius who runs the Starfall DLL, at most once per player per
        repeat time; it never holds the keyboard or waits for input, since they may already be fighting.
      </div>
      <div class="text-muted hint mt-1">
        Zones read the table once: after saving, <code>#bosscard reload</code> in the zone, then target the boss and
        <code>#bosscard</code> (to you) or <code>#bosscard all</code> (to everyone near it). A quest can show one any time,
        for any NPC: <code>$npc->ShowBossCard()</code> or <code>$npc->ShowBossCard("The Undying", "It will not stay down.")</code>
        for a phase change.
      </div>
      <info-error-banner class="mt-2" :notification="notification" :error="error"
                         @dismiss-error="error = ''" @dismiss-notification="notification = ''"/>
    </eq-window-simple>

    <div class="row mt-3" v-if="list">
      <!-- the bosses -->
      <div class="col-lg-4">
        <eq-window-simple :title="'Bosses (' + list.bosses.length + ')'">
          <div class="bc-add mb-2">
            <b-form-input size="sm" v-model="search" placeholder="Add a boss: NPC name or id" @input="findNPCs"/>
            <div v-if="results.length" class="bc-results">
              <div v-for="r in results" :key="'r' + r.id" class="bc-result" @click="addBoss(r)">
                {{ clean(r.name) }} <span class="text-muted">{{ r.id }} · level {{ r.level }}<span v-if="r.zones"> · {{ r.zones }}</span></span>
                <span v-if="bossOf(r.id)" class="bc-badge">has a card</span>
              </div>
            </div>
          </div>
          <div v-if="!list.bosses.length" class="text-muted hint">No boss has a card yet.</div>
          <table v-else class="eq-table eq-highlight-rows bc-bosses">
            <tbody>
            <tr v-for="b in list.bosses" :key="'b' + b.npc_type_id" @click="edit(b)"
                :class="'bc-clickable' + (editing && editing.npc_type_id === b.npc_type_id ? ' bc-selected' : '')">
              <td>
                <div :style="listTitleStyle(b)">{{ b.title || clean(b.npc.name) || ('NPC ' + b.npc_type_id) }}</div>
                <div v-if="b.subtitle" class="hint bc-sub">{{ b.subtitle }}</div>
              </td>
              <td class="text-muted hint text-right" style="white-space: nowrap">
                {{ b.npc_type_id }}<span v-if="b.npc.zones"> · {{ b.npc.zones }}</span>
                <div>
                  <span class="bc-badge">{{ b.trigger === 'spawn' ? 'on spawn' : 'on pull' }}</span>
                  <span v-if="!b.enabled" class="bc-badge bc-badge-off">off</span>
                  <span v-if="!b.npc.found" class="bc-badge bc-badge-off">no such NPC</span>
                </div>
              </td>
            </tr>
            </tbody>
          </table>
        </eq-window-simple>
      </div>

      <!-- the card -->
      <div class="col-lg-8">
        <eq-window-simple v-if="!editing" title="Card">
          <div class="text-muted">Pick a boss, or add one.</div>
        </eq-window-simple>
        <eq-window-simple v-else :title="npcLabel() + (isNew ? ' (new)' : '') + (dirty() ? ' (not saved)' : '')">
          <title-card :card="previewCard()" :shown="!!editing.enabled" hidden-text="Off: no card for this boss"
                      caption="As it engages" :timing="timing()" @appear="cardAppears" @stop="stopSound"/>

          <div class="row mt-3">
            <div class="col-md-6 form-group mb-2">
              <label class="mb-1">Title <span class="text-muted hint">(empty: its own name)</span></label>
              <b-form-input v-model="editing.title" :placeholder="clean(editing.npc.name)" maxlength="63"/>
              <div class="hint text-right" :class="bytes(editing.title) > 63 ? 'text-danger' : 'text-muted'">{{ bytes(editing.title) }} / 63</div>
            </div>
            <div class="col-md-6 form-group mb-2">
              <label class="mb-1">Epithet</label>
              <b-form-input v-model="editing.subtitle" placeholder="Who Would Not Stay Dead" maxlength="127"/>
              <div class="hint text-right" :class="bytes(editing.subtitle) > 127 ? 'text-danger' : 'text-muted'">{{ bytes(editing.subtitle) }} / 127</div>
            </div>
          </div>

          <div class="bc-style">
            <div class="d-flex align-items-center flex-wrap mb-2">
              <span class="bc-label">Font</span>
              <b-form-radio-group size="sm" v-model="editing.title_font" buttons button-variant="outline-secondary" class="mr-3">
                <b-form-radio v-for="f in fonts" :key="f.value" :value="f.value">
                  <span :style="{ fontFamily: f.family, fontWeight: f.weight }">{{ f.text }}</span>
                </b-form-radio>
              </b-form-radio-group>
              <span class="bc-label">Reveal</span>
              <b-form-radio-group size="sm" v-model="editing.reveal" buttons button-variant="outline-secondary"
                                  :options="[{ value: 'fade', text: 'Fade' }, { value: 'rise', text: 'Rise' }, { value: 'letters', text: 'Letters' }]"/>
            </div>
            <div class="d-flex align-items-center flex-wrap mb-2">
              <template v-for="c in colors">
                <span :key="'cl' + c.key" class="bc-label">{{ c.label }}</span>
                <span :key="'cc' + c.key" class="bc-color mr-3">
                  <input type="color" :value="'#' + (editing[c.key] || c.def)" @input="ev => setColor(c.key, ev.target.value)">
                  <b-form-input size="sm" :value="editing[c.key]" :placeholder="c.placeholder || c.def" maxlength="7" class="bc-hex"
                                @change="v => setColor(c.key, v)"/>
                  <a v-if="editing[c.key]" href="#" class="hint ml-1" @click.prevent="editing[c.key] = ''">default</a>
                </span>
              </template>
            </div>
            <div class="d-flex align-items-center flex-wrap mb-2">
              <span class="bc-label">Emblem</span>
              <span v-if="editing.emblem_icon > 0" :class="'item-' + editing.emblem_icon + '-sm'" class="bc-icon-sm mr-1"></span>
              <b-form-input size="sm" type="number" min="0" v-model.number="editing.emblem_icon" class="bc-num mr-1" placeholder="icon"/>
              <b-button size="sm" variant="outline-secondary" class="mr-1" @click="picking = true">Pick</b-button>
              <b-button v-if="editing.emblem_icon > 0" size="sm" variant="outline-secondary" class="mr-3" @click="editing.emblem_icon = 0">None</b-button>
              <span class="bc-label ml-2">Stays up</span>
              <b-form-input size="sm" type="number" min="0" max="15" step="0.5" :value="editing.display_ms ? editing.display_ms / 1000 : ''"
                            class="bc-num mr-1" placeholder="4.5" @change="v => setDisplay(v)"/>
              <span class="hint text-muted">seconds (empty: the DLL's 4.5)</span>
            </div>
            <div class="d-flex align-items-center flex-wrap mb-2">
              <span class="bc-label">Shroud</span>
              <input type="range" min="0" max="90" v-model.number="editing.shroud" class="bc-range mr-2">
              <b-form-input size="sm" type="number" min="0" max="90" v-model.number="editing.shroud" class="bc-num bc-num-sm mr-1"/>
              <span class="mr-3">%</span>
              <span class="hint text-muted">a boss card is usually lighter than a zone's (35%): the fight is under it</span>
            </div>
            <div class="d-flex align-items-center flex-wrap mb-2">
              <span class="bc-label">Sound</span>
              <b-form-input size="sm" v-model="editing.sound" list="bc-sounds" placeholder="an MP3 in the client folder" maxlength="64"
                            style="max-width: 300px" class="mr-1"/>
              <datalist id="bc-sounds">
                <option v-for="s in list.sounds" :key="s" :value="s"/>
              </datalist>
              <b-button size="sm" variant="outline-secondary" :disabled="!soundHeld()" @click="playSound()"
                        :title="soundHeld() ? 'Hear it' : 'Spire keeps no copy of it to play'"><i class="fa fa-volume-up"></i></b-button>
              <span v-if="editing.sound.trim() && !soundHeld()" class="hint text-warning ml-2">not in Spire's client patch</span>
            </div>
          </div>

          <h6 class="bc-h">When</h6>
          <div class="d-flex align-items-center flex-wrap mb-2">
            <b-form-radio-group size="sm" v-model="editing.trigger" buttons button-variant="outline-secondary" class="mr-3"
                                :options="[{ value: 'engage', text: 'On the first pull' }, { value: 'spawn', text: 'When it spawns' }]"/>
            <span class="bc-label">Within</span>
            <b-form-input size="sm" type="number" min="1" max="5000" v-model.number="editing.radius" class="bc-num mr-1"/>
            <span class="mr-3">units</span>
            <span class="bc-label">Again after</span>
            <b-form-input size="sm" type="number" min="0" v-model.number="editing.repeat_seconds" class="bc-num mr-1"/>
            <span class="hint text-muted">s per player ({{ duration(editing.repeat_seconds) }})</span>
          </div>
          <div class="hint text-muted mb-2">
            {{ editing.trigger === 'spawn' ? 'When it spawns: an arrival, a world event.' :
              'The first pull of each fight, as it starts: not when a raid zones in.' }}
            A wipe and re-pull within the repeat time doesn't show it again to the same player.
          </div>

          <div class="d-flex align-items-center flex-wrap mb-3">
            <b-form-checkbox v-model="editing.enabled" :value="1" :unchecked-value="0" class="mr-3">Enabled</b-form-checkbox>
            <b-form-input size="sm" v-model="editing.notes" maxlength="255" placeholder="Notes (players don't see them)"/>
          </div>

          <div class="d-flex align-items-center flex-wrap">
            <b-button size="sm" variant="success" class="mr-2" :disabled="!canSave()" @click="save()">
              <i :class="'fa ' + (busy ? 'fa-cog fa-spin' : 'fa-save') + ' mr-1'"></i> Save
            </b-button>
            <b-button size="sm" variant="outline-secondary" class="mr-2" @click="close()">Close</b-button>
            <b-button v-if="!isNew" size="sm" variant="outline-danger" class="ml-auto" :disabled="busy" @click="remove()">Delete</b-button>
          </div>

          <div class="mt-3 hint text-muted">
            As SQL, for a migration or another server:
            <pre class="bc-sql">{{ sql() }}</pre>
          </div>
        </eq-window-simple>
      </div>
    </div>

    <b-modal v-model="picking" size="xl" title="Emblem: an item icon" hide-footer>
      <item-icon-selector v-if="picking" :selected-icon="editing ? editing.emblem_icon || 0 : 0"
                          @input="id => { editing.emblem_icon = id; picking = false }"/>
    </b-modal>
  </div>
</template>

<script>
import EqWindowSimple   from "@/components/eq-ui/EQWindowSimple";
import InfoErrorBanner  from "@/components/InfoErrorBanner";
import ItemIconSelector from "@/views/items/components/ItemIconSelector";
import TitleCard, {DEFAULT_COLORS, FAMILIES, FONTS} from "@/components/starfall/TitleCard";
import {SpireApi}       from "@/app/api/spire-api";

// a new boss card: the table's defaults (custom migration 74)
function blank(npc) {
  return {
    npc_type_id: npc.id, title: "", subtitle: "", trigger: "engage", radius: 300, title_font: "", title_color: "", subtitle_color: "",
    accent_color: "", emblem_icon: 0, reveal: "rise", display_ms: 3500, shroud: 35, shroud_color: "", sound: "",
    repeat_seconds: 3600, enabled: 1, notes: "", npc: npc,
  }
}

const FIELDS = ["title", "subtitle", "trigger", "radius", "title_font", "title_color", "subtitle_color", "accent_color", "emblem_icon",
  "reveal", "display_ms", "shroud", "shroud_color", "sound", "repeat_seconds", "enabled", "notes"]

export default {
  name: "BossCards",
  components: { EqWindowSimple, InfoErrorBanner, ItemIconSelector, TitleCard },
  data() {
    return {
      list: null,
      search: "",
      results: [],
      timer: null,
      editing: null,
      original: "",
      isNew: false,
      fonts: FONTS,
      colors: [
        { key: "title_color", label: "Title", def: DEFAULT_COLORS.title_color },
        { key: "subtitle_color", label: "Epithet", def: DEFAULT_COLORS.subtitle_color },
        { key: "accent_color", label: "Lines", def: DEFAULT_COLORS.accent_color },
        { key: "shroud_color", label: "Shroud", def: "000000", placeholder: "black" },
      ],
      audio: null,
      picking: false,
      busy: false,
      error: "",
      notification: "",
    }
  },
  async mounted() {
    await this.load()
    const id = parseInt(this.$route.query.npc)
    const b = id && this.list.bosses.find(x => x.npc_type_id === id)
    if (b) {
      this.edit(b)
    }
  },
  beforeDestroy() {
    this.stopSound()
  },
  methods: {
    async load() {
      try {
        const r = await SpireApi.v1().get("zone-cards/bosses")
        this.list = r.data
      } catch (e) {
        this.error = this.message(e)
      }
    },
    message(e) {
      return (e.response && e.response.data && e.response.data.error) || e.message || String(e)
    },
    // EQEmu's CleanMobName: _ to a space, letters and ` kept
    clean(name) {
      return (name || "").replace(/_/g, " ").replace(/[^A-Za-z` ]/g, "")
    },
    bossOf(id) {
      return this.list.bosses.find(b => b.npc_type_id === id)
    },
    listTitleStyle(b) {
      const f = FAMILIES[b.title_font || ""] || FAMILIES[""]
      return { fontFamily: f.family, fontWeight: f.weight }
    },
    bossName() {
      return this.clean(this.editing.npc.name) || `NPC ${this.editing.npc_type_id}`
    },
    npcLabel() {
      const n = this.editing.npc
      return `${this.clean(n.name) || "NPC"} (${this.editing.npc_type_id})` + (n.zones ? ` · ${n.zones}` : "")
    },
    findNPCs() {
      clearTimeout(this.timer)
      const q = this.search
      if (!q.trim()) {
        this.results = []
        return
      }
      this.timer = setTimeout(async () => {
        try {
          const r = await SpireApi.v1().get("zone-cards/bosses/npcs", { params: { q: q } })
          if (this.search === q) {
            this.results = r.data
          }
        } catch (e) {
          this.error = this.message(e)
        }
      }, 250)
    },
    addBoss(npc) {
      this.search  = ""
      this.results = []
      const b = this.bossOf(npc.id)
      if (b) {
        this.edit(b)
        return
      }
      if (!this.leave()) {
        return
      }
      this.editing  = blank({ ...npc, found: true })
      this.original = ""
      this.isNew    = true
    },
    edit(b) {
      if (!this.leave()) {
        return
      }
      this.stopSound()
      this.editing  = { ...b, npc: { ...b.npc } }
      this.original = this.snapshot()
      this.isNew    = false
      if (String(this.$route.query.npc) !== String(b.npc_type_id)) {
        this.$router.replace({ query: { npc: String(b.npc_type_id) } }).catch(() => {})
      }
    },
    leave() {
      return !this.dirty() || confirm("Leave your changes to this card?")
    },
    close() {
      if (this.leave()) {
        this.editing = null
      }
    },
    snapshot() {
      const e = this.editing
      return JSON.stringify(FIELDS.map(f => e[f]))
    },
    dirty() {
      return !!this.editing && this.snapshot() !== this.original
    },
    bytes(s) {
      return new TextEncoder().encode((s || "").trim()).length
    },
    canSave() {
      const e = this.editing
      const hex = c => !c || /^[0-9A-Fa-f]{6}$/.test(c)
      return !this.busy && (this.isNew || this.dirty()) && this.bytes(e.title) <= 63 && this.bytes(e.subtitle) <= 127 &&
        hex(e.title_color) && hex(e.subtitle_color) && hex(e.accent_color) && hex(e.shroud_color) &&
        e.radius >= 1 && e.radius <= 5000 && e.shroud >= 0 && e.shroud <= 90
    },
    setColor(key, v) {
      this.editing[key] = String(v || "").trim().replace(/^#/, "").toUpperCase()
    },
    setDisplay(v) {
      const s = parseFloat(v)
      this.editing.display_ms = isNaN(s) || s <= 0 ? 0 : Math.min(15000, Math.round(s * 1000))
    },
    duration(s) {
      s = parseInt(s) || 0
      if (!s) {
        return "every time"
      }
      if (s >= 3600) {
        return Math.round(s / 360) / 10 + " h"
      }
      return s >= 60 ? Math.round(s / 6) / 10 + " min" : s + " s"
    },
    previewCard() {
      const e = this.editing
      return { ...e, title: e.title.trim() || this.clean(e.npc.name) || "The Boss", shroud: parseInt(e.shroud) || 0 }
    },
    // a boss card is always timed (display_ms 0: the DLL's default)
    timing() {
      return { displayMs: this.editing.display_ms || 4500, untilInput: false, minMs: 0, maxMs: 0, outMs: 1600 }
    },
    soundHeld() {
      const s = (this.editing.sound || "").trim().replace(/\\/g, "/").toLowerCase()
      return !!s && this.list.sounds.some(x => x.toLowerCase() === s)
    },
    cardAppears() {
      if (this.soundHeld()) {
        this.playSound()
      }
    },
    stopSound() {
      if (this.audio) {
        this.audio.pause()
      }
    },
    async playSound() {
      try {
        this.stopSound()
        const r = await SpireApi.v1().get("zone-cards/sound", { params: { path: this.editing.sound.trim() }, responseType: "blob" })
        this.audio = new Audio(URL.createObjectURL(r.data))
        await this.audio.play()
      } catch (e) {
        this.error = "Can't play " + this.editing.sound + ": " + this.message(e)
      }
    },
    sql() {
      const e = this.editing
      const q = s => "'" + String(s || "").trim().replace(/\\/g, "\\\\").replace(/'/g, "''") + "'"
      const cols = { npc_type_id: e.npc_type_id }
      FIELDS.forEach(f => (cols[f] = typeof e[f] === "number" ? e[f] : q(e[f])))
      const names = Object.keys(cols).map(n => n === "trigger" ? "`trigger`" : n)
      return `INSERT INTO custom_boss_card (${names.join(", ")})\nVALUES (${Object.values(cols).join(", ")})\n` +
        `ON DUPLICATE KEY UPDATE ${names.slice(1).map(n => `${n} = VALUES(${n})`).join(", ")};`
    },
    async save() {
      this.busy  = true
      this.error = ""
      try {
        const e = this.editing
        const body = {}
        FIELDS.forEach(f => (body[f] = e[f]))
        ;["radius", "emblem_icon", "display_ms", "shroud", "repeat_seconds"].forEach(f => (body[f] = parseInt(body[f]) || 0))
        await SpireApi.v1().put(`zone-cards/bosses/${e.npc_type_id}`, body)
        this.notification = `Saved ${this.bossName()}'s card. In its zone, #bosscard reload picks it up.`
        this.original = this.snapshot()
        const id = e.npc_type_id
        await this.load()
        this.editing = null
        this.edit(this.bossOf(id))
      } catch (e) {
        this.error = this.message(e)
      } finally {
        this.busy = false
      }
    },
    async remove() {
      if (!confirm(`Delete ${this.bossName()}'s boss card?`)) {
        return
      }
      this.busy = true
      try {
        await SpireApi.v1().delete(`zone-cards/bosses/${this.editing.npc_type_id}`)
        this.notification = `Deleted ${this.bossName()}'s card.`
        this.editing = null
        await this.load()
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

.bc-add {
  position: relative;
}

.bc-results {
  position: absolute;
  z-index: 10;
  left: 0;
  right: 0;
  background: #1b1f2a;
  border: 1px solid rgba(255, 255, 255, 0.2);
  max-height: 340px;
  overflow-y: auto;
}

.bc-result {
  padding: 4px 8px;
  cursor: pointer;
}

.bc-result:hover {
  background: rgba(255, 255, 255, 0.1);
}

.bc-clickable {
  cursor: pointer;
}

.bc-selected > td {
  background: rgba(212, 175, 55, 0.18);
}

.bc-sub {
  color: #c9b98a;
  font-style: italic;
}

.bc-badge {
  font-size: 0.8em;
  padding: 0 6px;
  border-radius: 3px;
  margin-left: 4px;
  background: rgba(212, 175, 55, 0.3);
  color: #f1e3b0;
}

.bc-badge-off {
  background: rgba(220, 53, 69, 0.35);
}

.bc-style {
  border-top: 1px solid rgba(255, 255, 255, 0.1);
  padding-top: 10px;
}

.bc-h {
  margin: 10px 0 6px;
  color: #d4af37;
  font-size: 0.9em;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.bc-label {
  margin-right: 6px;
  color: #bbb;
}

.bc-color {
  display: inline-flex;
  align-items: center;
}

.bc-color input[type=color] {
  width: 30px !important;
  min-width: 30px;
  height: 26px !important;
  flex: 0 0 30px;
  padding: 0 !important;
  border: 1px solid rgba(255, 255, 255, 0.3) !important;
  border-radius: 3px;
  background: none !important;
  margin-right: 4px;
  cursor: pointer;
}

.bc-hex {
  width: 84px;
}

.bc-num {
  width: 80px;
}

.bc-num-sm {
  width: 64px;
}

.bc-range {
  width: 160px;
  vertical-align: middle;
}

.bc-icon-sm {
  display: inline-block;
  vertical-align: middle;
}

.bc-sql {
  background: rgba(0, 0, 0, 0.35);
  color: #ddd;
  padding: 6px 8px;
  white-space: pre-wrap;
  margin: 4px 0 0;
}
</style>
