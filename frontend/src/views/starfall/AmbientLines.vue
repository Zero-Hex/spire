<template>
  <div>
    <eq-window-simple title="Ambient Lines">
      <div>
        What NPCs say or do near players, now and then (idle chatter) or when a player walks up (greetings), written
        as lines here instead of quest scripts. A line goes to <b>everyone nearby</b> (the NPC's own say or emote,
        with a speech bubble for a say) or to <b>only the player it's for</b> (the same NPC-speech channel and
        wording, no speech bubble). Left on "by kind", idle chatter is overheard and greetings are private.
      </div>
      <div class="text-muted hint mt-1">
        A zone reads its lines once: after editing, use <code>#ambient reload</code> in that zone
        (<code>#ambient test</code> plays an idle line near you, <code>#ambient greet</code> lets NPCs greet you again).
      </div>
      <div v-if="list && !list.audience" class="text-warning hint mt-1">
        This server's <code>custom_ambient_line</code> has no audience column yet (Starfall custom migration 72): every
        line goes by its kind.
      </div>
      <info-error-banner class="mt-2" :notification="notification" :error="error"
                         @dismiss-error="error = ''" @dismiss-notification="notification = ''"/>

      <div v-if="list" class="d-flex align-items-center flex-wrap mt-2">
        <span class="mr-1">Zone</span>
        <b-form-select size="sm" v-model="zone" :options="zoneOptions()" style="max-width: 300px" class="mr-3" @change="load()"/>
        <span class="mr-1">As seen by</span>
        <b-form-input size="sm" v-model="sample.name" style="width: 110px" class="mr-1" title="The player's name, for {player}"/>
        <b-form-select size="sm" v-model.number="sample.race" :options="playerRaceOptions(false)" style="width: 120px" class="mr-1"/>
        <b-form-select size="sm" v-model.number="sample.cls" :options="playerClassOptions(false)" style="width: 140px" class="mr-1"/>
        <span class="mr-1">level</span>
        <b-form-input size="sm" type="number" min="1" max="255" v-model.number="sample.level" style="width: 70px" class="mr-2"/>
        <b-form-radio-group size="sm" v-model="sample.night" buttons button-variant="outline-secondary" class="mr-3"
                            :options="[{ value: false, text: 'Day' }, { value: true, text: 'Night' }]"/>
        <template v-if="list.faction">
          <span class="mr-1" title="The con the NPC has toward them (Spire can't work out a real player's: race, class and deity modifiers count)">standing</span>
          <b-form-select size="sm" v-model="sample.standing" :options="standingOptions(false)" style="width: 130px" class="mr-3 al-sample-standing"/>
        </template>
        <b-form-checkbox v-model="onlyFitting">Only lines that fit them</b-form-checkbox>
      </div>
    </eq-window-simple>

    <div class="row mt-3" v-if="list">
      <!-- the lines -->
      <div :class="editing ? 'col-lg-7' : 'col-lg-12'">
        <eq-window-simple :title="(zone ? zoneLabel(zone) : 'Every zone') + ': ' + list.lines.length + ' line(s)'">
          <div class="d-flex mb-2">
            <b-button size="sm" variant="outline-warning" class="mr-2" @click="newLine('idle')"><i class="fa fa-plus mr-1"></i> Idle line</b-button>
            <b-button size="sm" variant="outline-warning" class="mr-3" @click="newLine('approach')"><i class="fa fa-plus mr-1"></i> Greeting</b-button>
            <b-form-input size="sm" v-model="search" placeholder="Search the lines" style="max-width: 240px"/>
          </div>
          <div v-for="k in ['idle', 'approach']" :key="'k' + k" class="mb-3">
            <h6 class="mb-1">{{ k === 'idle' ? 'Idle chatter' : 'Greetings' }}
              <span class="text-muted hint">{{ k === 'idle' ? 'now and then, by an NPC near a player' : 'once, when a player walks up' }}</span>
            </h6>
            <div v-if="!shown(k).length" class="text-muted hint">None.</div>
            <div v-for="l in shown(k)" :key="'l' + l.id"
                 :class="'al-line' + (editing && editing.id === l.id ? ' al-selected' : '') + (fitsPlayer(l) ? '' : ' al-unfit') + (l.enabled ? '' : ' al-disabled')"
                 @click="edit(l)">
              <div v-for="(part, i) in rendered(l, speaker(l))" :key="'p' + l.id + '-' + i" class="al-chat">
                <span v-if="i" class="al-after">+{{ i * 3 }} s</span>{{ part }}
              </div>
              <div class="hint">
                <span :class="'al-badge al-aud-' + audienceOf(l)">{{ audienceOf(l) === 'player' ? 'only them' : 'everyone nearby' }}</span>
                <span v-if="!l.audience" class="text-muted mr-2">by kind</span>
                <span class="text-muted mr-2">{{ whoText(l) }} · {{ matches(l).length }} NPC{{ matches(l).length === 1 ? '' : 's' }} here</span>
                <span v-if="forText(l)" class="text-muted mr-2">for {{ forText(l) }}</span>
                <span v-if="l.time_of_day !== 'any'" class="text-muted mr-2">{{ l.time_of_day }} only</span>
                <span v-if="bandText(l)" class="al-badge al-band">{{ bandText(l) }}</span>
                <span v-if="rendered(l, '').length > 1" class="text-muted mr-2">scene of {{ rendered(l, '').length }}</span>
                <span class="text-muted mr-2">weight {{ l.weight }}</span>
                <span v-if="!l.zone_short_name && zone" class="al-badge">every zone</span>
                <span v-if="!l.enabled" class="al-badge al-badge-off">off</span>
                <span v-if="!fitsPlayer(l)" class="text-muted">· not for this player now</span>
              </div>
            </div>
          </div>
        </eq-window-simple>
      </div>

      <!-- the editor -->
      <div class="col-lg-5" v-if="editing">
        <eq-window-simple :title="(editing.id ? 'Line ' + editing.id : 'New line') + (dirty() ? ' (not saved)' : '')" class="al-editor">
          <div class="d-flex align-items-center flex-wrap mb-2">
            <span class="al-label">Zone</span>
            <b-form-select size="sm" v-model="editing.zone_short_name" :options="zoneOptions(true)" style="max-width: 260px"/>
          </div>

          <h6 class="al-h">Which NPCs</h6>
          <div class="d-flex align-items-center flex-wrap mb-1">
            <span class="al-label">Race</span>
            <b-form-select size="sm" v-model.number="editing.npc_race" :options="npcRaceOptions()" style="max-width: 200px" class="mr-2"/>
            <span class="al-label">Body</span>
            <b-form-select size="sm" v-model.number="editing.npc_bodytype" :options="bodytypeOptions()" style="max-width: 170px"/>
          </div>
          <div class="d-flex align-items-center flex-wrap mb-1">
            <span class="al-label">Name has</span>
            <b-form-input size="sm" v-model="editing.npc_name" maxlength="64" style="max-width: 170px" class="mr-2" placeholder="any"/>
            <span class="al-label">NPC id</span>
            <b-form-input size="sm" type="number" min="0" v-model.number="editing.npc_type_id" style="max-width: 110px" placeholder="any"/>
          </div>
          <div class="hint mb-2" :class="matches(editing).length ? 'text-muted' : 'text-warning'">
            <template v-if="!zone">Pick a zone to see which of its NPCs this fits.</template>
            <template v-else-if="!matches(editing).length">No NPC {{ zoneLabel(zone) }} spawns fits this.</template>
            <template v-else>Fits {{ matches(editing).length }} NPC{{ matches(editing).length === 1 ? '' : 's' }} here:
              {{ uniqueNames(matches(editing)).slice(0, 8).join(', ') }}<span v-if="uniqueNames(matches(editing)).length > 8">, …</span>
            </template>
          </div>

          <h6 class="al-h">What</h6>
          <div class="d-flex align-items-center flex-wrap mb-1">
            <b-form-radio-group size="sm" v-model="editing.kind" buttons button-variant="outline-secondary" class="mr-2"
                                :options="[{ value: 'idle', text: 'Idle' }, { value: 'approach', text: 'Greeting' }]"/>
            <b-form-radio-group size="sm" v-model="editing.mode" buttons button-variant="outline-secondary" class="mr-2"
                                :options="[{ value: 'say', text: 'Says' }, { value: 'emote', text: 'Emote' }]"/>
            <b-form-radio-group size="sm" v-model="editing.time_of_day" buttons button-variant="outline-secondary"
                                :options="[{ value: 'any', text: 'Any time' }, { value: 'day', text: 'Day' }, { value: 'night', text: 'Night' }]"/>
          </div>
          <div class="d-flex align-items-center flex-wrap mb-2">
            <span class="al-label">Who sees it</span>
            <b-form-radio-group size="sm" v-model="editing.audience" buttons button-variant="outline-secondary" :disabled="!list.audience"
                                :options="[{ value: '', text: 'By kind (' + (editing.kind === 'approach' ? 'only them' : 'everyone nearby') + ')' },
                                           { value: 'nearby', text: 'Everyone nearby' }, { value: 'player', text: 'Only them' }]"/>
          </div>
          <b-form-input v-model="editing.text" maxlength="255" :placeholder="editing.mode === 'emote' ? 'clicks its jaw softly as it shuffles past.' : 'Life is a candle. Why mourn the wax?'"/>
          <div class="d-flex justify-content-between hint mb-1">
            <span>
              <a v-for="f in ['{player}', '{race}', '{class}', '{npc}']" :key="f" href="#" class="mr-2" @click.prevent="insert(f)">{{ f }}</a>
            </span>
            <span :class="bytes(editing.text) > 255 ? 'text-danger' : 'text-muted'">{{ bytes(editing.text) }} / 255</span>
          </div>
          <div class="hint text-muted mb-1">
            Split with <code>|</code> for a scene: one part about every 3 s from the same NPC (nothing else starts for that player
            meanwhile; it stops if the NPC fights, dies or is left behind). A part starting with <code>*</code> is an emote.
          </div>
          <div v-if="textWarning()" class="hint text-warning mb-1">{{ textWarning() }}</div>

          <!-- how it reads -->
          <div class="al-preview mb-2">
            <div class="hint text-muted mb-1">
              {{ sample.name }} sees, in the NPC-speech channel<span v-if="uniqueNames(matches(editing)).length > 1">, from
              <select v-model="previewNpc" class="al-inline-select">
                <option value="">{{ speaker(editing) }}</option>
                <option v-for="n in uniqueNames(matches(editing)).slice(1, 200)" :key="'pn' + n" :value="n">{{ n }}</option>
              </select></span>:
            </div>
            <div v-if="!editing.text.trim()" class="al-chat">…</div>
            <div v-for="(part, i) in rendered(editing, previewSpeaker())" :key="'pp' + i" class="al-chat">
              <span v-if="i" class="al-after">+{{ i * 3 }} s</span>{{ part }}
            </div>
            <div class="hint text-muted mt-1">
              <template v-if="audienceOf(editing) === 'player'">Only {{ sample.name }}: no one else sees it, and no speech bubble over the NPC.</template>
              <template v-else>Everyone nearby sees the same line<span v-if="editing.mode === 'say'">, with a speech bubble over the NPC</span>.</template>
            </div>
          </div>

          <h6 class="al-h">For which players</h6>
          <div class="d-flex align-items-center flex-wrap mb-2">
            <b-form-select size="sm" v-model.number="editing.player_race" :options="playerRaceOptions(true)" style="max-width: 150px" class="mr-1"/>
            <b-form-select size="sm" v-model.number="editing.player_class" :options="playerClassOptions(true)" style="max-width: 160px" class="mr-2"/>
            <span class="al-label">levels</span>
            <b-form-input size="sm" type="number" min="0" max="255" v-model.number="editing.min_level" style="width: 64px" placeholder="any"/>
            <span class="mx-1">–</span>
            <b-form-input size="sm" type="number" min="0" max="255" v-model.number="editing.max_level" style="width: 64px" placeholder="any"/>
          </div>
          <fieldset :disabled="!list.faction" class="mb-2">
            <div class="d-flex align-items-center flex-wrap mb-1">
              <span class="al-label">Standing</span>
              <b-form-select size="sm" v-model="editing.faction_from" :options="standingOptions(true)" style="max-width: 140px"/>
              <span class="mx-1">to</span>
              <b-form-select size="sm" v-model="editing.faction_to" :options="standingOptions(true)" style="max-width: 140px" class="mr-2"/>
              <span class="hint text-muted">{{ bandText(editing) ? bandText(editing) : 'any standing' }}</span>
            </div>
            <div v-if="!list.faction" class="hint text-warning">
              This server's <code>custom_ambient_line</code> has no faction columns yet (Starfall custom migration 73).
            </div>
            <template v-else-if="matches(editing).length">
              <div class="hint text-muted mb-1">
                {{ editing.kind === 'approach' ? 'Greetings' : 'Idle lines' }} these NPCs have for {{ sample.name }}, by their standing
                (this line marked; click one to see the page as that standing):
              </div>
              <div class="al-standings">
                <div v-for="(st, i) in standings" :key="'st' + st" @click="sample.standing = st"
                     :class="'al-standing' + (inBand(editing, i + 1) ? ' al-standing-mine' : '') + (standingCount(i + 1) ? '' : ' al-standing-none') + (sample.standing === st ? ' al-standing-sample' : '')"
                     :title="standingCount(i + 1) + ' line(s) for ' + st">
                  <div class="al-standing-n">{{ standingCount(i + 1) || '–' }}</div>
                  <div class="al-standing-name">{{ st }}</div>
                </div>
              </div>
            </template>
          </fieldset>

          <h6 class="al-h">How often</h6>
          <div class="d-flex align-items-center flex-wrap mb-2">
            <span class="al-label">Weight</span>
            <b-form-input size="sm" type="number" min="1" v-model.number="editing.weight" style="width: 70px" class="mr-1"
                          title="How likely among the lines that fit"/>
            <span class="hint text-muted mr-3">{{ weightShare() }}</span>
            <span class="al-label">Cooldown</span>
            <b-form-input size="sm" type="number" min="0" v-model.number="editing.cooldown" style="width: 80px" class="mr-1" placeholder="120"/>
            <span class="hint text-muted">s before it's used again in the zone (0: 120)</span>
          </div>

          <div class="d-flex align-items-center flex-wrap mb-2">
            <b-form-checkbox v-model="editing.enabled" :value="1" :unchecked-value="0" class="mr-3">Enabled</b-form-checkbox>
            <b-form-input size="sm" v-model="editing.notes" maxlength="255" placeholder="Notes (players don't see them)"/>
          </div>

          <div class="d-flex align-items-center flex-wrap">
            <b-button size="sm" variant="success" class="mr-2" :disabled="!canSave()" @click="save()">
              <i :class="'fa ' + (busy ? 'fa-cog fa-spin' : 'fa-save') + ' mr-1'"></i> Save
            </b-button>
            <b-button v-if="editing.id" size="sm" variant="outline-secondary" class="mr-2" @click="duplicate()">Duplicate</b-button>
            <b-button size="sm" variant="outline-secondary" class="mr-2" @click="close()">Close</b-button>
            <b-button v-if="editing.id" size="sm" variant="outline-danger" class="ml-auto" :disabled="busy" @click="remove()">Delete</b-button>
          </div>
        </eq-window-simple>
      </div>
    </div>
  </div>
</template>

<script>
import EqWindowSimple                    from "@/components/eq-ui/EQWindowSimple";
import InfoErrorBanner                   from "@/components/InfoErrorBanner";
import {SpireApi}                        from "@/app/api/spire-api";
import {DB_PLAYER_RACES, DB_RACE_NAMES}  from "@/app/constants/eq-races-constants";
import {DB_PLAYER_CLASSES}               from "@/app/constants/eq-classes-constants";
import {BODYTYPES}                       from "@/app/constants/eq-bodytype-constants";

const ZONE_KEY = "ambient-lines:zone"

function stored(key, fallback) {
  try {
    return localStorage.getItem(key) || fallback
  } catch (e) {
    return fallback
  }
}

// faction cons, as plugins/ambient.pl reads them (common/faction.h: ally 1 .. scowls 9)
const STANDINGS = ["ally", "warmly", "kindly", "amiably", "indifferent", "apprehensive", "dubious", "threatening", "scowls"]
const STANDING_ALIASES = { amiable: "amiably", indifferently: "indifferent", apprehensively: "apprehensive", dubiously: "dubious",
  threateningly: "threatening", scowling: "scowls" }

function con(name) {
  const n = (name || "").trim().toLowerCase()
  return STANDINGS.indexOf(STANDING_ALIASES[n] || n) + 1
}

function blank(zone, kind) {
  return {
    id: 0, zone_short_name: zone, npc_type_id: 0, npc_race: 0, npc_bodytype: 0, npc_name: "", kind: kind, mode: "say",
    audience: "", time_of_day: "any", player_race: 0, player_class: 0, min_level: 0, max_level: 0, weight: 1, cooldown: 0,
    text: "", enabled: 1, notes: "", faction_from: "", faction_to: "",
  }
}

export default {
  name: "AmbientLines",
  components: { EqWindowSimple, InfoErrorBanner },
  data() {
    return {
      list: null,
      zone: this.$route.query.zone !== undefined ? this.$route.query.zone : stored(ZONE_KEY, "paineel"),
      sample: { name: "Tester", race: 3, cls: 11, level: 30, night: false, standing: "indifferent" },
      standings: STANDINGS,
      onlyFitting: false,
      search: "",
      editing: null,
      original: "",
      previewNpc: "",
      busy: false,
      error: "",
      notification: "",
    }
  },
  mounted() {
    this.load()
  },
  methods: {
    async load() {
      try {
        const r = await SpireApi.v1().get("ambient-lines", { params: { zone: this.zone } })
        this.list = r.data
        try {
          localStorage.setItem(ZONE_KEY, this.zone)
        } catch (e) {
        }
        if (this.$route.query.zone !== this.zone) {
          this.$router.replace({ query: { zone: this.zone } }).catch(() => {})
        }
        if (this.editing && this.editing.id) {
          const l = this.list.lines.find(x => x.id === this.editing.id)
          l ? this.edit(l) : this.close()
        }
      } catch (e) {
        this.error = this.message(e)
      }
    },
    message(e) {
      return (e.response && e.response.data && e.response.data.error) || e.message || String(e)
    },

    // --- names ---
    zoneLabel(z) {
      const zz = this.list.zones.find(x => x.short_name === z)
      return zz ? zz.long_name : z
    },
    zoneOptions(forLine) {
      const counts = this.list.counts
      const opts = this.list.zones
        .map(z => ({ value: z.short_name, text: `${z.long_name} (${z.short_name})` + (counts[z.short_name] ? ` · ${counts[z.short_name]}` : ""), n: counts[z.short_name] || 0 }))
        .sort((a, b) => (b.n > 0) - (a.n > 0) || a.text.localeCompare(b.text))
      return [{ value: "", text: forLine ? "Every zone" : `Lines for every zone` + (counts[""] ? ` · ${counts[""]}` : "") }].concat(opts)
    },
    raceName(id) {
      return DB_RACE_NAMES[id] || `race ${id}`
    },
    className(id) {
      return DB_PLAYER_CLASSES[id] || `class ${id}`
    },
    playerRaceOptions(any) {
      const o = Object.keys(DB_PLAYER_RACES).map(k => ({ value: parseInt(k), text: DB_PLAYER_RACES[k].race }))
      return any ? [{ value: 0, text: "Any race" }].concat(o) : o
    },
    playerClassOptions(any) {
      const o = Object.keys(DB_PLAYER_CLASSES).map(k => ({ value: parseInt(k), text: DB_PLAYER_CLASSES[k] }))
      return any ? [{ value: 0, text: "Any class" }].concat(o) : o
    },
    npcRaceOptions() {
      // the zone's NPCs' races first
      const here = [...new Set(this.list.npcs.map(n => n.race))].sort((a, b) => this.raceName(a).localeCompare(this.raceName(b)))
      const opts = [{ value: 0, text: "Any race" }]
      here.forEach(r => opts.push({ value: r, text: `${this.raceName(r)} (${r})` }))
      if (this.editing && this.editing.npc_race && !here.includes(this.editing.npc_race)) {
        opts.push({ value: this.editing.npc_race, text: `${this.raceName(this.editing.npc_race)} (${this.editing.npc_race}, none here)` })
      }
      const rest = Object.keys(DB_RACE_NAMES).map(k => parseInt(k)).filter(r => r > 0 && !here.includes(r) && r !== (this.editing && this.editing.npc_race))
      return opts.concat([{ label: "Other races", options: rest.map(r => ({ value: r, text: `${this.raceName(r)} (${r})` })) }])
    },
    bodytypeOptions() {
      return [{ value: 0, text: "Any body" }].concat(Object.keys(BODYTYPES).filter(k => k !== "0").map(k => ({ value: parseInt(k), text: `${BODYTYPES[k]} (${k})` })))
    },

    // --- the plugin's rules (plugins/ambient.pl _amb_fits, _amb_lines, _amb_speak) ---
    audienceOf(l) {
      return l.audience === "player" || l.audience === "nearby" ? l.audience : (l.kind === "approach" ? "player" : "nearby")
    },
    matches(l) {
      const name = (l.npc_name || "").trim().toLowerCase()
      return this.list.npcs.filter(n => !n.silent &&
        (!l.npc_type_id || l.npc_type_id === n.id) && (!l.npc_race || l.npc_race === n.race) &&
        (!l.npc_bodytype || l.npc_bodytype === n.bodytype) && (!name || n.clean.toLowerCase().includes(name)))
    },
    uniqueNames(npcs) {
      return [...new Set(npcs.map(n => n.clean))]
    },
    fitsPlayer(l, standing) {
      const s = this.sample
      return (!l.player_race || l.player_race === s.race) && (!l.player_class || l.player_class === s.cls) &&
        (!l.min_level || s.level >= l.min_level) && (!l.max_level || s.level <= l.max_level) &&
        !(l.time_of_day === "night" && !s.night) && !(l.time_of_day === "day" && s.night) &&
        this.inBand(l, standing || con(s.standing))
    },
    // the line's band, as the plugin reads it: either order, one name alone that standing only
    band(l) {
      let f1 = con(l.faction_from), f2 = con(l.faction_to)
      ;[f1, f2] = [f1 || f2, f2 || f1]
      return f1 <= f2 ? [f1, f2] : [f2, f1]
    },
    inBand(l, c) {
      if (!this.list.faction) {
        return true
      }
      const [lo, hi] = this.band(l)
      return !lo || (c >= lo && c <= hi)
    },
    bandText(l) {
      const [lo, hi] = this.band(l)
      if (!lo) {
        return ""
      }
      return lo === hi ? `${STANDINGS[lo - 1]} only` : `${STANDINGS[lo - 1]} – ${STANDINGS[hi - 1]}`
    },
    standingOptions(any) {
      const o = STANDINGS.map(s => ({ value: s, text: s }))
      return any ? [{ value: "", text: "any" }].concat(o) : o
    },
    // the enabled lines of this kind for the same NPCs and player, at standing c
    standingCount(c) {
      const e = this.editing
      const mine = new Set(this.matches(e).map(n => n.id))
      const others = this.list.lines.filter(l => l.id !== e.id && l.enabled && l.kind === e.kind && this.fitsPlayer(l, c) &&
        this.matches(l).some(n => mine.has(n.id)))
      return others.length + (e.enabled && this.fitsPlayer(e, c) ? 1 : 0)
    },
    speaker(l) {
      const m = this.matches(l)
      return m.length ? m[0].clean : (l.npc_name ? this.titleCase(l.npc_name) : "The NPC")
    },
    previewSpeaker() {
      const names = this.uniqueNames(this.matches(this.editing))
      return names.includes(this.previewNpc) ? this.previewNpc : this.speaker(this.editing)
    },
    titleCase(s) {
      return s.trim().replace(/^./, c => c.toUpperCase())
    },
    // the lines a player reads: the parts split by |, a part starting with * an emote (plugins/ambient.pl _amb_speak)
    parts(l) {
      return (l.text || "").split("|").map(p => p.trim()).filter(p => p.length)
        .map(p => (/^\*/.test(p) ? { emote: true, text: p.replace(/^\*\s*/, "") } : { emote: l.mode === "emote", text: p }))
        .filter(p => p.text.length)
    },
    rendered(l, npc) {
      const fill = { player: this.sample.name, race: this.raceName(this.sample.race), class: this.className(this.sample.cls), npc: npc }
      return this.parts(l).map(p => {
        const text = p.text.replace(/\{(player|race|class|npc)\}/gi, (m, k) => fill[k.toLowerCase()])
        return p.emote ? `${npc} ${text}` : `${npc} says, '${text}'`
      })
    },
    whoText(l) {
      const parts = []
      if (l.npc_type_id) {
        parts.push(`NPC ${l.npc_type_id}`)
      }
      if (l.npc_race) {
        parts.push(this.raceName(l.npc_race))
      }
      if (l.npc_bodytype) {
        parts.push(BODYTYPES[l.npc_bodytype] || `body ${l.npc_bodytype}`)
      }
      if (l.npc_name) {
        parts.push(`"${l.npc_name}"`)
      }
      return parts.length ? parts.join(" · ") : "any NPC"
    },
    forText(l) {
      const parts = []
      if (l.player_race) {
        parts.push(this.raceName(l.player_race))
      }
      if (l.player_class) {
        parts.push(this.className(l.player_class))
      }
      if (l.min_level || l.max_level) {
        parts.push(`level ${l.min_level || 1}–${l.max_level || "any"}`)
      }
      return parts.join(" ")
    },
    shown(kind) {
      const q = this.search.trim().toLowerCase()
      return this.list.lines.filter(l => l.kind === kind && (!this.onlyFitting || this.fitsPlayer(l)) &&
        (!q || (l.text + " " + l.notes + " " + l.npc_name).toLowerCase().includes(q)))
    },

    // --- editing ---
    edit(l) {
      if (this.editing && this.dirty() && !confirm("Leave your changes to this line?")) {
        return
      }
      this.editing  = { ...l, audience: l.audience || "" }
      this.original = JSON.stringify(this.editing)
      this.previewNpc = ""
    },
    newLine(kind) {
      this.edit(blank(this.zone, kind))
      this.original = ""
    },
    duplicate() {
      const copy = { ...this.editing, id: 0, notes: this.editing.notes }
      this.editing  = copy
      this.original = ""
    },
    close() {
      if (this.editing && this.dirty() && !confirm("Leave your changes to this line?")) {
        return
      }
      this.editing = null
    },
    dirty() {
      return !!this.editing && JSON.stringify(this.editing) !== this.original
    },
    bytes(s) {
      return new TextEncoder().encode((s || "").trim()).length
    },
    canSave() {
      // a boolean: Vue 2 keeps disabled="0" as disabled
      return !this.busy && !!this.valid() && (!this.editing.id || this.dirty())
    },
    valid() {
      const e = this.editing
      return e.text.trim() && this.parts(e).length && this.bytes(e.text) <= 255 && !this.unknownFill() &&
        !(e.min_level && e.max_level && e.min_level > e.max_level)
    },
    unknownFill() {
      return (this.editing.text.match(/\{[^}]*\}/g) || []).find(b => !/^\{(player|race|class|npc)\}$/i.test(b))
    },
    textWarning() {
      const e = this.editing
      const t = e.text.trim()
      if (this.unknownFill()) {
        return `${this.unknownFill()} isn't filled in: use {player}, {race}, {class} or {npc}.`
      }
      if (t && !this.parts(e).length) {
        return "Every part is empty."
      }
      const names = this.uniqueNames(this.matches(e)).map(n => n.toLowerCase())
      for (const p of this.parts(e)) {
        const pt = p.text.toLowerCase()
        if (p.emote && (names.some(n => pt.startsWith(n)) || /^\{npc\}/.test(pt))) {
          return "An emote gets the NPC's name put in front: write \"clicks its jaw\", not \"a skeleton clicks its jaw\"."
        }
        if (!p.emote && /^["']|["']$/.test(p.text)) {
          return "A say is quoted for you: leave the quotes out."
        }
      }
      return ""
    },
    insert(f) {
      this.editing.text = (this.editing.text || "") + f
    },
    weightShare() {
      // among the enabled lines of its kind that fit the same NPCs and this player
      const e = this.editing
      const mine = new Set(this.matches(e).map(n => n.id))
      if (!mine.size) {
        return ""
      }
      const rivals = this.list.lines.filter(l => l.id !== e.id && l.enabled && l.kind === e.kind && this.fitsPlayer(l) &&
        this.matches(l).some(n => mine.has(n.id)))
      const total = rivals.reduce((a, l) => a + (l.weight || 1), 0) + (e.weight || 1)
      return `about ${Math.round((e.weight || 1) / total * 100)}% of the ${e.kind === "approach" ? "greetings" : "idle lines"} its NPCs have for this player`
    },
    async save() {
      this.busy = true
      this.error = ""
      try {
        const e = this.editing
        const r = await SpireApi.v1().put("ambient-lines", {
          ...e, npc_type_id: e.npc_type_id || 0, min_level: e.min_level || 0, max_level: e.max_level || 0,
          weight: e.weight || 1, cooldown: e.cooldown || 0,
        })
        const saved = r.data
        this.notification = `Saved line ${saved.id}. In ${saved.zone_short_name ? this.zoneLabel(saved.zone_short_name) : "each zone"}, #ambient reload picks it up.`
        this.editing = null
        await this.load()
        const l = this.list.lines.find(x => x.id === saved.id)
        if (l) {
          this.edit(l)
        }
      } catch (e) {
        this.error = this.message(e)
      } finally {
        this.busy = false
      }
    },
    async remove() {
      if (!confirm(`Delete line ${this.editing.id}: "${this.editing.text}"?`)) {
        return
      }
      this.busy = true
      try {
        await SpireApi.v1().delete(`ambient-lines/${this.editing.id}`)
        this.notification = `Deleted line ${this.editing.id}.`
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

.al-line {
  padding: 6px 10px;
  border-radius: 4px;
  cursor: pointer;
  border: 1px solid transparent;
}

.al-line:hover {
  background: rgba(255, 255, 255, 0.05);
}

.al-selected {
  border-color: rgba(212, 175, 55, 0.6);
  background: rgba(212, 175, 55, 0.1);
}

.al-unfit .al-chat {
  opacity: 0.45;
}

.al-disabled .al-chat {
  text-decoration: line-through;
  opacity: 0.4;
}

/* the chat window's NPC speech */
.al-chat {
  font-family: Arial, "Helvetica Neue", sans-serif;
  color: #d9e9f5;
  background: rgba(0, 0, 0, 0.45);
  padding: 3px 8px;
  border-radius: 3px;
}

.al-badge {
  font-size: 0.85em;
  padding: 0 6px;
  border-radius: 3px;
  margin-right: 6px;
  background: rgba(255, 255, 255, 0.12);
}

.al-aud-player {
  background: rgba(23, 162, 184, 0.4);
}

.al-aud-nearby {
  background: rgba(212, 175, 55, 0.3);
}

.al-badge-off {
  background: rgba(220, 53, 69, 0.4);
}

.al-after {
  color: rgba(255, 255, 255, 0.35);
  font-size: 0.8em;
  margin-right: 6px;
}

.al-chat + .al-chat {
  margin-top: 2px;
}

.al-band {
  background: rgba(111, 66, 193, 0.45);
}

.al-standings {
  display: flex;
  gap: 3px;
}

.al-standing {
  flex: 1 1 0;
  text-align: center;
  padding: 3px 0;
  border-radius: 3px;
  background: rgba(255, 255, 255, 0.06);
  cursor: pointer;
  border: 1px solid transparent;
}

.al-standing-mine {
  background: rgba(111, 66, 193, 0.45);
}

.al-standing-none .al-standing-n {
  color: #e88;
}

.al-standing-sample {
  border-color: rgba(212, 175, 55, 0.8);
}

.al-standing-n {
  font-weight: bold;
}

.al-standing-name {
  font-size: 0.7em;
  color: #bbb;
}

.al-editor {
  position: sticky;
  top: 10px;
}

.al-h {
  margin: 10px 0 6px;
  color: #d4af37;
  font-size: 0.9em;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.al-label {
  margin-right: 6px;
  color: #bbb;
}

.al-preview {
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 4px;
  padding: 8px;
}

.al-inline-select {
  width: auto;
  max-width: 220px;
  display: inline-block;
  padding: 0 4px;
  font-size: 1em;
  background: #1c2028;
  color: #ddd;
  border: 1px solid rgba(255, 255, 255, 0.2);
  border-radius: 3px;
}
</style>
