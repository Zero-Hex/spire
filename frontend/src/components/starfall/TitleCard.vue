<template>
  <div>
    <div ref="scene" class="zc-scene" :class="shown ? '' : 'zc-scene-off'" @click="sceneClick">
      <div v-if="shown && shroudPct() > 0" class="zc-shroud" :class="'zc-phase-' + play.phase" :style="shroudStyle()"></div>
      <div class="zc-card" :class="'zc-phase-' + play.phase + ' zc-reveal-' + revealName()" :style="cardStyle()">
        <div v-if="card.emblem_icon > 0" class="zc-emblem" :style="emblemStyle()">
          <span :class="'item-' + card.emblem_icon" class="zc-emblem-icon" :style="emblemIconStyle()"></span>
        </div>
        <div class="zc-rule"></div>
        <div class="zc-title" :style="titleStyle()">{{ shownTitle() }}<span class="zc-unwritten">{{ unwrittenTitle() }}</span></div>
        <div class="zc-rule"></div>
        <div v-if="(card.subtitle || '').trim()" class="zc-subtitle" :style="subtitleStyle()">{{ card.subtitle.trim() }}</div>
      </div>
      <div v-if="!shown" class="zc-off">{{ hiddenText }}</div>
      <div v-if="play.phase !== 'idle'" class="zc-phase">{{ phaseLabel() }}</div>
      <div v-if="waiting()" class="zc-waiting">
        {{ play.minDone ? "Up until the player acts: press a key or click" : "Holding (at least " + secs(timing.minMs) + "): Escape or a click skips it" }}
      </div>
    </div>
    <div class="d-flex justify-content-between align-items-center flex-wrap hint text-muted mt-1">
      <span>
        {{ caption }}, roughly as on a
        <b-form-select size="sm" v-model="screen" :options="screens.map(s => ({ value: s.w, text: s.w + '×' + s.h }))"
                       class="zc-screen"/>
        screen.
      </span>
      <span>
        <b-button size="sm" variant="outline-warning" class="mr-2" @click.stop="playCard()" :disabled="!shown">
          <i :class="'fa ' + (play.phase === 'idle' ? 'fa-play' : 'fa-repeat') + ' mr-1'"></i> Play it
        </b-button>
        {{ timingText() }}
      </span>
    </div>
  </div>
</template>

<script>
import "@fontsource/cinzel/latin-600.css";
import "@fontsource/pirata-one/latin-400.css";
import "@fontsource/arimo/latin-400.css";

// A Starfall title card (zone cards, boss cards) as the DLL draws it
// (starfall-3 docs/design/zone_cards.md): the title 8.5% of the screen height
// (48-120px), the subtitle 3.4%, between two rules, over a shroud; played
// through its timing.
export const DEFAULT_COLORS = { title_color: "F6F0E2", subtitle_color: "CDC4B0", accent_color: "DEBA5C" }
export const FAMILIES = {
  "": { family: "Cinzel", weight: 600 },
  cinzel: { family: "Cinzel", weight: 600 },
  pirata: { family: "'Pirata One'", weight: 400 },
  arimo: { family: "Arimo", weight: 400 },
}
export const FONTS = [
  { value: "", text: "Cinzel", family: "Cinzel", weight: 600 },
  { value: "pirata", text: "Pirata", family: "'Pirata One'", weight: 400 },
  { value: "arimo", text: "Arimo", family: "Arimo", weight: 400 },
]
const BEAT_MS = 800, IN_MS = 1200

export default {
  name: "TitleCard",
  props: {
    // title (as shown), subtitle, title_font, title_color, subtitle_color,
    // accent_color, emblem_icon, reveal ('' fade), shroud, shroud_color
    card: { type: Object, required: true },
    shown: { type: Boolean, default: true },
    hiddenText: { type: String, default: "Hidden" },
    caption: { type: String, default: "The card" },
    // displayMs: time up; untilInput: up until the player acts, after minMs,
    // at most maxMs (0 none); outMs: the fade out
    timing: { type: Object, default: () => ({ displayMs: 4500, untilInput: false, minMs: 0, maxMs: 0, outMs: 1600 }) },
  },
  data() {
    return {
      screen: 1920,
      screens: [{ w: 1280, h: 720 }, { w: 1920, h: 1080 }, { w: 2560, h: 1440 }],
      play: { phase: "idle", letters: -1, timers: [], minDone: false },
      sceneW: 0,
    }
  },
  mounted() {
    this.resizer = new ResizeObserver(() => (this.sceneW = this.$refs.scene ? this.$refs.scene.clientWidth : 0))
    this.resizer.observe(this.$refs.scene)
    this.sceneW = this.$refs.scene.clientWidth
    window.addEventListener("keydown", this.key)
  },
  beforeDestroy() {
    this.stopPlay()
    this.resizer.disconnect()
    window.removeEventListener("keydown", this.key)
  },
  watch: {
    // a different card: stop playing the old one
    "card.title"() {
      this.stopPlay()
    },
  },
  methods: {
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
      return this.card.reveal || "fade"
    },
    family() {
      return FAMILIES[this.card.title_font || ""] || FAMILIES[""]
    },
    color(key) {
      return "#" + (this.card[key] || DEFAULT_COLORS[key])
    },
    shroudPct() {
      const n = parseInt(this.card.shroud)
      return isNaN(n) ? 0 : n
    },
    outMs() {
      return this.timing.outMs || 1600
    },
    // the shroud: the scene darkened by shroud% at the card, more toward the edges
    shroudStyle() {
      const hex = this.card.shroud_color || "000000"
      const rgb = [0, 2, 4].map(i => parseInt(hex.substr(i, 2), 16) || 0).join(", ")
      const a = Math.min(90, Math.max(0, this.shroudPct())) / 100
      const edge = Math.min(0.95, a + (1 - a) * 0.45)
      return {
        background: `radial-gradient(ellipse 70% 60% at 50% 50%, rgba(${rgb}, ${a}) 0%, rgba(${rgb}, ${a}) 35%, rgba(${rgb}, ${edge}) 100%)`,
        "--zc-in": IN_MS + "ms", "--zc-out": this.outMs() + "ms",
      }
    },
    cardStyle() {
      return { "--zc-accent": this.color("accent_color"), "--zc-in": IN_MS + "ms", "--zc-out": this.outMs() + "ms" }
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
    shownTitle() {
      const t = this.card.title || ""
      return this.play.letters >= 0 ? t.slice(0, this.play.letters) : t
    },
    unwrittenTitle() {
      // the rest, kept invisible so the card doesn't shift as it writes out
      return this.play.letters >= 0 ? (this.card.title || "").slice(this.play.letters) : ""
    },
    secs(ms) {
      return (ms / 1000).toFixed(1).replace(/\.0$/, "") + " s"
    },
    timingText() {
      const t = this.timing
      const up = t.untilInput
        ? `up until the player acts (at least ${this.secs(t.minMs)}${t.maxMs ? ", at most " + this.secs(t.maxMs) : ""})`
        : `${this.secs(t.displayMs)} up`
      return `${this.secs(BEAT_MS)} beat, ${this.secs(IN_MS)} in, ${up}, ${this.secs(this.outMs())} out`
    },
    phaseLabel() {
      return { beat: "…", in: "in", hold: "up", out: "out" }[this.play.phase] || ""
    },
    waiting() {
      return this.timing.untilInput && (this.play.phase === "in" || this.play.phase === "hold")
    },
    stopPlay() {
      this.play.timers.forEach(t => clearTimeout(t))
      this.play.timers = []
      this.play.phase = "idle"
      this.play.letters = -1
      this.play.minDone = false
      this.$emit("stop")
    },
    after(ms, fn) {
      this.play.timers.push(setTimeout(fn, ms))
    },
    fadeOut() {
      if (this.play.phase !== "in" && this.play.phase !== "hold") {
        return
      }
      this.play.timers.forEach(t => clearTimeout(t))
      this.play.timers = []
      this.play.letters = -1
      this.play.phase = "out"
      this.after(this.outMs(), () => (this.play.phase = "idle"))
    },
    // until input: once the minimum is over any key ends it; Escape, any time
    key(ev) {
      if (!this.waiting() || /^(INPUT|TEXTAREA|SELECT)$/.test((ev.target && ev.target.tagName) || "")) {
        return
      }
      if (ev.key === "Escape" || this.play.minDone) {
        this.fadeOut()
      }
    },
    // a click skips it at once
    sceneClick() {
      if (this.waiting()) {
        this.fadeOut()
      }
    },
    playCard() {
      this.stopPlay()
      this.play.phase = "beat"
      const title = this.card.title || ""
      const letters = this.card.reveal === "letters"
      // letters: the title writes itself out over the reveal, then the subtitle
      const inMs = letters ? IN_MS + 600 : IN_MS
      this.after(BEAT_MS, () => {
        this.play.phase = "in"
        this.$emit("appear")
        if (letters) {
          this.play.letters = 0
          const step = IN_MS / Math.max(title.length, 1)
          for (let i = 1; i <= title.length; i++) {
            this.after(step * i, () => (this.play.letters = i))
          }
        }
        if (this.timing.untilInput) {
          this.after(this.timing.minMs, () => (this.play.minDone = true))
          if (this.timing.maxMs) {
            this.after(this.timing.maxMs, () => this.fadeOut())
          }
        }
      })
      this.after(BEAT_MS + inMs, () => {
        this.play.phase = "hold"
        this.play.letters = -1
      })
      if (!this.timing.untilInput) {
        this.after(BEAT_MS + inMs + this.timing.displayMs, () => (this.play.phase = "out"))
        this.after(BEAT_MS + inMs + this.timing.displayMs + this.outMs(), () => (this.play.phase = "idle"))
      }
    },
  },
}
</script>

<style scoped>
.hint {
  font-size: 0.85em;
}

.zc-waiting {
  position: absolute;
  bottom: 8px;
  left: 0;
  right: 0;
  text-align: center;
  font-size: 0.8em;
  color: rgba(255, 255, 255, 0.7);
  text-shadow: 0 0 4px #000;
}

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

.zc-screen {
  width: auto;
  display: inline-block;
  height: auto;
  padding: 0 4px;
  font-size: 1em;
}

</style>
