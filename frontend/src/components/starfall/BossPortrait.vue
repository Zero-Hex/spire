<template>
  <div>
    <div ref="scene" class="bp-scene" @click="play">
      <div class="bp-window" :class="'bp-' + phase + (textOnly ? ' bp-text-only' : '')" :style="windowStyle()">
        <div v-if="!textOnly" class="bp-pic" :style="{ borderColor: rgba(accent(), 0.5) }">
          <!-- the captured frame, cropped: 1/zoom of it, centred across, at cropCenterY down -->
          <div ref="capture" class="bp-capture" :style="captureStyle()">
            <span v-if="npc && npc.race" :class="'race-models-ctn-' + raceImage()" class="bp-model" :style="modelStyle()"></span>
          </div>
        </div>
        <div class="bp-text" :style="{ paddingTop: textOnly ? '0' : '0.4cqw' }">
          <div class="bp-title" :style="titleStyle()">{{ title }}</div>
          <div v-if="subtitle" class="bp-subtitle" :style="subtitleStyle()">{{ subtitle }}</div>
        </div>
      </div>
      <div class="bp-whiteout" :class="'bp-' + phase"></div>
      <div v-if="phase !== 'held'" class="bp-phase">{{ phaseLabel() }}</div>
    </div>
    <div class="d-flex justify-content-between align-items-center flex-wrap hint text-muted mt-1">
      <span>{{ caption }}</span>
      <span>
        <b-button size="sm" variant="outline-warning" class="mr-2" @click.stop="play">
          <i :class="'fa ' + (phase === 'held' ? 'fa-play' : 'fa-repeat') + ' mr-1'"></i> Play it
        </b-button>
        {{ textOnly ? "" : "flash, then " }}{{ (ms / 1000).toFixed(1).replace(/\.0$/, "") }} s
      </span>
    </div>
  </div>
</template>

<script>
import "@fontsource/cinzel/latin-600.css";
import "@fontsource/pirata-one/latin-400.css";
import "@fontsource/arimo/latin-400.css";
import {Npcs} from "@/app/npcs";
import {DEFAULT_COLORS, FAMILIES} from "@/components/starfall/TitleCard";

// A boss portrait as the Starfall DLL draws it (camera_probe.cpp DrawPortrait):
// a warm-white flash while the camera takes one frame from in front of the
// boss, then a window (players can move and resize it; first shown at the
// middle, 40% down, 34% of the screen wide) bordered in the card's accent
// colour, holding a crop of that frame (1/zoom of it, around the boss) and the
// card's title and epithet in its font and colours. Players who turned
// portraits off get the same window, text only. Spire has no frame to show:
// the boss's model picture stands in for it, placed as the portrait camera
// sees it -- the whole boss, below the middle, at a scale set by the distance.
const FLASH_OUT_MS = 350, IN_MS = 450, OUT_MS = 700
// in a frame shot at size x 2 (the default distance), the boss is about this
// much of the frame's height, centred this far down it
const BOSS_FRAC = 0.36, BOSS_Y = 0.70

export default {
  name: "BossPortrait",
  props: {
    title: { type: String, default: "" },
    subtitle: { type: String, default: "" },
    card: { type: Object, default: () => ({}) }, // title_font, title_color, subtitle_color, accent_color
    npc: { type: Object, default: null }, // race, gender, texture, helmtexture, size
    ms: { type: Number, default: 3000 },
    textOnly: { type: Boolean, default: false }, // the player's Boss portraits option is off
    // the portrait framing (camera_mode auto): the boss scaled by the camera's
    // distance, and the crop; other framings show the frame as their camera
    // mode takes it, uncropped by Spire's guess
    framing: { type: Boolean, default: true },
    rules: { type: Object, default: () => ({}) }, // distance_per_size, distance_min, crop_zoom, crop_center_y
    caption: { type: String, default: "The portrait" },
  },
  data() {
    return { phase: "held", timers: [], captureH: 0, observer: null }
  },
  mounted() {
    this.measure()
    if (window.ResizeObserver) {
      this.observer = new ResizeObserver(() => this.measure())
      this.observer.observe(this.$refs.scene)
    }
  },
  updated() {
    this.$nextTick(() => this.measure())
  },
  beforeDestroy() {
    this.clear()
    if (this.observer) {
      this.observer.disconnect()
    }
  },
  methods: {
    measure() {
      const h = this.$refs.capture ? this.$refs.capture.offsetHeight : 0
      if (h !== this.captureH) {
        this.captureH = h
      }
    },
    raceImage() {
      return Npcs.getRaceImage(this.npc)
    },
    color(key) {
      return "#" + ((this.card && this.card[key]) || DEFAULT_COLORS[key])
    },
    accent() {
      return this.color("accent_color")
    },
    rgba(hex, a) {
      const n = parseInt(hex.slice(1), 16)
      return `rgba(${(n >> 16) & 255}, ${(n >> 8) & 255}, ${n & 255}, ${a})`
    },
    zoom() {
      const z = parseFloat(this.rules.crop_zoom)
      return this.framing && z >= 1 && z <= 8 ? z : 1
    },
    centerY() {
      const y = parseFloat(this.rules.crop_center_y)
      return this.framing && y > 0 && y < 1 ? y : 0.5
    },
    windowStyle() {
      return { borderColor: this.accent() }
    },
    // the capture, scaled up by the zoom and shifted so the crop fills the picture
    captureStyle() {
      const z = this.zoom(), frac = 1 / z
      const v0 = Math.min(Math.max(this.centerY() - frac / 2, 0), 1 - frac)
      return { width: z * 100 + "%", height: z * 100 + "%", left: -(z - 1) / 2 * 100 + "%", top: -v0 * z * 100 + "%" }
    },
    // the boss: its height in the frame goes with size / distance
    // (boss_cards.cpp PortraitDistance; size 0 reads as 6)
    modelStyle() {
      let frac = BOSS_FRAC, y = BOSS_Y
      if (this.framing) {
        const size = this.npc && this.npc.size > 0 ? this.npc.size : 6
        const per = parseFloat(this.rules.distance_per_size) || 2, min = parseFloat(this.rules.distance_min) || 10
        const dist = Math.min(500, Math.max(min, size * per, 1))
        frac = Math.min(BOSS_FRAC * 2 * size / dist, 1.2)
      } else {
        y = 0.55
      }
      const scale = this.captureH ? this.captureH * frac / 100 : 0.9 // the sprite is 100px tall
      return { top: y * 100 + "%", transform: `translate(-50%, -50%) scale(${scale})` }
    },
    titleStyle() {
      const f = FAMILIES[(this.card && this.card.title_font) || ""] || FAMILIES[""]
      return { fontFamily: f.family, fontWeight: f.weight, color: this.color("title_color") }
    },
    subtitleStyle() {
      return { color: this.color("subtitle_color") }
    },
    clear() {
      this.timers.forEach(t => clearTimeout(t))
      this.timers = []
    },
    later(ms, f) {
      this.timers.push(setTimeout(f, ms))
    },
    play() {
      this.clear()
      this.$emit("stop")
      this.phase = this.textOnly ? "in" : "flash"
      const start = this.textOnly ? 0 : 120 // the capture takes a few frames behind the flash
      this.later(start, () => {
        this.phase = "in"
        this.$emit("appear")
      })
      this.later(start + IN_MS + this.ms, () => (this.phase = "out"))
      this.later(start + IN_MS + this.ms + OUT_MS, () => {
        this.phase = "gone"
        this.later(900, () => (this.phase = "held"))
      })
    },
    phaseLabel() {
      return {
        flash: "the flash: one frame from in front of the boss",
        in: this.textOnly ? "the card (portraits off)" : "the portrait",
        out: "fading", gone: "back to the game",
      }[this.phase]
    },
  },
}
</script>

<style scoped>
.bp-scene {
  position: relative;
  aspect-ratio: 16 / 9;
  overflow: hidden;
  border-radius: 4px;
  cursor: pointer;
  container-type: inline-size;
  background: radial-gradient(ellipse at 50% 70%, #3a4a3c 0%, #1d2620 55%, #0d1210 100%);
}

/* the window: 34% of the screen wide, centred 40% down */
.bp-window {
  position: absolute;
  left: 50%;
  top: 40%;
  width: 34cqw;
  transform: translate(-50%, -50%);
  padding: 0.36cqw;
  background: rgba(12, 10, 8, 0.88);
  border: 2px solid;
  border-radius: 3px;
  transition: opacity 0.45s;
}

.bp-text-only {
  min-height: 23cqw;
  display: flex;
  align-items: center;
  justify-content: center;
}

.bp-pic {
  position: relative;
  aspect-ratio: 16 / 9;
  overflow: hidden;
  border: 1px solid;
}

.bp-capture {
  position: absolute;
  background: radial-gradient(ellipse at 50% 72%, #6b5a3e 0%, #3b3a30 35%, #22241f 70%, #141414 100%);
}

.bp-model {
  position: absolute;
  left: 50%;
  display: inline-block;
  transform-origin: 50% 50%;
}

.bp-text {
  text-align: center;
}

.bp-title {
  font-size: 2.5cqw;
  line-height: 1.15;
  text-shadow: 2px 2px 0 rgba(0, 0, 0, 0.7);
}

.bp-subtitle {
  font-family: Arimo, Arial, sans-serif;
  font-size: 1.2cqw;
  margin-top: 0.1cqw;
  text-shadow: 1px 1px 0 rgba(0, 0, 0, 0.7);
}

/* the flash hides the camera's few frames away; it fades as the portrait comes in */
.bp-whiteout {
  position: absolute;
  inset: 0;
  background: #fff6e4;
  opacity: 0;
  pointer-events: none;
  transition: opacity 0.35s ease-out;
}

.bp-whiteout.bp-flash {
  opacity: 1;
  transition: none;
}

.bp-window.bp-flash, .bp-window.bp-out, .bp-window.bp-gone {
  opacity: 0;
}

.bp-window.bp-out {
  transition: opacity 0.7s;
}

.bp-phase {
  position: absolute;
  right: 8px;
  top: 6px;
  font-size: 0.75em;
  color: rgba(255, 255, 255, 0.7);
  background: rgba(0, 0, 0, 0.45);
  padding: 1px 6px;
  border-radius: 3px;
}

.hint {
  font-size: 0.85em;
}
</style>
