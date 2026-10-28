<template>
  <div>
    <eq-window-simple title="Client Patch">
      <div>
        Everything the RoF2 client needs from Spire in one zip, laid out as the client folder: unzip it into the folder
        with <code>eqgame.exe</code>. <code>spells_us.txt</code>, <code>dbstr_us.txt</code>, <code>SkillCaps.txt</code> and
        <code>BaseData.txt</code> are made from the database by the server's <code>export_client_files</code> as you
        download or publish; icon sheets and item models come from
        <router-link :to="iconsRoute">Icons &amp; Models</router-link>; the files below that the database cannot make
        (<code>dinput8.dll</code>, <code>eqhost.txt</code>, <code>GlobalLoad.txt</code>, <code>&lt;zone&gt;_chr.txt</code>,
        race and zone <code>.eqg</code> / <code>.s3d</code>, <code>EQUI*.xml</code>, or anything in a folder you choose)
        are kept as you upload them.
      </div>

      <!-- the patcher -->
      <div class="client-patch-publish mt-3">
        <div class="d-flex align-items-center flex-wrap">
          <b-button variant="success" class="mr-3" @click="publish()" :disabled="publishing">
            <i :class="'fa ' + (publishing ? 'fa-cog fa-spin' : 'fa-paper-plane') + ' mr-1'"></i>
            {{ publishing ? "Publishing..." : "Publish for the patcher" }}
          </b-button>
          <span v-if="published && published.manifest">
            Published <b>{{ published.manifest.version.slice(0, 8) }}</b>
            {{ when(published.manifest.published) }}: {{ published.manifest.files.length }} files,
            {{ size(publishedSize()) }}
          </span>
          <span v-else-if="published" class="text-muted">Nothing is published yet.</span>
        </div>
        <div class="mt-2 text-muted" v-if="published">
          Players' patchers only get what is published. Build the patcher with
          <code>FILELIST_URL</code> = <code>{{ published.filelist_url }}</code>; it reads
          <a :href="published.filelist_url + 'rof/filelist_rof.yml'" target="_blank">filelist_rof.yml</a> there and
          downloads only the files whose MD5 differs. Removing a file here does not delete it from players' clients.
        </div>
        <div v-if="publishResult" class="mt-2">
          <div v-if="nothingChanged()" class="text-muted">Nothing changed since the last publish.</div>
          <div v-for="(list, kind) in changeLists()" :key="kind">
            <span :class="kind === 'removed' ? 'text-warning' : 'text-success'">{{ kind }} ({{ list.length }}):</span>
            <code v-for="f in list" :key="f" class="mr-2">{{ f }}</code>
          </div>
        </div>
      </div>

      <!-- the patcher program: built here, and what players' patchers update to -->
      <div class="client-patch-publish mt-3" v-if="patcher">
        <div class="d-flex align-items-center flex-wrap">
          <b class="mr-3">Patcher program</b>
          <span v-if="patcher.released" class="mr-3">
            Players update to <b>{{ patcher.released.version }}</b>
            (<code>{{ patcher.released.file_name }}.exe</code>, {{ when(patcher.released.built) }})
          </span>
          <span v-else class="mr-3 text-muted">None released: players' patchers keep the one they have.</span>
        </div>

        <div class="mt-2 patcher-settings">
          <div class="d-flex align-items-center mb-1">
            <span class="patcher-label">Source folder</span>
            <b-form-input size="sm" v-model="patcherSettings.source_dir" placeholder="/home/eqemu/starfall-patcher (a checkout with build.sh)"/>
          </div>
          <div class="d-flex align-items-center mb-1">
            <span class="patcher-label">Public address</span>
            <b-form-input size="sm" v-model="patcherSettings.public_url" :placeholder="patcher.suggested_url"/>
            <b-button size="sm" variant="outline-secondary" class="ml-2 text-nowrap" @click="patcherSettings.public_url = patcher.suggested_url">
              This Spire
            </b-button>
          </div>
          <div class="d-flex align-items-center mb-1">
            <span class="patcher-label">Server name</span>
            <b-form-input size="sm" v-model="patcherSettings.server_name" style="max-width: 280px" class="mr-3"/>
            <span class="mr-2 text-nowrap">File name</span>
            <b-form-input size="sm" v-model="patcherSettings.file_name" style="max-width: 200px"/>
            <span class="ml-1">.exe</span>
          </div>
          <div class="d-flex align-items-center">
            <b-form-checkbox v-model="patcherSettings.git_pull" class="mr-3">git pull before building</b-form-checkbox>
            <b-button size="sm" variant="outline-warning" @click="savePatcherSettings()">Save settings</b-button>
          </div>
          <div class="text-muted mt-1" style="font-size: 13px">
            The public address is baked into the patcher: it reads the file list there and updates itself from it, so it
            must be how players reach this Spire, not how you do.
          </div>
        </div>

        <div class="mt-2 d-flex align-items-center flex-wrap">
          <b-button variant="outline-success" class="mr-2" @click="buildPatcher()" :disabled="patcher.running || !patcher.dotnet">
            <i :class="'fa ' + (patcher.running ? 'fa-cog fa-spin' : 'fa-wrench') + ' mr-1'"></i>
            {{ patcher.running ? "Building..." : "Build patcher" }}
          </b-button>
          <b-button variant="outline-secondary" class="mr-2" @click="$refs.patcherPicker.click()" :disabled="patcher.running">
            <i class="fa fa-upload mr-1"></i> Upload one built elsewhere
          </b-button>
          <input ref="patcherPicker" type="file" accept=".exe" style="display: none" @change="uploadPatcher">
          <span v-if="patcher.dotnet" class="text-muted" style="font-size: 13px">
            .NET SDK: <code>{{ patcher.dotnet_path }}</code>
          </span>
          <span v-else class="text-warning">
            The .NET SDK is not installed where Spire runs, so it cannot build here. As the user Spire runs as (in
            akk-stack, inside the eqemu-server container), install it in the server folder, which outlives the
            container:
            <code>curl -sSL https://dot.net/v1/dotnet-install.sh | bash -s -- --channel 8.0{{ patcher.dotnet_install_dir ? " --install-dir " + patcher.dotnet_install_dir : "" }}</code>
          </span>
        </div>

        <div v-if="patcher.staged" class="mt-2 d-flex align-items-center flex-wrap">
          <span class="mr-3">
            Staged <b>{{ patcher.staged.version }}</b> ({{ patcher.staged.source }} {{ when(patcher.staged.built) }},
            reads <code>{{ patcher.staged.public_url }}</code>)
          </span>
          <b-button size="sm" variant="outline-secondary" class="mr-2" @click="downloadStaged()">
            <i class="fa fa-download mr-1"></i> Download to try
          </b-button>
          <b-button size="sm" variant="success" @click="releasePatcher()"
                    :disabled="patcher.released && patcher.released.md5 === patcher.staged.md5">
            <i class="fa fa-paper-plane mr-1"></i> Release to players
          </b-button>
        </div>

        <div v-if="patcher.last_error" class="mt-2 text-danger">{{ patcher.last_error }}</div>
        <pre v-if="patcher.log" ref="patcherLog" class="patcher-log mt-2">{{ patcher.log }}</pre>
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
        {{ uploading ? "Uploading..." : "Drop client files here, or click to choose" }}
        <span v-if="folder"> &rarr; <code>{{ folder }}/</code></span>
        <input ref="picker" type="file" multiple style="display: none" @change="onPick">
      </div>
      <div class="mt-2 d-flex align-items-center flex-wrap">
        <span class="mr-2">Into folder</span>
        <b-form-input size="sm" v-model="folder" placeholder="the usual one (e.g. uifiles/default)" style="max-width: 320px" class="mr-3"/>
        <b-button size="sm" variant="outline-warning" @click="$refs.folderPicker.click()" :disabled="uploading">
          <i class="fa fa-folder-open mr-1"></i> Upload a client folder
        </b-button>
        <input ref="folderPicker" type="file" webkitdirectory multiple style="display: none" @change="onPickFolder">
        <span class="ml-2 text-muted" style="font-size: 13px">
          e.g. the patcher's <code>rof</code> folder: each file goes to its place inside it
        </span>
      </div>
      <div class="mt-2" v-if="results.length > 0">
        <div v-for="(r, i) in results" :key="i" :class="resultClass(r)">
          <i :class="'fa ' + (skipped(r) ? 'fa-minus' : r.error ? 'fa-times' : 'fa-check') + ' mr-1'"></i>
          {{ skipped(r) ? r.file + ": skipped, the patch builds it from the database" : (r.error || r.result) }}
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
      patcher: null,         // null when this user may not build the patcher
      patcherSettings: {},
      patcherTimer: null,
      published: null,
      publishing: false,
      publishResult: null,
      folder: "",
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

    // ------------------------------------------------------------ patcher program
    async loadPatcher() {
      try {
        const r      = await SpireApi.v1().get("patcher-build")
        const first  = !this.patcher
        this.patcher = r.data
        if (first) {
          this.patcherSettings = Object.assign({}, r.data.settings)
        }
        this.$nextTick(() => {
          const log = this.$refs.patcherLog
          if (log) {
            log.scrollTop = log.scrollHeight
          }
        })
        clearTimeout(this.patcherTimer)
        if (r.data.running) {
          this.patcherTimer = setTimeout(() => this.loadPatcher(), 2000)
        }
      } catch (e) {
        this.patcher = null // no Patcher Build permission: the panel is not shown
      }
    },
    async savePatcherSettings() {
      try {
        const r              = await SpireApi.v1().put("patcher-build/settings", this.patcherSettings)
        this.patcher         = r.data
        this.patcherSettings = Object.assign({}, r.data.settings)
        this.notification    = "Patcher settings saved"
      } catch (e) {
        this.showError(e)
      }
    },
    async buildPatcher() {
      this.error = ""
      try {
        await SpireApi.v1().post("patcher-build/build")
      } catch (e) {
        this.showError(e)
      }
      await this.loadPatcher()
    },
    async uploadPatcher(e) {
      const f = e.target.files[0]
      e.target.value = ""
      if (!f) {
        return
      }
      try {
        const form = new FormData()
        form.append("file", f, f.name)
        await SpireApi.v1().post("patcher-build/upload", form)
        this.notification = f.name + " staged"
      } catch (err) {
        this.showError(err)
      }
      await this.loadPatcher()
    },
    downloadStaged() {
      const q = SpireApi.getAccessTokenQueryString()
      window.open(SpireApi.getBaseV1Path() + "/patcher-build/staged.exe" + (q.jwt ? "?jwt=" + encodeURIComponent(q.jwt) : ""), "_blank")
    },
    async releasePatcher() {
      const s = this.patcher.staged
      if (!confirm("Release patcher " + s.version + "? Every player's patcher updates itself to it the next time it runs. Try it first.")) {
        return
      }
      try {
        await SpireApi.v1().post("patcher-build/release")
        this.notification = "Patcher " + s.version + " released"
      } catch (e) {
        this.showError(e)
      }
      await this.loadPatcher()
    },

    async load() {
      try {
        const [contents, published] = await Promise.all([
          SpireApi.v1().get("client-files/patch/contents"),
          SpireApi.v1().get("client-files/patch/published"),
        ])
        this.contents  = contents.data
        this.published = published.data
      } catch (e) {
        this.showError(e)
      }
    },
    publishedSize() {
      return this.published.manifest.files.reduce((t, f) => t + f.size, 0)
    },
    async publish() {
      this.publishing    = true
      this.publishResult = null
      this.error         = ""
      try {
        const r            = await SpireApi.v1().post("client-files/patch/publish")
        this.publishResult = r.data
        ;(r.data.manifest.notes || []).forEach((n) => (this.notification = n))
      } catch (e) {
        this.showError(e)
      }
      this.publishing = false
      await this.load()
    },
    changeLists() {
      const d   = this.publishResult.diff
      const out = {}
      for (const kind of ["added", "changed", "removed"]) {
        if (d[kind].length) {
          out[kind] = d[kind]
        }
      }
      return out
    },
    nothingChanged() {
      const d = this.publishResult.diff
      return !d.added.length && !d.changed.length && !d.removed.length
    },
    skipped(r) {
      return r.error && r.error.includes("built from the database")
    },
    resultClass(r) {
      return this.skipped(r) ? "text-muted" : r.error ? "text-danger" : "text-success"
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
    // a client folder: each file keeps its place inside the chosen folder
    onPickFolder(e) {
      const files = Array.from(e.target.files || [])
      const paths = files.map((f) => (f.webkitRelativePath || f.name).split("/").slice(1).join("/"))
      this.upload(files, paths)
      e.target.value = ""
    },
    async upload(fileList, paths) {
      const files = Array.from(fileList || [])
      if (files.length === 0) {
        return
      }
      this.uploading = true
      this.results   = []
      this.error     = ""
      try {
        const form = new FormData()
        files.forEach((f, i) => {
          form.append("files", f, f.name)
          if (paths) {
            form.append("paths", paths[i])
          }
        })
        if (!paths && this.folder.trim()) {
          form.append("folder", this.folder.trim())
        }
        const r      = await SpireApi.v1().post("client-assets/upload", form)
        this.results = r.data.results || []
      } catch (e) {
        this.showError(e)
      }
      this.uploading = false
      await this.load()
    },
    async remove(f) {
      if (!confirm("Leave " + f.path + " out of the client patch? Spire forgets its copy. Players keep theirs.")) {
        return
      }
      try {
        await SpireApi.v1().delete("client-assets/client-file/" + f.path.split("/").map(encodeURIComponent).join("/"))
        this.notification = f.path + " removed"
      } catch (e) {
        this.showError(e)
      }
      await this.load()
    },
  },
  mounted() {
    this.load()
    this.loadPatcher()
  },
  beforeDestroy() {
    clearTimeout(this.patcherTimer)
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

.client-patch-publish {
  border: 1px solid rgba(40, 167, 69, .45);
  border-radius: 8px;
  padding: 12px;
}

.patcher-label {
  min-width: 120px;
}

.patcher-log {
  max-height: 260px;
  overflow: auto;
  background: rgba(0, 0, 0, .45);
  color: #ccc;
  padding: 8px;
  font-size: 12px;
  white-space: pre-wrap;
}

.client-patch-files td {
  vertical-align: middle;
}
</style>
