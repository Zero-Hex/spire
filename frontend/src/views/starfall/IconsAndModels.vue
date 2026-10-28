<template>
  <div>
    <eq-window-simple title="Icons & Models">
      <div>
        Drop the client's icon sheets (<code>dragitem&lt;N&gt;.dds</code> or <code>.png</code>) or item model archives
        (<code>.eqg</code> from the weapon toolkit) here. Spire then shows them wherever it shows an icon or an item model.
        Model previews are drawn in this browser by the EQ Model Viewer.
      </div>
      <div class="mt-2 text-muted" style="font-size: 13px">
        Sheet <i>N</i> must be the <i>N</i>-th <code>&lt;Frames&gt;</code> entry of <code>A_DragItem</code> in
        <code>EQUI_Animations.xml</code>: it holds icons 500 + 36(<i>N</i>&minus;1) onward. Icon pickers list the
        RoF2 client's icons ({{ firstIcon }}&ndash;{{ rof2LastIcon }}) plus imported sheets.
        <span v-if="dropFolder">
          Files copied into <code>{{ dropFolder }}</code> on the server are imported too.
        </span>
      </div>

      <div
        :class="'starfall-drop mt-3 ' + (dragging ? 'dragging' : '')"
        @dragover.prevent="dragging = true"
        @dragleave.prevent="dragging = false"
        @drop.prevent="onDrop"
        @click="$refs.picker.click()"
      >
        <i class="fa fa-upload mr-2"></i>
        {{ uploading ? "Importing..." : "Drop icon sheets or .eqg archives here, or click to choose files" }}
        <input
          ref="picker"
          type="file"
          multiple
          accept=".dds,.png,.eqg"
          style="display: none"
          @change="onPick"
        >
      </div>
      <div class="mt-2 d-flex align-items-center">
        <span class="mr-2">Send uploads to</span>
        <b-form-select size="sm" v-model="hold" :options="holds" style="max-width: 260px" class="mr-2"/>
        <span class="text-muted" style="font-size: 13px">Spire shows them either way; change it per file on Client Patch</span>
      </div>

      <div class="mt-2" v-if="results.length > 0">
        <div v-for="(r, i) in results" :key="i" :class="r.error ? 'text-danger' : 'text-success'">
          <i :class="'fa ' + (r.error ? 'fa-times' : 'fa-check') + ' mr-1'"></i>
          {{ r.error || r.result }}
        </div>
      </div>

      <div class="mt-2" v-if="rendering">
        <i class="fa fa-cog fa-spin mr-1"></i> {{ rendering }}
      </div>

      <info-error-banner
        class="mt-2"
        :notification="notification"
        :error="error"
        @dismiss-error="error = ''"
        @dismiss-notification="notification = ''"
      />
    </eq-window-simple>

    <eq-window-simple class="mt-3" title="Icon sheets">
      <div v-if="sheets().length === 0" class="text-muted text-center">
        No icon sheets imported: Spire shows the stock icons.
      </div>
      <div style="overflow-x: auto" v-if="sheets().length > 0">
      <table class="eq-table" style="display: table; width: 100%">
        <thead>
        <tr>
          <th style="width: 70px">Sheet</th>
          <th style="width: 200px">File</th>
          <th style="width: 130px">Icons</th>
          <th></th>
          <th style="width: 90px"></th>
        </tr>
        </thead>
        <tbody>
        <tr v-for="s in sheets()" :key="s.sheet">
          <td>{{ s.sheet }} <b-badge v-if="s.ignore" variant="warning" title="Kept out of the client patch (Ignore)">not sent</b-badge><b-badge v-else-if="s.beta" variant="info" title="Sent to the beta channel only">beta only</b-badge></td>
          <td>{{ s.file }}<div class="text-muted" v-if="s.sheet <= 178">replaces a stock sheet</div></td>
          <td>{{ s.first_icon }}&ndash;{{ s.last_icon }}</td>
          <td>
            <span
              v-for="id in range(s.first_icon, s.last_icon)"
              :key="id"
              :class="'item-' + id + ' mr-1'"
              :title="id"
            />
          </td>
          <td>
            <b-button size="sm" variant="outline-danger" @click="removeSheet(s)">
              <i class="fa fa-trash"></i> Remove
            </b-button>
          </td>
        </tr>
        </tbody>
      </table>
      </div>
    </eq-window-simple>

    <eq-window-simple class="mt-3" title="Item model archives">
      <div v-if="archives().length === 0" class="text-muted text-center">
        No model archives imported.
      </div>
      <!-- one card per archive; cards sit side by side and wrap -->
      <div class="starfall-archives">
        <div v-for="a in archives()" :key="a.file" class="starfall-archive">
          <div class="starfall-archive-name font-weight-bold" :title="a.file">
            {{ a.file }} <b-badge v-if="a.ignore" variant="warning" title="Kept out of the client patch (Ignore)">not sent</b-badge><b-badge v-else-if="a.beta" variant="info" title="Sent to the beta channel only">beta only</b-badge>
          </div>
          <div class="mt-1 mb-2">
            <b-button size="sm" variant="outline-warning" class="mr-1" @click="redraw(a)" :disabled="!!rendering" title="Redraw previews">
              <i class="fa fa-refresh"></i> Redraw previews
            </b-button>
            <b-button size="sm" variant="outline-danger" class="mr-1" @click="removeArchive(a)" title="Remove this archive">
              <i class="fa fa-trash"></i> Remove
            </b-button>
            <b-button size="sm" variant="outline-secondary" class="mr-1" @click="setKind(a, 'armor')"
                      title="A Heroes' Forge armor set: show it as one set, not item models">
              Armor set
            </b-button>
            <b-button size="sm" variant="outline-success" @click="$refs.generator.open(a, 'weapon')"
                      title="Make items with these models: copies of a weapon">
              <i class="fa fa-magic"></i> Create items
            </b-button>
          </div>
          <div class="starfall-models">
            <div v-for="id in a.models" :key="id" class="starfall-model text-center">
              <span v-if="hasPreview(id)" :class="'object-ctn-' + id" />
              <div v-else class="starfall-model-empty">no preview yet</div>
              <div>IT{{ id }}</div>
            </div>
          </div>
        </div>
      </div>
    </eq-window-simple>

    <eq-window-simple class="mt-3" title="Armor sets" v-if="sets.length > 0">
      <div class="text-muted mb-2" style="font-size: 13px">
        One card per Heroes' Forge set, whatever archives its models are in: <code>NewArmorTagData.txt</code> (upload
        it to Client Patch, in <code>Resources</code>) says which set each model is in, and each model's materials say
        which texture it wears. Models no tag row names are grouped by their texture.
      </div>
      <div class="starfall-archives">
        <div v-for="set in sets" :key="setKey(set)" class="starfall-archive starfall-armor">
          <div class="d-flex">
            <img v-if="set.swatch" :src="fileUrl(set.swatch)" :alt="set.texture" class="starfall-swatch mr-2" :title="set.texture">
            <div v-else class="starfall-model-empty starfall-swatch mr-2">no texture</div>
            <div class="starfall-archive-body">
              <div class="font-weight-bold">
                <span v-if="set.set">Set {{ set.set }}</span>
                <span v-else class="text-warning" title="No NewArmorTagData.txt row names these models: the client shows nothing for them">
                  Not in NewArmorTagData.txt<span v-if="set.guess"> (set {{ set.guess }}?)</span>
                </span>
              </div>
              <div>
                {{ set.models.length }} pieces<span v-if="set.texture">, <code>{{ set.texture }}</code></span>
              </div>
              <div v-if="set.set" class="mt-1">
                <span v-for="slot in slotNames" :key="slot.slot"
                      :class="'badge mr-1 mb-1 ' + (set.slots.includes(slot.slot) ? 'badge-success' : 'badge-dark')"
                      :title="set.slots.includes(slot.slot) ? 'has models' : 'no models'">
                  {{ slot.name }}
                </span>
              </div>
              <div class="mt-1 starfall-set-archives">
                <span v-for="ar in set.archives" :key="ar.file" class="mr-2">
                  {{ ar.file }}<span class="text-muted"> ({{ ar.models }})</span>
                  <b-badge v-if="ar.ignore" variant="warning">not sent</b-badge>
                  <b-badge v-else-if="ar.remove" variant="danger">removed</b-badge>
                  <b-badge v-else-if="ar.beta" variant="info">beta only</b-badge>
                </span>
              </div>
              <div v-if="set.missing.length" class="text-warning mt-1" style="font-size: 13px"
                   :title="set.missing.map((m) => 'IT' + m).join(', ')">
                <i class="fa fa-exclamation-triangle"></i>
                {{ set.missing.length }} model(s) its tag rows name are in no archive here (IT{{ set.missing[0] }}...)
              </div>
              <div class="mt-1">
                <b-button size="sm" variant="outline-success" class="mr-1" @click="$refs.generator.openSet(set, 'armor')"
                          title="Make a piece per slot of this set: copies of the cloth set">
                  <i class="fa fa-magic"></i> Create armor
                </b-button>
                <b-button size="sm" variant="outline-success" @click="$refs.generator.openSet(set, 'ornament')"
                          title="Make an ornament per slot of this set: copies of the Old Hero's Forge set">
                  <i class="fa fa-magic"></i> Create ornaments
                </b-button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </eq-window-simple>

    <eq-window-simple class="mt-3" title="Armor archives" v-if="armorArchives().length > 0">
      <table class="eq-table" style="display: table; width: 100%">
        <thead>
        <tr>
          <th>Archive</th>
          <th>Pieces</th>
          <th>Models</th>
          <th></th>
        </tr>
        </thead>
        <tbody>
        <tr v-for="a in armorArchives()" :key="a.file">
          <td>
            {{ a.file }}
            <b-badge v-if="a.ignore" variant="warning" title="Kept out of the client patch (Ignore)">not sent</b-badge>
            <b-badge v-else-if="a.beta" variant="info" title="Sent to the beta channel only">beta only</b-badge>
          </td>
          <td>{{ a.models.length }}</td>
          <td>
            <span v-for="g in groups(a.models)" :key="g.from" class="badge badge-secondary mr-1 mb-1">
              IT{{ g.from }}<span v-if="g.to !== g.from">&ndash;{{ g.to }}</span> ({{ g.count }})
            </span>
          </td>
          <td class="text-nowrap">
            <b-button size="sm" variant="outline-secondary" class="mr-1" @click="setKind(a, 'items')"
                      title="Not armor: show its models as item models, with previews">
              Item models
            </b-button>
            <b-button size="sm" variant="outline-danger" @click="removeArchive(a)">
              <i class="fa fa-trash"></i> Remove
            </b-button>
          </td>
        </tr>
        </tbody>
      </table>
    </eq-window-simple>

    <item-generator ref="generator"/>

    <!-- the model viewer, off screen, renders previews -->
    <iframe
      ref="viewer"
      src="/model-viewer/viewer.html"
      title="Model preview renderer"
      style="position: fixed; left: -10000px; top: 0; width: 800px; height: 600px; border: 0"
      aria-hidden="true"
      tabindex="-1"
    />
  </div>
</template>

<script>
import EqWindowSimple  from "@/components/eq-ui/EQWindowSimple";
import InfoErrorBanner from "@/components/InfoErrorBanner";
import {SpireApi}      from "@/app/api/spire-api";
import {ClientAssets}  from "@/app/client-assets";
import ItemGenerator   from "./ItemGenerator";

const PREVIEW_SIZE = 125 // the stock item model previews are 125px
const sleep        = (ms) => new Promise((r) => setTimeout(r, ms))

export default {
  name: "StarfallIconsAndModels",
  components: { EqWindowSimple, InfoErrorBanner, ItemGenerator },
  data() {
    return {
      index: { sheets: {}, models: {}, archives: {} },
      dropFolder: "",
      firstIcon: 500,
      rof2LastIcon: 6907,
      dragging: false,
      uploading: false,
      rendering: "",
      results: [],
      sets: [], // Heroes' Forge sets, by set number
      slotNames: ["Helm", "Chest", "Arms", "Wrists", "Hands", "Legs", "Feet", "Robe"].map((name, slot) => ({ slot, name })),
      hold: "", // how uploads are held back from players: "", "beta" or "ignore"
      holds: [
        { value: "", text: "Live and beta" },
        { value: "beta", text: "Beta only" },
        { value: "ignore", text: "Neither (Ignore)" },
      ],
      notification: "",
      error: "",
    }
  },
  methods: {
    range(a, b) {
      const out = []
      for (let i = a; i <= b; i++) {
        out.push(i)
      }
      return out
    },
    sheets() {
      return Object.values(this.index.sheets || {}).sort((a, b) => a.sheet - b.sheet)
    },
    // smallest first, so one-model archives (the toolkit's usual output)
    // pack into rows instead of being split up by wide ones
    archives() {
      return Object.values(this.index.archives || {}).filter((a) => a.kind !== "armor").sort((a, b) => (a.models.length - b.models.length) || a.file.localeCompare(b.file))
    },
    armorArchives() {
      return Object.values(this.index.archives || {}).filter((a) => a.kind === "armor").sort((a, b) => a.file.localeCompare(b.file))
    },
    async loadSets() {
      try {
        const r   = await SpireApi.v1().get("client-assets/armor-sets")
        this.sets = r.data.sets || []
      } catch (e) {
        this.showError(e)
      }
    },
    setKey(set) {
      return set.set ? "set" + set.set : "tex" + set.archives.map((a) => a.file).join() + set.texture
    },
    // runs of model numbers: a set's slots are numbered in blocks
    groups(ids) {
      const out = []
      for (const id of [...ids].sort((a, b) => a - b)) {
        const last = out[out.length - 1]
        if (last && id === last.to + 1) {
          last.to = id
          last.count++
        } else {
          out.push({ from: id, to: id, count: 1 })
        }
      }
      return out
    },
    fileUrl(path) {
      return SpireApi.getBasePath() + "/api/v1/client-assets/file/" + path + "?v=" + this.index.version
    },
    async setKind(a, kind) {
      try {
        const r    = await SpireApi.v1().put("client-assets/archive/" + encodeURIComponent(a.file) + "/kind", { kind: kind })
        this.index = r.data.index
        await ClientAssets.refresh()
      } catch (e) {
        this.showError(e)
      }
      if (kind === "items") {
        await this.renderMissing()
      }
    },
    hasPreview(id) {
      const m = (this.index.models || {})[id]
      return m && m.preview
    },
    showError(e) {
      this.error = e && e.response && e.response.data && e.response.data.error ? e.response.data.error : String(e)
    },

    async load() {
      try {
        const r           = await SpireApi.v1().get("client-assets")
        this.index        = r.data.index
        this.dropFolder   = r.data.drop_folder
        this.rof2LastIcon = r.data.rof2_last_icon
      } catch (e) {
        this.showError(e)
      }
    },

    onDrop(e) {
      this.dragging = false
      this.upload(e.dataTransfer.files)
    },
    onPick(e) {
      this.upload(e.target.files)
      e.target.value = ""
    },

    async upload(fileList) {
      const files = Array.from(fileList || [])
      if (files.length === 0) {
        return
      }

      this.uploading = true
      this.results   = []
      this.error     = ""
      try {
        const form = new FormData()
        files.forEach((f) => form.append("files", f, f.name))
        if (this.hold) {
          form.append(this.hold, "1")
        }
        const r      = await SpireApi.v1().post("client-assets/upload", form)
        this.results = r.data.results || []
        this.index   = r.data.index
        await ClientAssets.refresh()
      } catch (e) {
        this.showError(e)
      }
      this.uploading = false

      await this.renderMissing()
    },

    // ------------------------------------------------------------ previews

    // the viewer's functions are globals of its page; wait for them
    async viewer() {
      const frame = this.$refs.viewer
      for (let i = 0; i < 200; i++) {
        const win = frame && frame.contentWindow
        if (win && typeof win.exportIcon === "function" && typeof win.openArchiveFile === "function") {
          return win
        }
        await sleep(100)
      }
      throw new Error("The model viewer did not load (/model-viewer/viewer.html)")
    },

    async renderMissing() {
      let drawn = 0
      for (const a of this.archives()) {
        const missing = a.models.filter((id) => !this.hasPreview(id))
        if (missing.length > 0) {
          drawn += await this.renderArchive(a, missing)
        }
      }
      if (drawn > 0) {
        this.notification = "Drew " + drawn + " model preview" + (drawn === 1 ? "" : "s")
      }
    },

    // returns how many previews were drawn
    async renderArchive(archive, ids) {
      this.rendering = "Drawing previews for " + archive.file
      let drawn      = 0
      try {
        const win  = await this.viewer()
        const r    = await SpireApi.v1().get("client-assets/archive/" + encodeURIComponent(archive.file), { responseType: "blob" })
        const file = new File([r.data], archive.file)
        if (!(await win.openArchiveFile(file))) {
          throw new Error("The model viewer could not open " + archive.file)
        }

        // exportIcon hands its PNG to saveFile; catch it instead of downloading
        let captured = null
        win.saveFile = async (name, blob) => {
          captured = blob
        }

        for (const id of ids) {
          this.rendering = "Drawing IT" + id + " (" + (drawn + 1) + " of " + ids.length + ")"
          win.openModel("it" + id + ".mod")
          await sleep(300) // textures settle
          captured = null
          await win.exportIcon(PREVIEW_SIZE)
          if (!captured) {
            throw new Error("No preview came back for IT" + id)
          }

          const form = new FormData()
          form.append("file", captured, "CTN_" + id + ".png")
          await SpireApi.v1().post("client-assets/model/" + id + "/preview", form)
          drawn++
        }
      } catch (e) {
        this.showError(e)
      }
      this.rendering = ""

      await this.load()
      await ClientAssets.refresh()
      return drawn
    },

    async redraw(archive) {
      const drawn       = await this.renderArchive(archive, archive.models)
      this.notification = "Redrew " + drawn + " preview" + (drawn === 1 ? "" : "s") + " for " + archive.file
    },

    // ------------------------------------------------------------ removal

    async removeSheet(s) {
      if (!confirm("Remove sheet " + s.sheet + "? Spire shows the stock icons " + s.first_icon + "-" + s.last_icon + " again" + (s.sheet > 178 ? " (none: RoF2 has no such sheet)" : "") + ".")) {
        return
      }
      try {
        const r    = await SpireApi.v1().delete("client-assets/sheet/" + s.sheet)
        this.index = r.data.index
        await ClientAssets.refresh()
      } catch (e) {
        this.showError(e)
      }
    },

    async removeArchive(a) {
      const what = a.kind === "armor" ? "the armor set " + a.file + " (" + a.models.length + " pieces)" : a.file + " and the previews of its " + a.models.length + " models"
      if (!confirm("Remove " + what + "?")) {
        return
      }
      try {
        const r    = await SpireApi.v1().delete("client-assets/archive/" + encodeURIComponent(a.file))
        this.index = r.data.index
        await ClientAssets.refresh()
      } catch (e) {
        this.showError(e)
      }
    },
  },
  watch: {
    "index.version"() {
      this.loadSets()
    },
  },
  async mounted() {
    await this.load()
    // anything copied into the drop folder since: draw what is missing
    await this.renderMissing()
  },
}
</script>

<style scoped>
.starfall-drop {
  border: 2px dashed rgba(255, 255, 255, .35);
  border-radius: 8px;
  padding: 30px;
  text-align: center;
  cursor: pointer;
}

.starfall-drop.dragging {
  border-color: #ffc107;
  background: rgba(255, 193, 7, .08);
}

.starfall-archives {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  gap: 12px;
}

.starfall-swatch {
  width: 125px;
  height: 125px;
  flex: none;
  border-radius: 4px;
}

.starfall-armor .starfall-archive-body {
  min-width: 0;
}

.starfall-armor-groups {
  max-width: 420px;
}

.starfall-archive {
  min-width: 250px;
  max-width: 100%;
  border: 1px solid rgba(255, 255, 255, .15);
  border-radius: 8px;
  padding: 8px;
}

/* takes the card's width without widening it: a long name is cut short */
.starfall-archive-name {
  width: 0;
  min-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.starfall-models {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 8px;
}

.starfall-model {
  min-width: 130px;
  border: 1px solid rgba(218, 218, 218, .3);
  border-radius: 7px;
  padding: 4px;
}

.starfall-model-empty {
  height: 125px;
  width: 125px;
  display: flex;
  align-items: center;
  justify-content: center;
  opacity: .5;
}
</style>
