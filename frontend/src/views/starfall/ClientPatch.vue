<template>
  <div>
    <eq-window-simple title="Client Patch">
      <div>
        Everything the RoF2 client needs from Spire in one zip, laid out as the client folder: unzip it into the folder
        with <code>eqgame.exe</code>. <code>spells_us.txt</code> and <code>dbstr_us.txt</code> are made from the
        database as you download; icon sheets and item models come from
        <router-link :to="iconsRoute">Icons &amp; Models</router-link>; the files below that the database cannot make
        (<code>GlobalLoad.txt</code>, <code>&lt;zone&gt;_chr.txt</code>, race and zone <code>.eqg</code> /
        <code>.s3d</code>, <code>EQUI_*.xml</code>) are kept as you upload them.
      </div>

      <div class="mt-3 d-flex align-items-center flex-wrap">
        <b-button variant="warning" class="mr-3" @click="download()" :disabled="!contents">
          <i class="fa fa-cloud-download mr-1"></i> Download client patch
        </b-button>
        <span class="text-muted" v-if="contents">
          {{ contents.files.length }} files<span v-if="totalSize()">, {{ size(totalSize()) }} before the spells and strings</span>
        </span>
      </div>

      <div v-for="(n, i) in (contents ? contents.notes : [])" :key="'n' + i" class="mt-2 text-warning">
        <i class="fa fa-exclamation-triangle mr-1"></i> {{ n }}
      </div>

      <div
        :class="'starfall-drop mt-3 ' + (dragging ? 'dragging' : '')"
        @dragover.prevent="dragging = true"
        @dragleave.prevent="dragging = false"
        @drop.prevent="onDrop"
        @click="$refs.picker.click()"
      >
        <i class="fa fa-upload mr-2"></i>
        {{ uploading ? "Uploading..." : "Drop client files here (GlobalLoad.txt, zone _chr.txt, .eqg, .s3d, EQUI_*.xml, icon sheets), or click to choose" }}
        <input ref="picker" type="file" multiple accept=".txt,.xml,.eqg,.s3d,.dds,.png" style="display: none" @change="onPick">
      </div>
      <div class="mt-2" v-if="results.length > 0">
        <div v-for="(r, i) in results" :key="i" :class="r.error ? 'text-danger' : 'text-success'">
          <i :class="'fa ' + (r.error ? 'fa-times' : 'fa-check') + ' mr-1'"></i> {{ r.error || r.result }}
        </div>
      </div>

      <info-error-banner class="mt-2" :notification="notification" :error="error"
                         @dismiss-error="error = ''" @dismiss-notification="notification = ''"/>

      <table class="eq-table eq-highlight-rows mt-3 client-patch-files" v-if="contents">
        <thead>
        <tr>
          <th>In the client folder</th>
          <th>From</th>
          <th class="text-right">Size</th>
          <th>Updated</th>
          <th></th>
        </tr>
        </thead>
        <tbody>
        <tr v-for="f in contents.files" :key="f.path">
          <td><code>{{ f.path }}</code></td>
          <td>{{ f.source }}</td>
          <td class="text-right">{{ f.size ? size(f.size) : "" }}</td>
          <td>{{ when(f.updated) }}</td>
          <td>
            <b-button v-if="f.source.startsWith('uploaded')" size="sm" variant="outline-danger"
                      @click="remove(f)" title="Leave it out of the patch">
              <i class="fa fa-trash"></i>
            </b-button>
          </td>
        </tr>
        </tbody>
      </table>
    </eq-window-simple>
  </div>
</template>

<script>
import EqWindowSimple  from "@/components/eq-ui/EQWindowSimple";
import InfoErrorBanner from "@/components/InfoErrorBanner";
import {SpireApi}      from "@/app/api/spire-api";
import {ROUTE}         from "@/routes";

export default {
  name: "StarfallClientPatch",
  components: { EqWindowSimple, InfoErrorBanner },
  data() {
    return {
      contents: null,
      dragging: false,
      uploading: false,
      results: [],
      notification: "",
      error: "",
      iconsRoute: ROUTE.STARFALL_ICONS_MODELS,
    }
  },
  methods: {
    showError(e) {
      this.error = e && e.response && e.response.data && e.response.data.error ? e.response.data.error : String(e)
    },
    size(n) {
      return n >= 1 << 20 ? (n / (1 << 20)).toFixed(1) + " MB" : Math.max(1, Math.round(n / 1024)) + " KB"
    },
    totalSize() {
      return this.contents ? this.contents.files.reduce((t, f) => t + (f.size || 0), 0) : 0
    },
    when(unix) {
      return unix ? new Date(unix * 1000).toLocaleString() : ""
    },

    async load() {
      try {
        const r       = await SpireApi.v1().get("client-files/patch/contents")
        this.contents = r.data
      } catch (e) {
        this.showError(e)
      }
    },
    // a link, so the browser saves the zip as it streams
    download() {
      const q = SpireApi.getAccessTokenQueryString()
      window.open(SpireApi.getBaseV1Path() + "/client-files/patch" + (q.jwt ? "?jwt=" + encodeURIComponent(q.jwt) : ""), "_blank")
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
        const r      = await SpireApi.v1().post("client-assets/upload", form)
        this.results = r.data.results || []
      } catch (e) {
        this.showError(e)
      }
      this.uploading = false
      await this.load()
    },
    async remove(f) {
      const name = f.path.split("/").pop()
      if (!confirm("Leave " + f.path + " out of the client patch? Spire forgets its copy.")) {
        return
      }
      try {
        await SpireApi.v1().delete("client-assets/client-file/" + encodeURIComponent(name))
        this.notification = f.path + " removed"
      } catch (e) {
        this.showError(e)
      }
      await this.load()
    },
  },
  mounted() {
    this.load()
  },
}
</script>

<style scoped>
.starfall-drop {
  border: 2px dashed rgba(255, 255, 255, .35);
  border-radius: 8px;
  padding: 24px;
  text-align: center;
  cursor: pointer;
}

.starfall-drop.dragging {
  border-color: #ffc107;
  background: rgba(255, 193, 7, .08);
}

.client-patch-files td {
  vertical-align: middle;
}
</style>
