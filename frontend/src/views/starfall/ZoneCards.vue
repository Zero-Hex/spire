<template>
  <div>
    <eq-window-simple title="Zone Cards">
      <div>
        The title card players see as they arrive in a zone, drawn by the Starfall DLL. Every zone has one with its long
        name; here you can give a zone a different title, add a subtitle line under it, or hide its card. A player's
        first entry shows the full card with a short pause; return visits show the name small near the top.
      </div>
      <div class="text-muted hint mt-1">
        A zone reads its card when it boots: after saving, use <code>#zonecard reload</code> in that zone (then
        <code>#zonecard first</code> to see the full card), or wait for it to restart.
      </div>
      <info-error-banner class="mt-2" :notification="notification" :error="error"
                         @dismiss-error="error = ''" @dismiss-notification="notification = ''"/>
    </eq-window-simple>

    <div class="row mt-3" v-if="list">
      <!-- the zones -->
      <div class="col-lg-5">
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
                  <div>{{ cardTitle(z) }}</div>
                  <div v-if="cardOf(z) && cardOf(z).subtitle" class="hint zc-sub">{{ cardOf(z).subtitle }}</div>
                </td>
                <td class="text-muted hint text-right" style="white-space: nowrap">
                  {{ z.short_name }}
                  <div>
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
      <div class="col-lg-7">
        <eq-window-simple v-if="!selected" title="Card">
          <div class="text-muted">Pick a zone.</div>
        </eq-window-simple>
        <eq-window-simple v-else :title="selected.long_name + ' (' + selected.short_name + ')' + (dirty() ? ' (not saved)' : '')">
          <!-- the preview -->
          <div class="zc-scene" :class="form.enabled ? '' : 'zc-scene-off'" :style="{ '--zc-w': screen }">
            <div class="zc-card">
              <div class="zc-rule"></div>
              <div class="zc-title">{{ form.title.trim() || selected.long_name }}</div>
              <div class="zc-rule"></div>
              <div v-if="form.subtitle.trim()" class="zc-subtitle">{{ form.subtitle.trim() }}</div>
            </div>
            <div v-if="!form.enabled" class="zc-off">Hidden: players see no card here</div>
          </div>
          <div class="d-flex justify-content-between hint text-muted mt-1 mb-3">
            <span>
              First entry, roughly as on a
              <b-form-select size="sm" v-model="screen" :options="[1280, 1920, 2560]" class="zc-screen"/>
              wide screen.
            </span>
            <span>Return visit: <span class="zc-return">{{ form.title.trim() || selected.long_name }}</span></span>
          </div>

          <!-- the text -->
          <div class="form-group mb-2">
            <label class="mb-1">Title <span class="text-muted hint">(empty: the zone's long name)</span></label>
            <b-form-input v-model="form.title" :placeholder="selected.long_name" maxlength="63"/>
            <div class="hint d-flex justify-content-between">
              <span class="text-warning">{{ textWarning(form.title) }}</span>
              <span :class="bytes(form.title) > 63 ? 'text-danger' : 'text-muted'">{{ bytes(form.title) }} / 63</span>
            </div>
          </div>
          <div class="form-group mb-2">
            <label class="mb-1">Subtitle <span class="text-muted hint">(empty: none)</span></label>
            <b-form-input v-model="form.subtitle" placeholder="One line, a few words, in the zone's voice" maxlength="127"/>
            <div class="hint d-flex justify-content-between">
              <span class="text-warning">{{ textWarning(form.subtitle) }}</span>
              <span :class="bytes(form.subtitle) > 127 ? 'text-danger' : 'text-muted'">{{ bytes(form.subtitle) }} / 127</span>
            </div>
          </div>
          <b-form-checkbox v-model="form.enabled" class="mb-3">Show this zone's card</b-form-checkbox>

          <div class="d-flex align-items-center flex-wrap">
            <b-button size="sm" variant="success" class="mr-2" :disabled="!dirty() || busy || !valid()" @click="save()">
              <i :class="'fa ' + (busy ? 'fa-cog fa-spin' : 'fa-save') + ' mr-1'"></i> Save
            </b-button>
            <b-button size="sm" variant="outline-secondary" class="mr-2" :disabled="!dirty() || busy" @click="pick(selected)">Undo</b-button>
            <b-button v-if="cardOf(selected)" size="sm" variant="outline-danger" class="mr-3" :disabled="busy" @click="reset()"
                      title="Delete the row: the card shows the long name, no subtitle">
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
  </div>
</template>

<script>
import EqWindowSimple  from "@/components/eq-ui/EQWindowSimple";
import InfoErrorBanner from "@/components/InfoErrorBanner";
import {SpireApi}      from "@/app/api/spire-api";

export default {
  name: "ZoneCards",
  components: { EqWindowSimple, InfoErrorBanner },
  data() {
    return {
      list: null,
      search: "",
      filter: "all",
      selected: null,
      form: { title: "", subtitle: "", enabled: true },
      screen: 1280,
      busy: false,
      error: "",
      notification: "",
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
    pick(z) {
      if (this.selected && this.selected.short_name !== z.short_name && this.dirty() && !confirm("Leave your changes to this card?")) {
        return
      }
      const c = this.cardOf(z)
      this.selected = z
      this.form = { title: c ? c.title : "", subtitle: c ? c.subtitle : "", enabled: c ? !!c.enabled : true }
      if (this.$route.query.zone !== z.short_name) {
        this.$router.replace({ query: { zone: z.short_name } }).catch(() => {})
      }
    },
    dirty() {
      if (!this.selected) {
        return false
      }
      const c = this.cardOf(this.selected) || { title: "", subtitle: "", enabled: 1 }
      return this.form.title.trim() !== c.title || this.form.subtitle.trim() !== c.subtitle || this.form.enabled !== !!c.enabled
    },
    bytes(s) {
      return new TextEncoder().encode(s.trim()).length
    },
    valid() {
      return this.bytes(this.form.title) <= 63 && this.bytes(this.form.subtitle) <= 127
    },
    textWarning(s) {
      return /[^\x20-\x7e]/.test(s) ? "Letters outside plain English text may not draw in the DLL's font." : ""
    },
    sql() {
      const q = s => "'" + s.trim().replace(/\\/g, "\\\\").replace(/'/g, "''") + "'"
      return `INSERT INTO custom_zone_card (zone_short_name, title, subtitle, enabled)\n` +
        `VALUES (${q(this.selected.short_name)}, ${q(this.form.title)}, ${q(this.form.subtitle)}, ${this.form.enabled ? 1 : 0})\n` +
        `ON DUPLICATE KEY UPDATE title = VALUES(title), subtitle = VALUES(subtitle), enabled = VALUES(enabled);`
    },
    async save() {
      this.busy = true
      this.error = ""
      try {
        const r = await SpireApi.v1().put(`zone-cards/${this.selected.short_name}`, {
          title: this.form.title, subtitle: this.form.subtitle, enabled: this.form.enabled ? 1 : 0,
        })
        this.list.cards = this.list.cards.filter(c => c.zone_short_name !== r.data.zone_short_name).concat([r.data])
        this.form = { title: r.data.title, subtitle: r.data.subtitle, enabled: !!r.data.enabled }
        this.notification = `Saved ${this.selected.long_name}'s card. In that zone, #zonecard reload shows it.`
      } catch (e) {
        this.error = this.message(e)
      } finally {
        this.busy = false
      }
    },
    async reset() {
      if (!confirm(`Put ${this.selected.long_name}'s card back to its long name, with no subtitle?`)) {
        return
      }
      this.busy = true
      this.error = ""
      try {
        await SpireApi.v1().delete(`zone-cards/${this.selected.short_name}`)
        this.list.cards = this.list.cards.filter(c => c.zone_short_name !== this.selected.short_name)
        this.form = { title: "", subtitle: "", enabled: true }
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
  max-height: 68vh;
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

/* the card, scaled to a screen --zc-w pixels wide: the DLL's title is 44px, its subtitle 22px */
.zc-scene {
  container-type: inline-size;
  position: relative;
  aspect-ratio: 16 / 7;
  background: radial-gradient(ellipse at 50% 40%, #3a4250 0%, #1c2028 55%, #0d0f13 100%);
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
}

.zc-scene-off .zc-card {
  opacity: 0.25;
}

.zc-card {
  text-align: center;
  font-family: Arimo, Arial, "Helvetica Neue", sans-serif;
  max-width: 90%;
}

.zc-title {
  font-size: calc(44 / var(--zc-w) * 100cqw);
  color: #f3ecd8;
  line-height: 1.25;
  letter-spacing: 0.02em;
  text-shadow: 0 0 0.3em rgba(0, 0, 0, 0.9);
  padding: 0.25em 0;
}

.zc-rule {
  height: 1px;
  margin: 0 auto;
  width: calc(520 / var(--zc-w) * 100cqw);
  background: linear-gradient(90deg, rgba(212, 175, 55, 0) 0%, #d4af37 30%, #d4af37 70%, rgba(212, 175, 55, 0) 100%);
}

.zc-subtitle {
  font-size: calc(22 / var(--zc-w) * 100cqw);
  color: #d8cfb4;
  margin-top: 0.7em;
  text-shadow: 0 0 0.3em rgba(0, 0, 0, 0.9);
}

.zc-off {
  position: absolute;
  bottom: 8px;
  right: 12px;
  font-size: 0.85em;
  color: #e88;
}

.zc-return {
  font-family: Arimo, Arial, sans-serif;
  color: #f3ecd8;
  background: #1c2028;
  padding: 1px 8px;
  border-radius: 3px;
}

.zc-screen {
  width: auto;
  display: inline-block;
  height: auto;
  padding: 0 4px;
  font-size: 1em;
}

.zc-sql {
  background: rgba(0, 0, 0, 0.35);
  color: #ddd;
  padding: 6px 8px;
  white-space: pre-wrap;
  margin: 4px 0 0;
}
</style>
