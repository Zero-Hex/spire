<template>
  <div>
    <eq-window-simple title="Boss Cards">
      <div>
        A boss announces itself to the players near it with the zone-card look: the scene shrouds, its name rises
        in a display face with its epithet beneath, an emblem and a sting. It shows on the first pull of each fight
        (or when the boss spawns), to everyone within its radius who runs the Starfall DLL, at most once per player per
        repeat time; it never holds the keyboard or waits for input, since they may already be fighting.
        It can also show players the boss itself: a portrait taken from the camera's view of it, or a live shot.
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
                  <span v-if="b.camera_ms > 0" class="bc-badge bc-badge-cam" :title="cameraSummary(b)">
                    <i class="fa fa-camera"></i> {{ b.camera_style === 'live' ? 'live' : 'portrait' }}
                  </span>
                  <span v-if="!b.enabled" class="bc-badge bc-badge-off">off</span>
                  <span v-if="!b.npc.found" class="bc-badge bc-badge-off">no such NPC</span>
                </div>
              </td>
            </tr>
            </tbody>
          </table>
        </eq-window-simple>

        <eq-window-simple v-if="rules" title="Portraits: every boss" class="mt-3 bc-rules">
          <div class="hint text-muted mb-2">
            Server rules every boss portrait follows. The camera stands straight in front of the boss, far enough to
            see all of it; the portrait then shows a crop of that frame around the boss.
          </div>
          <div class="d-flex align-items-center flex-wrap mb-2">
            <span class="bc-label bc-rule-label">Camera back</span>
            <b-form-input size="sm" type="number" min="0.1" max="25" step="0.1" v-model.number="rules.distance_per_size"
                          class="bc-num bc-num-sm mr-1 bc-rule-dist"/>
            <span class="mr-2">× size, at least</span>
            <b-form-input size="sm" type="number" min="1" max="500" step="1" v-model.number="rules.distance_min"
                          class="bc-num bc-num-sm bc-rule-min"/>
          </div>
          <div class="d-flex align-items-center flex-wrap mb-2">
            <span class="bc-label bc-rule-label">Camera up</span>
            <b-form-input size="sm" type="number" min="0" max="10" step="0.05" v-model.number="rules.height_per_size"
                          class="bc-num bc-num-sm mr-1 bc-rule-height"/>
            <span>× size</span>
          </div>
          <div class="d-flex align-items-center flex-wrap mb-2">
            <span class="bc-label bc-rule-label">Crop zoom</span>
            <input type="range" min="1" max="8" step="0.1" v-model.number="rules.crop_zoom" class="bc-range-sm mr-2">
            <b-form-input size="sm" type="number" min="1" max="8" step="0.1" v-model.number="rules.crop_zoom"
                          class="bc-num bc-num-sm mr-1 bc-rule-zoom"/>
            <span class="hint text-muted">× (1: the whole frame)</span>
          </div>
          <div class="d-flex align-items-center flex-wrap mb-2">
            <span class="bc-label bc-rule-label">Crop centre</span>
            <input type="range" min="0.01" max="0.99" step="0.01" v-model.number="rules.crop_center_y" class="bc-range-sm mr-2">
            <b-form-input size="sm" type="number" min="0.01" max="0.99" step="0.01" v-model.number="rules.crop_center_y"
                          class="bc-num bc-num-sm mr-1 bc-rule-center"/>
            <span class="hint text-muted">down (0 top, 1 bottom)</span>
          </div>
          <div class="d-flex align-items-center flex-wrap mb-2">
            <b-form-checkbox class="mr-2 bc-rule-hide" :checked="rules.hide_ui_value >= 0"
                             @change="v => (rules.hide_ui_value = v ? 3 : -1)">Hide the client's windows</b-form-checkbox>
            <template v-if="rules.hide_ui_value >= 0">
              <span class="bc-label">screen mode</span>
              <b-form-input size="sm" type="number" min="0" max="254" v-model.number="rules.hide_ui_value" class="bc-num bc-num-sm mr-1"/>
              <span class="hint text-muted">(3 on RoF2)</span>
            </template>
          </div>
          <div class="hint text-danger mb-2" v-if="rulesProblem()">{{ rulesProblem() }}</div>
          <div class="d-flex align-items-center flex-wrap">
            <b-button size="sm" variant="success" class="mr-2 bc-rules-save" :disabled="!canSaveRules()" @click="saveRules()">
              <i :class="'fa ' + (busy ? 'fa-cog fa-spin' : 'fa-save') + ' mr-1'"></i> Save rules
            </b-button>
            <b-button v-if="rulesDirty()" size="sm" variant="outline-secondary" class="mr-2" @click="resetRules(false)">Undo</b-button>
            <a v-if="!rulesAreDefaults()" href="#" class="hint" @click.prevent="resetRules(true)">defaults</a>
          </div>
          <div class="hint text-muted mt-2">
            <template v-if="list.camera.rules_editable">
              Saved to ruleset 1. Zones read rules at start: <code>#reload rules</code> picks them up now. The preview
              follows these fields as you change them. Try numbers on one boss first:
              <code>#bosscam snap 5 3 3 portrait &lt;distance&gt; &lt;height&gt; z&lt;zoom&gt; y&lt;centre&gt;</code>.
            </template>
            <span v-else class="text-warning">This server's database has no <code>rule_values</code> table to save them in.</span>
          </div>
        </eq-window-simple>
      </div>

      <!-- the card -->
      <div class="col-lg-8">
        <eq-window-simple v-if="!editing" title="Card">
          <div class="text-muted">Pick a boss, or add one.</div>
        </eq-window-simple>
        <eq-window-simple v-else :title="npcLabel() + (isNew ? ' (new)' : '') + (dirty() ? ' (not saved)' : '')">
          <boss-portrait v-if="portrait() && editing.enabled" :key="'bp' + (portraitsOff ? 1 : 0)" :title="previewCard().title"
                         :subtitle="editing.subtitle.trim()" :card="editing" :npc="editing.npc" :ms="parseInt(editing.camera_ms)"
                         :framing="!editing.camera_mode" :rules="rules" :text-only="portraitsOff"
                         :caption="portraitCaption()" @appear="cardAppears" @stop="stopSound"/>
          <title-card v-else :card="previewCard()" :shown="!!editing.enabled" hidden-text="Off: no card for this boss"
                      :caption="cardCaption()" :timing="timing()" @appear="cardAppears" @stop="stopSound"/>
          <div v-if="editing.camera_ms > 0 && editing.enabled" class="d-flex align-items-center flex-wrap hint text-muted mt-1">
            <b-form-checkbox v-model="portraitsOff" class="mr-2 bc-portraits-off">
              As a player with Options › Windows › Boss portraits off sees it
            </b-form-checkbox>
            <span>
              {{ portraitsOff ? (portrait() ? "(the same window, text only: no flash, no capture)" : "(just the card: no camera)") : "" }}
            </span>
          </div>
          <div v-if="portrait() && editing.enabled" class="hint text-muted mt-1">
            The portrait is a window players can drag and resize (it opens where they last left it; Options › Windows ›
            Place the boss card positions it). It takes the card's font, title and epithet colours, a frame in the accent
            colour, and its sound; the emblem, reveal, shroud and time below are for the full card, which nobody gets
            while the shot is a portrait.
          </div>

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

          <h6 class="bc-h">Camera</h6>
          <div v-if="!list.camera.shot" class="hint text-muted mb-2">
            Showing players the boss needs the <code>camera_ms</code> column, which this server's
            <code>custom_boss_card</code> doesn't have yet (Starfall custom migration 75, run when world starts on a
            build that has it).
          </div>
          <template v-else>
            <div class="d-flex align-items-center flex-wrap mb-2">
              <b-form-checkbox switch class="mr-3 bc-cam-on" :checked="editing.camera_ms > 0" @change="v => setCamera(v)">
                Show the boss
              </b-form-checkbox>
              <template v-if="editing.camera_ms > 0">
                <span class="bc-label">For</span>
                <b-form-input size="sm" type="number" min="1" max="6" step="0.5" :value="editing.camera_ms / 1000"
                              class="bc-num bc-num-sm mr-1 bc-cam-secs" @change="v => setCameraSecs(v)"/>
                <span class="mr-3">seconds <span class="hint text-muted">(1 to 6)</span></span>
                <span class="bc-label">As</span>
                <b-form-radio-group size="sm" v-model="editing.camera_style" buttons button-variant="outline-secondary"
                                    class="mr-3 bc-cam-style" :options="styleOptions()"/>
              </template>
            </div>
            <template v-if="editing.camera_ms > 0">
              <div class="hint text-muted mb-2">
                <template v-if="editing.camera_style === 'live'">
                  <b>Live shot:</b> players near it who aren't fighting (no aggro) get the full card, and their camera
                  goes to the boss for {{ secs(editing.camera_ms) }} with their keyboard and mouse held, then comes back
                  exactly as it was. Taking damage ends it at once; players already fighting get just the card. A
                  player running at the time keeps running blind, so mind cliffs and lava near it.
                </template>
                <template v-else>
                  <b>Portrait:</b> everyone in range, fighting or running, gets it instead of the full card: behind a
                  warm flash their camera goes to the boss for a few frames, one frame is captured and the camera comes
                  straight back (no input held). The frame then shows for {{ secs(editing.camera_ms) }}, framed, with the
                  title and epithet beneath, and the sound plays.
                </template>
              </div>
              <div v-if="!list.camera.style" class="hint text-warning mb-2">
                Portraits need the <code>camera_style</code> column (Starfall custom migration 77): until then this
                server's shot is live.
              </div>
              <div class="d-flex align-items-center flex-wrap mb-2">
                <span class="bc-label">Framing</span>
                <b-form-select size="sm" v-model="editing.camera_mode" :options="modeOptions()" :disabled="!list.camera.mode"
                               class="bc-cam-mode mr-2" style="max-width: 260px"/>
                <span class="hint text-muted bc-cam-hint">{{ modeHint() }}</span>
              </div>
              <div v-if="portrait() && !editing.camera_mode" class="hint text-muted mb-2">
                By the boss's NPC <i>size</i> and the rules under <b>Portraits: every boss</b>: the camera
                {{ rules.distance_per_size }} × size back (at least {{ rules.distance_min }}) and {{ rules.height_per_size }}
                × size up, then a {{ rules.crop_zoom }}× crop centred {{ rules.crop_center_y }} down the frame.
              </div>
              <div v-else-if="portrait()" class="hint text-muted mb-2">
                Not the portrait framing: the portrait shows this camera mode's whole frame (no crop, no sizing).
              </div>
              <div v-if="!list.camera.mode" class="hint text-warning mb-2">
                Framing needs the <code>camera_mode</code> column (Starfall custom migration 76): until then each player's
                own camera mode is used.
              </div>
              <div v-if="editing.camera_style !== 'live'" class="hint text-muted mb-2">
                {{ rules.hide_ui_value >= 0 ? "The client's windows (chat, hotbars) are hidden while the frame is captured." :
                  "The frame catches the client's windows (chat, hotbars) too." }}
                Try one on your own client: target the boss, <code>#bosscam snap [seconds] [frames] [ui] [view]</code>
                (view: portrait, front, far, close, side, top, first or keep; raise <i>frames</i> if the portrait catches the
                camera half-moved).
              </div>
            </template>
          </template>

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
import BossPortrait     from "@/components/starfall/BossPortrait";
import {SpireApi}       from "@/app/api/spire-api";

// a new boss card: the table's defaults (custom migrations 74-77); without
// migration 76 or 77 a shot keeps the player's own camera mode and is live
function blank(npc, camera) {
  return {
    npc_type_id: npc.id, title: "", subtitle: "", trigger: "engage", radius: 300, title_font: "", title_color: "", subtitle_color: "",
    accent_color: "", emblem_icon: 0, reveal: "rise", display_ms: 3500, camera_ms: 0, camera_mode: camera.mode ? "" : "keep",
    camera_style: camera.style ? "snap" : "live", shroud: 35, shroud_color: "", sound: "", repeat_seconds: 3600, enabled: 1, notes: "", npc: npc,
  }
}

const FIELDS = ["title", "subtitle", "trigger", "radius", "title_font", "title_color", "subtitle_color", "accent_color", "emblem_icon",
  "reveal", "display_ms", "shroud", "shroud_color", "sound", "repeat_seconds", "enabled", "notes", "camera_ms", "camera_mode", "camera_style"]

// the portrait rules' defaults (ruletypes.h)
const PORTRAIT_DEFAULTS = { hide_ui_value: 3, distance_per_size: 2, distance_min: 10, height_per_size: 0.4, crop_zoom: 2.2, crop_center_y: 0.7 }

// the camera columns, with the flag for each in the list's camera
const CAMERA = { camera_ms: "shot", camera_mode: "mode", camera_style: "style" }

// the shot's framings (boss_cards.cpp CameraMode; the client's F9 modes)
const MODES = [
  { value: "", text: "Auto", hint: "players in first person see it from far behind; everyone else through their own camera mode" },
  { value: "keep", text: "Their own view", hint: "everyone's own camera mode: a player in first person sees out of the boss's eyes" },
  { value: "first", text: "First person", hint: "from inside the boss, for everyone" },
  { value: "top", text: "From above", hint: "the top-down view, for everyone" },
  { value: "close", text: "Close behind", hint: "the chase view, close behind it, for everyone" },
  { value: "side", text: "Behind, to the side", hint: "for everyone" },
  { value: "far", text: "Far behind", hint: "for everyone" },
  { value: "front", text: "From the front", hint: "facing it, side-on, for everyone" },
]

export default {
  name: "BossCards",
  components: { EqWindowSimple, InfoErrorBanner, ItemIconSelector, TitleCard, BossPortrait },
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
      // the portrait rules being edited (Custom:Boss_Portrait_*, Boss_Snap_Hide_Ui_Value)
      rules: null,
      rulesOriginal: "",
      portraitsOff: false,
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
        if (!this.rulesDirty()) {
          this.rules = this.rulesFrom(r.data.camera)
          this.rulesOriginal = JSON.stringify(this.rules)
        }
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
      this.editing  = blank({ ...npc, found: true }, this.list.camera)
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
        e.radius >= 1 && e.radius <= 5000 && e.shroud >= 0 && e.shroud <= 90 &&
        (!e.camera_ms || (e.camera_ms >= 1000 && e.camera_ms <= 6000))
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
    secs(ms) {
      return (ms / 1000).toFixed(1).replace(/\.0$/, "") + " s"
    },
    // the fields this server has a column for
    fields() {
      return FIELDS.filter(f => !CAMERA[f] || this.list.camera[CAMERA[f]])
    },
    setCamera(on) {
      this.editing.camera_ms = on ? 3000 : 0
    },
    setCameraSecs(v) {
      const s = parseFloat(v)
      this.editing.camera_ms = isNaN(s) || s <= 0 ? 0 : Math.min(6000, Math.max(1000, Math.round(s * 1000)))
    },
    styleOptions() {
      return [
        { value: "snap", text: "A portrait", disabled: !this.list.camera.style },
        { value: "live", text: "A live shot" },
      ]
    },
    modeOptions() {
      return MODES.map(m => ({ value: m.value, text: m.value === "" && this.portrait() ? "Portrait (auto)" : m.text }))
    },
    // a portrait's auto is the portrait framing (boss_cards.cpp): the camera
    // straight in front of the boss, its distance and height by its size
    // (PortraitDistance, PortraitHeight; Custom:Boss_Portrait_*)
    portraitFraming() {
      const c = this.rules
      const size = this.editing.npc.size > 0 ? this.editing.npc.size : 6
      const r = v => Math.round(v * 10) / 10
      return {
        size: r(size),
        distance: r(Math.min(500, Math.max(c.distance_min, size * c.distance_per_size, 1))),
        height: r(Math.min(200, Math.max(-200, size * c.height_per_size))),
      }
    },
    modeHint() {
      const m = MODES.find(x => x.value === this.editing.camera_mode)
      if (!m) {
        return ""
      }
      if (this.portrait() && m.value === "") {
        const f = this.portraitFraming()
        return `straight in front of it, ${f.distance} back and ${f.height} up for its size ${f.size}`
      }
      return m.hint
    },
    // the shot is a portrait (which replaces the card)
    portrait() {
      return this.editing.camera_ms > 0 && this.editing.camera_style !== "live"
    },
    portraitCaption() {
      return (this.editing.trigger === "spawn" ? "As it spawns" : "As it engages") +
        (this.portraitsOff ? ": its card, text only" : ": its portrait (Spire's model picture stands in for the captured frame)")
    },
    rulesFrom(camera) {
      const { hide_ui_value, distance_per_size, distance_min, height_per_size, crop_zoom, crop_center_y } = camera
      return { hide_ui_value, distance_per_size, distance_min, height_per_size, crop_zoom, crop_center_y }
    },
    rulesDirty() {
      return !!this.rules && JSON.stringify(this.rules) !== this.rulesOriginal
    },
    rulesAreDefaults() {
      return JSON.stringify(this.rules) === JSON.stringify(PORTRAIT_DEFAULTS)
    },
    resetRules(defaults) {
      this.rules = defaults ? { ...PORTRAIT_DEFAULTS } : JSON.parse(this.rulesOriginal)
    },
    // the ranges SavePortraitRules allows
    rulesProblem() {
      const r = this.rules
      const bad = (v, lo, hi) => typeof v !== "number" || isNaN(v) || v < lo || v > hi
      if (bad(r.distance_per_size, 0.1, 25)) return "Camera back is 0.1 to 25 × size."
      if (bad(r.distance_min, 1, 500)) return "The shortest distance is 1 to 500."
      if (bad(r.height_per_size, 0, 10)) return "Camera up is 0 to 10 × size (it can't go below the boss's feet)."
      if (bad(r.crop_zoom, 1, 8)) return "The crop zoom is 1 to 8."
      if (bad(r.crop_center_y, 0.01, 0.99)) return "The crop centre is 0.01 to 0.99."
      if (r.hide_ui_value >= 0 && bad(r.hide_ui_value, 0, 254)) return "The screen mode is 0 to 254."
      return ""
    },
    canSaveRules() {
      return !this.busy && this.list.camera.rules_editable && this.rulesDirty() && !this.rulesProblem()
    },
    async saveRules() {
      this.busy  = true
      this.error = ""
      try {
        const r = await SpireApi.v1().put("zone-cards/bosses/portrait-rules", this.rules)
        this.rules = this.rulesFrom(r.data)
        this.rulesOriginal = JSON.stringify(this.rules)
        Object.assign(this.list.camera, this.rules)
        this.notification = "Saved the portrait rules. #reload rules in game picks them up."
      } catch (e) {
        this.error = this.message(e)
      } finally {
        this.busy = false
      }
    },
    cardCaption() {
      const as = this.editing.trigger === "spawn" ? "As it spawns" : "As it engages"
      return this.editing.camera_ms > 0 ? `${as}, with ${this.secs(this.editing.camera_ms)} of camera on it for players not fighting` : as
    },
    cameraSummary(b) {
      const m = MODES.find(x => x.value === b.camera_mode)
      return `${b.camera_style === "live" ? "A live shot" : "A portrait"}, ${this.secs(b.camera_ms)}, framing: ${m ? m.text : b.camera_mode}`
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
      this.fields().forEach(f => (cols[f] = typeof e[f] === "number" ? e[f] : q(e[f])))
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
        ;["radius", "emblem_icon", "display_ms", "shroud", "repeat_seconds", "camera_ms"].forEach(f => (body[f] = parseInt(body[f]) || 0))
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

.bc-rule-label {
  min-width: 92px;
}

.bc-range-sm {
  width: 110px;
  vertical-align: middle;
}

.bc-badge-cam {
  background: rgba(23, 162, 184, 0.35);
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
