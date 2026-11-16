<template>
  <div>
    <eq-window-simple title="Zone Cards">
      <div>
        The title card players see as they arrive in a zone, drawn by the Starfall DLL. Every zone has one with its long
        name; here you can give a zone a different title, a subtitle line, its own font, colours, emblem, reveal,
        time on screen and sound, or hide its card. A player's first entry shows the full card; return visits show the
        name small near the top.
      </div>
      <div class="text-muted hint mt-1">
        A zone reads its card when it boots: after saving, use <code>#zonecard reload</code> in that zone (then
        <code>#zonecard first</code> to see the full card), or wait for it to restart.
      </div>
      <div v-if="list && !list.styles" class="text-warning hint mt-1">
        This server's <code>custom_zone_card</code> has no style columns yet (Starfall custom migration 69 adds them when
        world starts on a build that has it): until then only the title, subtitle and "shown" can be saved.
      </div>
      <info-error-banner class="mt-2" :notification="notification" :error="error"
                         @dismiss-error="error = ''" @dismiss-notification="notification = ''"/>
    </eq-window-simple>

    <div class="row mt-3" v-if="list">
      <!-- the zones -->
      <div class="col-lg-4">
        <eq-window-simple :title="'Zones (' + shown().length + ')'">
          <b-form-input size="sm" v-model="search" placeholder="Search zones" class="mb-2"/>
          <b-form-radio-group size="sm" v-model="filter" buttons button-variant="outline-secondary" class="mb-2"
                              :options="filterOptions()"/>
          <div class="zc-list">
            <table class="eq-table eq-highlight-rows zc-zones">
              <tbody>
              <tr v-for="z in shown()" :key="z.short_name" @click="pick(z)"
                  :class="'zc-clickable' + (selected && selected.short_name === z.short_name ? ' zc-selected' : '')">
                <td>
                  <div :style="listTitleStyle(z)">{{ cardTitle(z) }}</div>
                  <div v-if="cardOf(z) && cardOf(z).subtitle" class="hint zc-sub">{{ cardOf(z).subtitle }}</div>
                </td>
                <td class="text-muted hint text-right" style="white-space: nowrap">
                  {{ z.short_name }}
                  <div>
                    <span v-if="cardOf(z) && cardOf(z).sound" class="mr-1" title="Has a sound"><i class="fa fa-volume-up"></i></span>
                    <span v-if="cardOf(z) && !cardOf(z).enabled" class="zc-badge zc-badge-hidden">hidden</span>
                    <span v-else-if="cardOf(z)" class="zc-badge">custom</span>
                  </div>
                </td>
              </tr>
              </tbody>
            </table>
          </div>
        </eq-window-simple>
      </div>

      <!-- the card -->
      <div class="col-lg-8">
        <eq-window-simple v-if="!selected" title="Card">
          <div class="text-muted">Pick a zone.</div>
        </eq-window-simple>
        <eq-window-simple v-else :title="selected.long_name + ' (' + selected.short_name + ')' + (dirty() ? ' (not saved)' : '')">
          <!-- the preview -->
          <div ref="scene" class="zc-scene" :class="form.enabled ? '' : 'zc-scene-off'">
            <div v-if="form.enabled && shroudPct() > 0" class="zc-shroud" :class="'zc-phase-' + play.phase" :style="shroudStyle()"></div>
            <div class="zc-card" :class="'zc-phase-' + play.phase + ' zc-reveal-' + revealName()" :style="cardStyle()">
              <div v-if="form.emblem_icon > 0" class="zc-emblem" :style="emblemStyle()">
                <span :class="'item-' + form.emblem_icon" class="zc-emblem-icon" :style="emblemIconStyle()"></span>
              </div>
              <div class="zc-rule"></div>
              <div class="zc-title" :style="titleStyle()">{{ shownTitle() }}<span class="zc-unwritten">{{ unwrittenTitle() }}</span></div>
              <div class="zc-rule"></div>
              <div v-if="form.subtitle.trim()" class="zc-subtitle" :style="subtitleStyle()">{{ form.subtitle.trim() }}</div>
            </div>
            <div v-if="!form.enabled" class="zc-off">Hidden: players see no card here</div>
            <div v-if="play.phase !== 'idle'" class="zc-phase">{{ phaseLabel() }}</div>
          </div>
          <div class="d-flex justify-content-between align-items-center flex-wrap hint text-muted mt-1">
            <span>
              First entry, roughly as on a
              <b-form-select size="sm" v-model="screen" :options="screens.map(s => ({ value: s.w, text: s.w + '×' + s.h }))"
                             class="zc-screen"/>
              screen.
            </span>
            <span>
              <b-button size="sm" variant="outline-warning" class="mr-2" @click="playCard()" :disabled="!form.enabled">
                <i :class="'fa ' + (play.phase === 'idle' ? 'fa-play' : 'fa-repeat') + ' mr-1'"></i> Play it
              </b-button>
              {{ timingText() }}
            </span>
          </div>
          <div class="zc-return-scene mt-2 mb-3">
            <span class="hint text-muted mr-2">Return visit:</span>
            <span v-if="!form.return_card" class="hint text-warning">none for this zone</span>
            <span v-else class="zc-return" :style="returnStyle()">{{ form.title.trim() || selected.long_name }}</span>
            <span v-if="form.return_card && form.sound && form.sound_on_return" class="hint text-muted ml-2">
              <i class="fa fa-volume-up"></i> with its sound
            </span>
          </div>

          <!-- the text -->
          <div class="row">
            <div class="col-md-6 form-group mb-2">
              <label class="mb-1">Title <span class="text-muted hint">(empty: the zone's long name)</span></label>
              <b-form-input v-model="form.title" :placeholder="selected.long_name" maxlength="63"/>
              <div class="hint d-flex justify-content-between">
                <span class="text-warning">{{ textWarning(form.title, true) }}</span>
                <span :class="bytes(form.title) > 63 ? 'text-danger' : 'text-muted'">{{ bytes(form.title) }} / 63</span>
              </div>
            </div>
            <div class="col-md-6 form-group mb-2">
              <label class="mb-1">Subtitle <span class="text-muted hint">(empty: none)</span></label>
              <b-form-input v-model="form.subtitle" placeholder="One line, a few words, in the zone's voice" maxlength="127"/>
              <div class="hint d-flex justify-content-between">
                <span class="text-warning">{{ textWarning(form.subtitle) }}</span>
                <span :class="bytes(form.subtitle) > 127 ? 'text-danger' : 'text-muted'">{{ bytes(form.subtitle) }} / 127</span>
              </div>
            </div>
          </div>

          <!-- the look -->
          <fieldset :disabled="!list.styles" class="zc-style">
            <div class="d-flex align-items-center flex-wrap mb-2">
              <span class="zc-label">Font</span>
              <b-form-radio-group size="sm" v-model="form.title_font" buttons button-variant="outline-secondary" class="mr-3">
                <b-form-radio v-for="f in fonts" :key="f.value" :value="f.value">
                  <span :style="{ fontFamily: f.family, fontWeight: f.weight }">{{ f.text }}</span>
                </b-form-radio>
              </b-form-radio-group>
              <span class="zc-label">Reveal</span>
              <b-form-radio-group size="sm" v-model="form.reveal" buttons button-variant="outline-secondary"
                                  :options="reveals"/>
            </div>
            <div class="d-flex align-items-center flex-wrap mb-2">
              <template v-for="c in colors">
                <span :key="'cl' + c.key" class="zc-label">{{ c.label }}</span>
                <span :key="'cc' + c.key" class="zc-color mr-3">
                  <input type="color" :value="'#' + (form[c.key] || c.def)" @input="ev => setColor(c.key, ev.target.value)">
                  <b-form-input size="sm" :value="form[c.key]" :placeholder="c.def" maxlength="7" class="zc-hex"
                                @change="v => setColor(c.key, v)"/>
                  <a v-if="form[c.key]" href="#" class="hint ml-1" @click.prevent="form[c.key] = ''" :title="'Back to ' + c.name">default</a>
                </span>
              </template>
            </div>
            <div class="d-flex align-items-center flex-wrap mb-2">
              <span class="zc-label">Emblem</span>
              <span v-if="form.emblem_icon > 0" :class="'item-' + form.emblem_icon + '-sm'" class="zc-icon-sm mr-1"></span>
              <b-form-input size="sm" type="number" min="0" v-model.number="form.emblem_icon" class="zc-num mr-1"
                            placeholder="icon" title="An item icon id, drawn above the title (0: none)"/>
              <b-button size="sm" variant="outline-secondary" class="mr-1" @click="picking = true">Pick</b-button>
              <b-button v-if="form.emblem_icon > 0" size="sm" variant="outline-secondary" class="mr-3" @click="form.emblem_icon = 0">None</b-button>
              <span class="zc-label ml-2">Stays up</span>
              <b-form-input size="sm" type="number" min="0" max="15" step="0.5" :value="displaySeconds()" class="zc-num mr-1"
                            @change="v => setDisplay(v)" placeholder="4.5"/>
              <span class="hint text-muted">seconds (empty: 4.5, at most 15)</span>
            </div>
            <div class="d-flex align-items-center flex-wrap mb-2">
              <span class="zc-label">Sound</span>
              <b-form-input size="sm" v-model="form.sound" list="zc-sounds" placeholder="an MP3 in the client folder" maxlength="64"
                            style="max-width: 300px" class="mr-1"/>
              <datalist id="zc-sounds">
                <option v-for="s in list.sounds" :key="s" :value="s"/>
              </datalist>
              <b-button size="sm" variant="outline-secondary" class="mr-2" :disabled="!soundHeld()" @click="playSound()"
                        :title="soundHeld() ? 'Hear it' : 'Spire keeps no copy of it to play'">
                <i class="fa fa-volume-up"></i>
              </b-button>
              <b-form-checkbox v-model="form.sound_on_return" :disabled="!form.sound.trim()" class="mr-3">also on return visits</b-form-checkbox>
              <b-form-checkbox v-model="form.return_card">Small card on return visits</b-form-checkbox>
            </div>
            <div v-if="form.sound.trim() && !soundHeld()" class="hint text-warning mb-2">
              Spire's client patch has no {{ form.sound.trim() }}: players need it in their client folder already, or upload
              it on <router-link :to="clientPatchRoute">Client Patch</router-link> so the patcher sends it.
            </div>
          </fieldset>
          <fieldset :disabled="!list.shroud" class="zc-style zc-shroud-controls">
            <div class="d-flex align-items-center flex-wrap mb-1">
              <span class="zc-label">Shroud</span>
              <input type="range" min="0" max="90" step="1" v-model.number="form.shroud" class="zc-range mr-2"
                     title="How far the scene dims behind the first-entry card">
              <b-form-input size="sm" type="number" min="0" max="90" v-model.number="form.shroud" class="zc-num zc-num-sm mr-1"/>
              <span class="mr-3">% <span class="hint text-muted">{{ form.shroud ? "" : "(none)" }}</span></span>
              <span class="zc-label">Tint</span>
              <span class="zc-color mr-2">
                <input type="color" :value="'#' + (form.shroud_color || '000000')" @input="ev => setColor('shroud_color', ev.target.value)">
                <b-form-input size="sm" :value="form.shroud_color" placeholder="black" maxlength="7" class="zc-hex"
                              @change="v => setColor('shroud_color', v)"/>
                <a v-if="form.shroud_color" href="#" class="hint ml-1" @click.prevent="form.shroud_color = ''" title="Back to black">default</a>
              </span>
            </div>
            <div class="hint text-muted mb-2">
              The scene darkens behind a first entry's card (evenly, and more at the edges) and clears as the card fades;
              return visits aren't shrouded.
              <span v-if="!cardOf(selected)">This zone has no row, so it dims {{ list.default_shroud }}% (<code>Custom:Zone_Card_Default_Shroud</code>);
                saving a row keeps that unless you change it.</span>
            </div>
            <div v-if="!list.shroud" class="hint text-warning mb-2">
              This server's <code>custom_zone_card</code> has no shroud columns yet (Starfall custom migration 70).
            </div>
          </fieldset>

          <b-form-checkbox v-model="form.enabled" class="mb-3">Show this zone's card</b-form-checkbox>

          <div class="d-flex align-items-center flex-wrap">
            <b-button size="sm" variant="success" class="mr-2" :disabled="!dirty() || busy || !valid()" @click="save()">
              <i :class="'fa ' + (busy ? 'fa-cog fa-spin' : 'fa-save') + ' mr-1'"></i> Save
            </b-button>
            <b-button size="sm" variant="outline-secondary" class="mr-2" :disabled="!dirty() || busy" @click="pick(selected, true)">Undo</b-button>
            <b-button v-if="cardOf(selected)" size="sm" variant="outline-danger" class="mr-3" :disabled="busy" @click="reset()"
                      title="Delete the row: the card shows the long name, no subtitle, the default look">
              Back to the default
            </b-button>
            <span class="text-muted hint">
              {{ discovered(selected) }} character{{ discovered(selected) === 1 ? " has" : "s have" }} entered this zone
              (and had its first-entry card)
            </span>
          </div>

          <div class="mt-3 hint text-muted">
            As SQL, for a migration or another server:
            <pre class="zc-sql">{{ sql() }}</pre>
          </div>
        </eq-window-simple>
      </div>
    </div>

    <b-modal v-model="picking" size="xl" title="Emblem: an item icon" hide-footer>
      <item-icon-selector v-if="picking" :selected-icon="form.emblem_icon || 0" @input="id => { form.emblem_icon = id; picking = false }"/>
    </b-modal>
  </div>
</template>

<script>
import "@fontsource/cinzel/latin-600.css";
import "@fontsource/pirata-one/latin-400.css";
import "@fontsource/arimo/latin-400.css";
import EqWindowSimple   from "@/components/eq-ui/EQWindowSimple";
import InfoErrorBanner  from "@/components/InfoErrorBanner";
import ItemIconSelector from "@/views/items/components/ItemIconSelector";
import {SpireApi}       from "@/app/api/spire-api";
import {ROUTE}          from "@/routes";

// as the DLL draws it (starfall-3 docs/design/zone_cards.md)
const DEFAULT_COLORS = { title_color: "F6F0E2", subtitle_color: "CDC4B0", accent_color: "DEBA5C" }
const FAMILIES = {
  "": { family: "Cinzel", weight: 600 },
  pirata: { family: "'Pirata One'", weight: 400 },
  arimo: { family: "Arimo", weight: 400 },
}
const BEAT_MS = 800, IN_MS = 1200, OUT_MS = 1600, DEFAULT_DISPLAY_MS = 4500

// a row's shroud when it doesn't set one (the column default)
const ROW_SHROUD = 45

// a new row's form: a zone with no row dims by Custom:Zone_Card_Default_Shroud,
// so its first row starts there
function emptyForm(shroud) {
  return {
    title: "", subtitle: "", enabled: true, title_font: "", title_color: "", subtitle_color: "", accent_color: "",
    emblem_icon: 0, reveal: "", display_ms: 0, return_card: true, sound: "", sound_on_return: false,
    shroud: shroud === undefined ? ROW_SHROUD : shroud, shroud_color: "",
  }
}

export default {
  name: "ZoneCards",
  components: { EqWindowSimple, InfoErrorBanner, ItemIconSelector },
  data() {
    return {
      list: null,
      search: "",
      filter: "all",
      selected: null,
      form: emptyForm(),
      screen: 1920,
      screens: [{ w: 1280, h: 720 }, { w: 1920, h: 1080 }, { w: 2560, h: 1440 }],
      fonts: [
        { value: "", text: "Cinzel", family: "Cinzel", weight: 600 },
        { value: "pirata", text: "Pirata", family: "'Pirata One'", weight: 400 },
        { value: "arimo", text: "Arimo", family: "Arimo", weight: 400 },
      ],
      reveals: [{ value: "", text: "Fade" }, { value: "rise", text: "Rise" }, { value: "letters", text: "Letters" }],
      colors: [
        { key: "title_color", label: "Title", def: DEFAULT_COLORS.title_color, name: "ivory" },
        { key: "subtitle_color", label: "Subtitle", def: DEFAULT_COLORS.subtitle_color, name: "parchment" },
        { key: "accent_color", label: "Lines", def: DEFAULT_COLORS.accent_color, name: "gold" },
      ],
      play: { phase: "idle", letters: -1, timers: [] },
      audio: null,
      sceneW: 0, // the preview's width, for the icon sprite's scale
      picking: false,
      busy: false,
      error: "",
      notification: "",
      clientPatchRoute: ROUTE.STARFALL_CLIENT_PATCH,
    }
  },
  async mounted() {
    await this.load()
    const zone = this.$route.query.zone
    const z = zone && this.list && this.list.zones.find(x => x.short_name === zone)
    if (z) {
      this.pick(z)
    }
  },
  updated() {
    const el = this.$refs.scene
    if (el && el !== this.observed) {
      if (this.resizer) {
        this.resizer.disconnect()
      }
      this.observed = el
      this.resizer  = new ResizeObserver(() => (this.sceneW = el.clientWidth))
      this.resizer.observe(el)
      this.sceneW = el.clientWidth
    }
  },
  beforeDestroy() {
    this.stopPlay()
    if (this.resizer) {
      this.resizer.disconnect()
    }
  },
  methods: {
    async load() {
      try {
        const r = await SpireApi.v1().get("zone-cards")
        this.list = r.data
      } catch (e) {
        this.error = this.message(e)
      }
    },
    message(e) {
      return (e.response && e.response.data && e.response.data.error) || e.message || String(e)
    },
    cardOf(z) {
      return this.list.cards.find(c => c.zone_short_name === z.short_name)
    },
    cardTitle(z) {
      const c = this.cardOf(z)
      return (c && c.title) || z.long_name
    },
    listTitleStyle(z) {
      const c = this.cardOf(z)
      const f = FAMILIES[(c && c.title_font) || ""] || FAMILIES[""]
      return { fontFamily: f.family, fontWeight: f.weight }
    },
    discovered(z) {
      return this.list.discovered[z.short_name] || 0
    },
    filterOptions() {
      const cards = this.list.cards
      return [
        { value: "all", text: `All ${this.list.zones.length}` },
        { value: "custom", text: `Custom ${cards.length}` },
        { value: "hidden", text: `Hidden ${cards.filter(c => !c.enabled).length}` },
      ]
    },
    shown() {
      const q = this.search.trim().toLowerCase()
      return this.list.zones.filter(z => {
        const c = this.cardOf(z)
        if (this.filter === "custom" && !c) {
          return false
        }
        if (this.filter === "hidden" && !(c && !c.enabled)) {
          return false
        }
        return !q || z.short_name.includes(q) || z.long_name.toLowerCase().includes(q) ||
          (c && (c.title + " " + c.subtitle).toLowerCase().includes(q))
      })
    },
    // the form for a card row (or the defaults)
    formOf(c) {
      if (!c) {
        return emptyForm(this.list && this.list.shroud ? this.list.default_shroud : ROW_SHROUD)
      }
      return {
        title: c.title || "", subtitle: c.subtitle || "", enabled: !!c.enabled, title_font: c.title_font === "cinzel" ? "" : (c.title_font || ""),
        title_color: c.title_color || "", subtitle_color: c.subtitle_color || "", accent_color: c.accent_color || "",
        emblem_icon: c.emblem_icon || 0, reveal: c.reveal === "fade" ? "" : (c.reveal || ""), display_ms: c.display_ms || 0,
        return_card: c.return_card === undefined ? true : !!c.return_card, sound: c.sound || "", sound_on_return: !!c.sound_on_return,
        shroud: c.shroud === undefined || c.shroud === null ? ROW_SHROUD : c.shroud, shroud_color: c.shroud_color || "",
      }
    },
    pick(z, force) {
      if (!force && this.selected && this.selected.short_name !== z.short_name && this.dirty() && !confirm("Leave your changes to this card?")) {
        return
      }
      this.stopPlay()
      this.selected = z
      this.form = this.formOf(this.cardOf(z))
      if (this.$route.query.zone !== z.short_name) {
        this.$router.replace({ query: { zone: z.short_name } }).catch(() => {})
      }
    },
    normal(f) {
      return JSON.stringify({
        ...f, title: f.title.trim(), subtitle: f.subtitle.trim(), sound: f.sound.trim(), emblem_icon: parseInt(f.emblem_icon) || 0,
        display_ms: parseInt(f.display_ms) || 0, title_color: f.title_color.toUpperCase(), subtitle_color: f.subtitle_color.toUpperCase(),
        accent_color: f.accent_color.toUpperCase(), sound_on_return: !!f.sound_on_return && !!f.sound.trim(),
        shroud: parseInt(f.shroud) || 0, shroud_color: f.shroud_color.toUpperCase(),
      })
    },
    dirty() {
      return !!this.selected && this.normal(this.form) !== this.normal(this.formOf(this.cardOf(this.selected)))
    },
    bytes(s) {
      return new TextEncoder().encode(s.trim()).length
    },
    valid() {
      const hex = c => !c || /^[0-9A-Fa-f]{6}$/.test(c)
      return this.bytes(this.form.title) <= 63 && this.bytes(this.form.subtitle) <= 127 &&
        hex(this.form.title_color) && hex(this.form.subtitle_color) && hex(this.form.accent_color) && hex(this.form.shroud_color) &&
        this.shroudPct() >= 0 && this.shroudPct() <= 90 &&
        (!this.form.sound.trim() || /\.mp3$/i.test(this.form.sound.trim()))
    },
    textWarning(s, title) {
      if (/[^\x20-\x7e]/.test(s)) {
        return "Letters outside plain English text may not draw in the DLL's fonts" + (title && this.form.reveal === "letters" ? " or write out letter by letter." : ".")
      }
      return ""
    },
    setColor(key, v) {
      this.form[key] = String(v || "").trim().replace(/^#/, "").toUpperCase()
    },
    displaySeconds() {
      return this.form.display_ms ? this.form.display_ms / 1000 : ""
    },
    setDisplay(v) {
      const s = parseFloat(v)
      this.form.display_ms = isNaN(s) || s <= 0 ? 0 : Math.min(15000, Math.round(s * 1000))
    },
    soundHeld() {
      const s = this.form.sound.trim().replace(/\\/g, "/").toLowerCase()
      return !!s && this.list.sounds.some(x => x.toLowerCase() === s)
    },
    async playSound() {
      try {
        if (this.audio) {
          this.audio.pause()
        }
        const r = await SpireApi.v1().get("zone-cards/sound", { params: { path: this.form.sound.trim() }, responseType: "blob" })
        this.audio = new Audio(URL.createObjectURL(r.data))
        await this.audio.play()
      } catch (e) {
        this.error = "Can't play " + this.form.sound + ": " + this.message(e)
      }
    },

    // --- the preview, sized as the DLL sizes it: the title 8.5% of the screen
    // height (48-120px), the subtitle 3.4%, the return card's name 4.5% ---
    screenSize() {
      return this.screens.find(s => s.w === this.screen) || this.screens[1]
    },
    cqw(px) {
      return `${px / this.screen * 100}cqw`
    },
    titlePx() {
      return Math.min(120, Math.max(48, this.screenSize().h * 0.085))
    },
    revealName() {
      return this.form.reveal || "fade"
    },
    family() {
      return FAMILIES[this.form.title_font] || FAMILIES[""]
    },
    color(key) {
      return "#" + (this.form[key] || DEFAULT_COLORS[key])
    },
    shroudPct() {
      const n = parseInt(this.form.shroud)
      return isNaN(n) ? 0 : n
    },
    // the shroud: the scene darkened by shroud% at the card, more toward the edges
    shroudStyle() {
      const hex = this.form.shroud_color || "000000"
      const rgb = [0, 2, 4].map(i => parseInt(hex.substr(i, 2), 16) || 0).join(", ")
      const a = Math.min(90, Math.max(0, this.shroudPct())) / 100
      const edge = Math.min(0.95, a + (1 - a) * 0.45)
      return {
        background: `radial-gradient(ellipse 70% 60% at 50% 50%, rgba(${rgb}, ${a}) 0%, rgba(${rgb}, ${a}) 35%, rgba(${rgb}, ${edge}) 100%)`,
        "--zc-in": IN_MS + "ms", "--zc-out": OUT_MS + "ms",
      }
    },
    cardStyle() {
      return { "--zc-accent": this.color("accent_color"), "--zc-in": IN_MS + "ms", "--zc-out": OUT_MS + "ms" }
    },
    titleStyle() {
      const f = this.family()
      return { fontFamily: f.family, fontWeight: f.weight, fontSize: this.cqw(this.titlePx()), color: this.color("title_color") }
    },
    subtitleStyle() {
      return { fontSize: this.cqw(this.screenSize().h * 0.034), color: this.color("subtitle_color") }
    },
    emblemStyle() {
      const size = this.titlePx() * 1.1
      return { width: this.cqw(size), height: this.cqw(size) }
    },
    emblemIconStyle() {
      // the item icon sprites are 40px: scaled to the emblem's size
      return { transform: `scale(${this.titlePx() * 1.1 / 40 * (this.sceneW || 600) / this.screen})` }
    },
    returnStyle() {
      const f = this.family()
      return { fontFamily: f.family, fontWeight: f.weight, color: this.color("title_color") }
    },
    shownTitle() {
      const t = this.form.title.trim() || this.selected.long_name
      return this.play.letters >= 0 ? t.slice(0, this.play.letters) : t
    },
    unwrittenTitle() {
      // the rest, kept invisible so the card doesn't shift as it writes out
      const t = this.form.title.trim() || this.selected.long_name
      return this.play.letters >= 0 ? t.slice(this.play.letters) : ""
    },
    displayMs() {
      return this.form.display_ms || DEFAULT_DISPLAY_MS
    },
    timingText() {
      const s = ms => (ms / 1000).toFixed(1).replace(/\.0$/, "") + " s"
      return `${s(BEAT_MS)} beat, ${s(IN_MS)} in, ${s(this.displayMs())} up, ${s(OUT_MS)} out`
    },
    phaseLabel() {
      return { beat: "…", in: "in", hold: "up", out: "out" }[this.play.phase] || ""
    },
    stopPlay() {
      this.play.timers.forEach(t => clearTimeout(t))
      this.play.timers = []
      this.play.phase = "idle"
      this.play.letters = -1
      if (this.audio) {
        this.audio.pause()
      }
    },
    after(ms, fn) {
      this.play.timers.push(setTimeout(fn, ms))
    },
    playCard() {
      this.stopPlay()
      this.play.phase = "beat"
      const title = this.form.title.trim() || this.selected.long_name
      const letters = this.form.reveal === "letters"
      // letters: the title writes itself out over the reveal, then the subtitle
      const inMs = letters ? IN_MS + 600 : IN_MS
      this.after(BEAT_MS, () => {
        this.play.phase = "in"
        if (this.form.sound.trim() && this.soundHeld()) {
          this.playSound()
        }
        if (letters) {
          this.play.letters = 0
          const step = IN_MS / Math.max(title.length, 1)
          for (let i = 1; i <= title.length; i++) {
            this.after(step * i, () => (this.play.letters = i))
          }
        }
      })
      this.after(BEAT_MS + inMs, () => (this.play.phase = "hold"))
      this.after(BEAT_MS + inMs + this.displayMs(), () => (this.play.phase = "out"))
      this.after(BEAT_MS + inMs + this.displayMs() + OUT_MS, () => {
        this.play.phase = "idle"
        this.play.letters = -1
      })
    },

    sql() {
      const f = this.form
      const q = s => "'" + String(s).trim().replace(/\\/g, "\\\\").replace(/'/g, "''") + "'"
      if (!this.list.styles) {
        return `INSERT INTO custom_zone_card (zone_short_name, title, subtitle, enabled)\n` +
          `VALUES (${q(this.selected.short_name)}, ${q(f.title)}, ${q(f.subtitle)}, ${f.enabled ? 1 : 0})\n` +
          `ON DUPLICATE KEY UPDATE title = VALUES(title), subtitle = VALUES(subtitle), enabled = VALUES(enabled);`
      }
      const cols = {
        zone_short_name: q(this.selected.short_name), title: q(f.title), subtitle: q(f.subtitle), title_font: q(f.title_font),
        title_color: q(f.title_color), subtitle_color: q(f.subtitle_color), accent_color: q(f.accent_color),
        emblem_icon: parseInt(f.emblem_icon) || 0, reveal: q(f.reveal), display_ms: parseInt(f.display_ms) || 0,
        return_card: f.return_card ? 1 : 0, sound: q(f.sound), sound_on_return: f.sound_on_return && f.sound.trim() ? 1 : 0,
        enabled: f.enabled ? 1 : 0,
      }
      if (this.list.shroud) {
        cols.shroud = this.shroudPct()
        cols.shroud_color = q(f.shroud_color)
      }
      const names = Object.keys(cols)
      return `INSERT INTO custom_zone_card (${names.join(", ")})\nVALUES (${names.map(n => cols[n]).join(", ")})\n` +
        `ON DUPLICATE KEY UPDATE ${names.slice(1).map(n => `${n} = VALUES(${n})`).join(", ")};`
    },
    async save() {
      const f = this.form
      this.busy = true
      this.error = ""
      try {
        const r = await SpireApi.v1().put(`zone-cards/${this.selected.short_name}`, {
          title: f.title, subtitle: f.subtitle, enabled: f.enabled ? 1 : 0, title_font: f.title_font,
          title_color: f.title_color, subtitle_color: f.subtitle_color, accent_color: f.accent_color,
          emblem_icon: parseInt(f.emblem_icon) || 0, reveal: f.reveal, display_ms: parseInt(f.display_ms) || 0,
          return_card: f.return_card ? 1 : 0, sound: f.sound, sound_on_return: f.sound_on_return && f.sound.trim() ? 1 : 0,
          shroud: this.shroudPct(), shroud_color: f.shroud_color,
        })
        this.list.cards = this.list.cards.filter(c => c.zone_short_name !== r.data.zone_short_name).concat([r.data])
        this.form = this.formOf(r.data)
        this.notification = `Saved ${this.selected.long_name}'s card. In that zone, #zonecard reload shows it.`
      } catch (e) {
        this.error = this.message(e)
      } finally {
        this.busy = false
      }
    },
    async reset() {
      if (!confirm(`Put ${this.selected.long_name}'s card back to its long name, with no subtitle and the default look?`)) {
        return
      }
      this.busy = true
      this.error = ""
      try {
        await SpireApi.v1().delete(`zone-cards/${this.selected.short_name}`)
        this.list.cards = this.list.cards.filter(c => c.zone_short_name !== this.selected.short_name)
        this.form = this.formOf(null)
        this.notification = `${this.selected.long_name}'s card is back to the default.`
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

.zc-list {
  max-height: 72vh;
  overflow-y: auto;
}

.zc-clickable {
  cursor: pointer;
}

.zc-selected > td {
  background: rgba(212, 175, 55, 0.18);
}

.zc-sub {
  color: #c9b98a;
  font-style: italic;
}

.zc-badge {
  font-size: 0.8em;
  padding: 0 6px;
  border-radius: 3px;
  background: rgba(212, 175, 55, 0.3);
  color: #f1e3b0;
}

.zc-badge-hidden {
  background: rgba(220, 53, 69, 0.35);
}

/* the scene: a screen, sized in cqw so the card keeps the DLL's proportions */
.zc-scene {
  container-type: inline-size;
  position: relative;
  aspect-ratio: 16 / 9;
  /* a lit stand-in for the zone, so the shroud has something to dim */
  background:
    radial-gradient(ellipse 30% 22% at 72% 24%, rgba(255, 244, 214, 0.55), transparent 70%),
    radial-gradient(ellipse 45% 18% at 25% 62%, rgba(70, 92, 52, 0.7), transparent 75%),
    linear-gradient(180deg, #6f88a3 0%, #a7b6c1 40%, #8e9a7a 41%, #5f6e47 65%, #3c472f 100%);
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
}

.zc-shroud {
  position: absolute;
  inset: 0;
  pointer-events: none;
}

.zc-shroud.zc-phase-beat {
  opacity: 0;
}

.zc-shroud.zc-phase-in {
  animation: zc-fade var(--zc-in) ease-out both;
}

.zc-shroud.zc-phase-out {
  animation: zc-fade var(--zc-out) ease-in reverse both;
}

.zc-card {
  position: relative;
}

.zc-range {
  width: 160px;
  vertical-align: middle;
}

.zc-num-sm {
  width: 64px;
}

.zc-shroud-controls {
  border-top: none;
  padding-top: 0;
}

.zc-scene-off .zc-card {
  opacity: 0.25;
}

.zc-card {
  text-align: center;
  max-width: 92%;
}

.zc-emblem {
  margin: 0 auto 0.6cqw;
  display: flex;
  align-items: center;
  justify-content: center;
}

.zc-emblem-icon {
  display: inline-block;
  transform-origin: center;
}

.zc-title {
  line-height: 1.2;
  letter-spacing: 0.02em;
  text-shadow: 0 0 0.25em rgba(0, 0, 0, 0.9);
  padding: 0.12em 0;
}

.zc-unwritten {
  visibility: hidden;
}

.zc-rule {
  height: 1px;
  margin: 0 auto;
  width: 30cqw;
  background: linear-gradient(90deg, transparent 0%, var(--zc-accent) 30%, var(--zc-accent) 70%, transparent 100%);
}

.zc-subtitle {
  font-family: Arimo, Arial, sans-serif;
  margin-top: 0.6em;
  text-shadow: 0 0 0.3em rgba(0, 0, 0, 0.9);
}

/* playing: beat (nothing), in (the reveal), hold, out (fade) */
.zc-phase-beat {
  opacity: 0;
}

.zc-phase-beat .zc-rule {
  transform: scaleX(0);
}

.zc-phase-in .zc-rule {
  animation: zc-draw var(--zc-in) ease-out both;
}

.zc-phase-in.zc-reveal-fade .zc-title, .zc-phase-in.zc-reveal-fade .zc-subtitle, .zc-phase-in .zc-emblem {
  animation: zc-fade var(--zc-in) ease-out both;
}

.zc-phase-in.zc-reveal-rise .zc-title, .zc-phase-in.zc-reveal-rise .zc-subtitle {
  animation: zc-rise var(--zc-in) ease-out both;
}

.zc-phase-in.zc-reveal-letters .zc-subtitle {
  animation: zc-fade 600ms ease-out var(--zc-in) both;
}

.zc-phase-out {
  animation: zc-fade var(--zc-out) ease-in reverse both;
}

@keyframes zc-fade {
  from { opacity: 0; }
  to { opacity: 1; }
}

@keyframes zc-rise {
  from { opacity: 0; transform: translateY(0.6em); }
  to { opacity: 1; transform: translateY(0); }
}

@keyframes zc-draw {
  from { transform: scaleX(0); }
  to { transform: scaleX(1); }
}

.zc-off {
  position: absolute;
  bottom: 8px;
  right: 12px;
  font-size: 0.85em;
  color: #e88;
}

.zc-phase {
  position: absolute;
  top: 6px;
  left: 10px;
  font-size: 0.8em;
  color: rgba(255, 255, 255, 0.4);
}

.zc-return-scene {
  background: #1c2028;
  border-radius: 3px;
  padding: 4px 10px;
  text-align: center;
}

.zc-return {
  font-size: 1.25em;
  text-shadow: 0 0 0.3em rgba(0, 0, 0, 0.9);
}

.zc-screen {
  width: auto;
  display: inline-block;
  height: auto;
  padding: 0 4px;
  font-size: 1em;
}

.zc-style {
  border-top: 1px solid rgba(255, 255, 255, 0.1);
  padding-top: 10px;
}

.zc-style[disabled] {
  opacity: 0.5;
}

.zc-label {
  margin-right: 6px;
  color: #bbb;
}

.zc-color {
  display: inline-flex;
  align-items: center;
}

.zc-color input[type=color] {
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

.zc-hex {
  width: 84px;
}

.zc-num {
  width: 80px;
}

.zc-icon-sm {
  display: inline-block;
  vertical-align: middle;
}

.zc-sql {
  background: rgba(0, 0, 0, 0.35);
  color: #ddd;
  padding: 6px 8px;
  white-space: pre-wrap;
  margin: 4px 0 0;
}
</style>
