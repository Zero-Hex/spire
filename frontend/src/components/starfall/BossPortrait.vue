<template>
  <div>
    <div class="bp-scene" @click="play">
      <div class="bp-dim" :class="'bp-' + phase"></div>
      <div class="bp-frame" :class="'bp-' + phase">
        <div class="bp-shot">
          <span v-if="npc && npc.race" :class="'race-models-ctn-' + raceImage()" class="bp-model"></span>
          <template v-if="!uiHidden">
            <div class="bp-ui bp-ui-chat"></div>
            <div class="bp-ui bp-ui-bar"></div>
            <div class="bp-ui bp-ui-target"></div>
          </template>
        </div>
        <div class="bp-rule"></div>
        <div class="bp-title">{{ title }}</div>
        <div v-if="subtitle" class="bp-subtitle">{{ subtitle }}</div>
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
        flash, then {{ (ms / 1000).toFixed(1).replace(/\.0$/, "") }} s
      </span>
    </div>
  </div>
</template>

<script>
import "@fontsource/cinzel/latin-600.css";
import {Npcs} from "@/app/npcs";

// A boss portrait (starfall-3 docs/design/boss_cards.md, camera_style snap):
// a warm-white flash while the camera takes one frame from the boss's view,
// then the frame, framed, with the title and epithet beneath and a light dim
// behind. The portrait carries only the text (the server sends it no card
// style), so it's drawn in the card's default lettering. Spire has no frame
// to show: the boss's model picture stands in for it.
const FLASH_MS = 350, OUT_MS = 700

export default {
  name: "BossPortrait",
  props: {
    title: { type: String, default: "" },
    subtitle: { type: String, default: "" },
    npc: { type: Object, default: null }, // race, gender, texture, helmtexture
    ms: { type: Number, default: 3000 },
    uiHidden: { type: Boolean, default: false }, // Custom:Boss_Snap_Hide_Ui_Value set
    caption: { type: String, default: "The portrait" },
  },
  data() {
    return { phase: "held", timers: [] }
  },
  beforeDestroy() {
    this.clear()
  },
  methods: {
    raceImage() {
      return Npcs.getRaceImage(this.npc)
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
      this.phase = "flash"
      this.later(FLASH_MS, () => {
        this.phase = "in"
        this.$emit("appear")
      })
      this.later(FLASH_MS + this.ms, () => (this.phase = "out"))
      this.later(FLASH_MS + this.ms + OUT_MS, () => {
        this.phase = "gone"
        this.later(900, () => (this.phase = "held"))
      })
    },
    phaseLabel() {
      return { flash: "the flash: a frame from the boss's view", in: "the portrait", out: "fading", gone: "back to the game" }[this.phase]
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

.bp-dim {
  position: absolute;
  inset: 0;
  background: rgba(0, 0, 0, 0.3);
  transition: opacity 0.6s;
}

.bp-frame {
  position: absolute;
  left: 50%;
  top: 10%;
  width: 44%;
  transform: translateX(-50%);
  text-align: center;
  transition: opacity 0.6s;
}

.bp-shot {
  position: relative;
  aspect-ratio: 16 / 9;
  overflow: hidden;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 2px solid #DEBA5C;
  box-shadow: 0 0 0 1px rgba(0, 0, 0, 0.6), 0 6px 24px rgba(0, 0, 0, 0.7);
  background: radial-gradient(ellipse at 50% 85%, #6b5a3e 0%, #2b2a24 60%, #141414 100%);
}

.bp-model {
  display: inline-block;
  transform: scale(0.9);
}

/* the client's own windows, caught in the frame unless the rule hides them */
.bp-ui {
  position: absolute;
  background: rgba(20, 24, 40, 0.75);
  border: 1px solid rgba(160, 170, 200, 0.5);
}

.bp-ui-chat {
  left: 3%;
  bottom: 4%;
  width: 34%;
  height: 26%;
}

.bp-ui-bar {
  right: 3%;
  bottom: 4%;
  width: 40%;
  height: 9%;
}

.bp-ui-target {
  left: 38%;
  top: 4%;
  width: 24%;
  height: 7%;
}

.bp-rule {
  height: 1px;
  margin: 1.2cqw 18% 0.6cqw;
  background: linear-gradient(90deg, transparent, #DEBA5C, transparent);
}

.bp-title {
  font-family: Cinzel, serif;
  font-weight: 600;
  font-size: 3.4cqw;
  color: #F6F0E2;
  text-shadow: 0 2px 6px rgba(0, 0, 0, 0.9);
  line-height: 1.2;
}

.bp-subtitle {
  font-family: Cinzel, serif;
  font-size: 1.6cqw;
  color: #CDC4B0;
  font-style: italic;
  text-shadow: 0 1px 4px rgba(0, 0, 0, 0.9);
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

.bp-frame.bp-flash, .bp-dim.bp-flash, .bp-frame.bp-out, .bp-dim.bp-out, .bp-frame.bp-gone, .bp-dim.bp-gone {
  opacity: 0;
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
