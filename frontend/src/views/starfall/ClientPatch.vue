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

      <!-- the channels: beta is published from here, live is promoted from beta -->
      <div class="client-patch-channels mt-3" v-if="published">
        <div class="client-patch-publish client-patch-beta-panel">
          <div class="d-flex align-items-center flex-wrap">
            <b class="mr-3">Beta</b>
            <b-button variant="info" class="mr-3" @click="publish('beta')" :disabled="publishing !== ''">
              <i :class="'fa ' + (publishing === 'beta' ? 'fa-cog fa-spin' : 'fa-paper-plane') + ' mr-1'"></i>
              {{ publishing === "beta" ? "Publishing..." : "Publish to Beta" }}
            </b-button>
            <span v-if="published.beta.manifest">
              <b>{{ published.beta.manifest.version.slice(0, 8) }}</b> {{ when(published.beta.manifest.published) }}:
              {{ published.beta.manifest.files.length }} files, {{ size(manifestSize(published.beta.manifest)) }}
              <span v-if="betaOnlyCount()">({{ betaOnlyCount() }} beta only)</span>
            </span>
            <span v-else class="text-muted">Nothing published yet.</span>
          </div>
          <div class="mt-2 text-muted client-patch-hint">
            Testers' patchers read <a :href="published.beta.filelist_url + 'rof/filelist_rof.yml'" target="_blank">{{ published.beta.filelist_url }}</a>.
            Everything set to Add or Beta goes here.
          </div>
          <publish-result v-if="publishResult && publishResult.channel === 'beta'" :diff="publishResult.diff"/>
        </div>

        <div class="client-patch-publish">
          <div class="d-flex align-items-center flex-wrap">
            <b class="mr-3">Live</b>
            <b-button variant="success" class="mr-3" @click="previewPromote()" :disabled="publishing !== '' || !published.beta.manifest">
              <i :class="'fa ' + (publishing === 'live' ? 'fa-cog fa-spin' : 'fa-level-up') + ' mr-1'"></i>
              {{ publishing === "live" ? "Promoting..." : "Promote Beta to Live" }}
            </b-button>
            <span v-if="published.live.manifest">
              <b>{{ published.live.manifest.version.slice(0, 8) }}</b> {{ when(published.live.manifest.published) }}:
              {{ published.live.manifest.files.length }} files, {{ size(manifestSize(published.live.manifest)) }}
              <span v-if="published.live.manifest.promoted" class="text-muted">
                (beta {{ published.live.manifest.promoted.slice(0, 8) }})
              </span>
            </span>
            <span v-else class="text-muted">Nothing published yet.</span>
          </div>
          <div class="mt-2 text-muted client-patch-hint">
            Every player's patcher reads <a :href="published.live.filelist_url + 'rof/filelist_rof.yml'" target="_blank">{{ published.live.filelist_url }}</a>.
            Promoting sends players exactly what Beta has published, without the Beta only files.
            <a href="#" @click.prevent="publishLive()" class="ml-1">Publish straight to Live</a>
            (skips beta: for urgent fixes).
          </div>

          <!-- what promoting would do, before it is done -->
          <div v-if="promotePreview" class="client-patch-preview mt-2">
            <publish-result :diff="promotePreview.diff" empty="Live already has what Beta has published."/>
            <div v-if="promotePreview.left_out.length">
              <span class="text-info">beta only, left out ({{ promotePreview.left_out.length }}):</span>
              <code v-for="f in promotePreview.left_out" :key="f" class="mr-2">{{ f }}</code>
            </div>
            <div class="mt-2">
              <b-button size="sm" variant="success" class="mr-2" @click="promote()" :disabled="changeCount(promotePreview.diff) === 0">
                Promote beta {{ promotePreview.beta.version.slice(0, 8) }} to every player
              </b-button>
              <b-button size="sm" variant="outline-secondary" @click="promotePreview = null">Cancel</b-button>
            </div>
          </div>
          <publish-result v-if="publishResult && publishResult.channel === 'live'" :diff="publishResult.diff"/>
        </div>
      </div>

      <!-- the patcher programs: built here, and what players' patchers update to -->
      <div class="client-patch-publish mt-3" v-if="patcher">
        <div class="d-flex align-items-center flex-wrap">
          <b class="mr-3">Patcher program</b>
          <div class="btn-group btn-group-sm mr-3">
            <b-button v-for="ch in ['live', 'beta']" :key="ch" size="sm"
                      :variant="patcherChannel === ch ? (ch === 'beta' ? 'info' : 'success') : 'outline-secondary'"
                      @click="patcherChannel = ch">
              {{ ch === "beta" ? "Beta" : "Live" }}
            </b-button>
          </div>
          <span v-if="pc().released" class="mr-3">
            {{ patcherChannel === "beta" ? "Testers" : "Players" }} update to <b>{{ pc().released.version }}</b>
            (<code>{{ pc().released.file_name }}.exe</code>, {{ when(pc().released.built) }})
          </span>
          <span v-else class="mr-3 text-muted">None released: {{ patcherChannel === "beta" ? "testers" : "players" }}' patchers keep the one they have.</span>
        </div>
        <div v-if="patcherChannel === 'beta'" class="mt-2 text-info client-patch-hint">
          <i class="fa fa-info-circle mr-1"></i>
          Testers run the beta patcher in a <b>copy of the client folder</b> of its own. The live patcher puts back the
          files both channels send, but leaves Beta only files where it finds them.
        </div>

        <div class="mt-2 patcher-settings">
          <div class="d-flex align-items-center mb-1">
            <span class="patcher-label">Source folder</span>
            <b-form-input size="sm" v-model="ps().source_dir" placeholder="/home/eqemu/starfall-patcher (a checkout with build.sh)"/>
          </div>
          <div class="d-flex align-items-center mb-1">
            <span class="patcher-label">Public address</span>
            <b-form-input size="sm" v-model="ps().public_url" :placeholder="patcher.suggested_urls[patcherChannel]"/>
            <b-button size="sm" variant="outline-secondary" class="ml-2 text-nowrap" @click="ps().public_url = patcher.suggested_urls[patcherChannel]">
              This Spire
            </b-button>
          </div>
          <div class="d-flex align-items-center mb-1">
            <span class="patcher-label">Server name</span>
            <b-form-input size="sm" v-model="ps().server_name" style="max-width: 280px" class="mr-3"/>
            <span class="mr-2 text-nowrap">File name</span>
            <b-form-input size="sm" v-model="ps().file_name" style="max-width: 200px"/>
            <span class="ml-1">.exe</span>
          </div>
          <div class="d-flex align-items-center">
            <b-form-checkbox v-model="ps().git_pull" class="mr-3">git pull before building</b-form-checkbox>
            <b-button size="sm" variant="outline-warning" @click="savePatcherSettings()">Save {{ patcherChannel }} settings</b-button>
          </div>
          <div class="text-muted mt-1" style="font-size: 13px">
            The public address is baked into the patcher: it reads the file list there and updates itself from it, so it
            must be how players reach this Spire, not how you do.
            <span v-if="patcherChannel === 'beta'">The beta patcher's ends in <code>/patch/beta/</code>, and its file name differs from live's.</span>
          </div>
        </div>

        <div class="mt-2 d-flex align-items-center flex-wrap">
          <b-button variant="outline-success" class="mr-2" @click="buildPatcher()" :disabled="patcher.running || !patcher.dotnet">
            <i :class="'fa ' + (patcher.running ? 'fa-cog fa-spin' : 'fa-wrench') + ' mr-1'"></i>
            {{ patcher.running ? "Building the " + patcher.building + " patcher..." : "Build " + patcherChannel + " patcher" }}
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

        <div v-if="!published[patcherChannel].manifest" class="mt-2 text-warning">
          <i class="fa fa-exclamation-triangle mr-1"></i>
          Nothing is published to {{ patcherChannel }} yet: its patcher stops with "Failed to fetch filelist" until you
          <b>{{ patcherChannel === "beta" ? "Publish to Beta" : "Promote Beta to Live" }}</b> above.
        </div>
        <div v-if="pc().staged" class="mt-2 d-flex align-items-center flex-wrap">
          <span class="mr-3">
            Staged <b>{{ pc().staged.version }}</b> ({{ pc().staged.source }} {{ when(pc().staged.built) }},
            reads <code>{{ pc().staged.public_url }}</code>)
          </span>
          <b-button size="sm" variant="outline-secondary" class="mr-2" @click="downloadStaged()">
            <i class="fa fa-download mr-1"></i> Download to try
          </b-button>
          <b-button size="sm" variant="success" @click="releasePatcher()"
                    :disabled="pc().released && pc().released.md5 === pc().staged.md5">
            <i class="fa fa-paper-plane mr-1"></i> Release to {{ patcherChannel === "beta" ? "testers" : "players" }}
          </b-button>
        </div>

        <div v-if="patcher.last_error" class="mt-2 text-danger">{{ patcher.building }} patcher: {{ patcher.last_error }}</div>
        <pre v-if="patcher.log" ref="patcherLog" class="patcher-log mt-2">{{ patcher.log }}</pre>
      </div>

      <div class="mt-3 d-flex align-items-center flex-wrap">
        <b-button variant="warning" class="mr-2" @click="download()" :disabled="!contents">
          <i class="fa fa-cloud-download mr-1"></i> Download client patch
        </b-button>
        <b-form-select size="sm" v-model="downloadChannel" style="width: 110px" class="mr-3"
                       :options="[{ value: 'live', text: 'as live' }, { value: 'beta', text: 'as beta' }]"/>
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
      <div class="mt-2 d-flex align-items-center">
        <span class="mr-2">Send uploads to</span>
        <b-form-select size="sm" v-model="hold" :options="holds" style="max-width: 260px" class="mr-2"/>
        <span class="text-muted" style="font-size: 13px">Spire keeps and uses them either way</span>
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
      <div class="mt-2 d-flex align-items-center flex-wrap">
        <b-button size="sm" variant="outline-warning" class="mr-3" @click="$refs.zipPicker.click()" :disabled="uploading">
          <i class="fa fa-file-archive-o mr-1"></i> Import from a .zip
        </b-button>
        <input ref="zipPicker" type="file" accept=".zip" style="display: none" @change="onPickZip">
        <span class="mr-2">Remove from players</span>
        <b-form-input size="sm" v-model="removalPath" placeholder="e.g. IT9001.eqg or uifiles/default/EQUI_Old.xml"
                      style="max-width: 360px" class="mr-2" @keyup.enter="addRemoval()"/>
        <b-button size="sm" variant="outline-danger" @click="addRemoval()" :disabled="!removalPath.trim()">Remove if found</b-button>
      </div>

      <!-- a staged zip: pick what to add, or to remove from players -->
      <div v-if="zip" class="client-patch-zip mt-3">
        <div class="d-flex align-items-center mb-2">
          <b class="mr-2">{{ zip.name }}</b>
          <span class="text-muted mr-auto">{{ zip.entries.length }} files: choose what each does</span>
          <b-button size="sm" variant="outline-success" class="mr-2" @click="importZip()" :disabled="!zipPicks().length || uploading">
            Import {{ zipPicks().length }} selected
          </b-button>
          <b-button size="sm" variant="outline-secondary" @click="zip = null">Close</b-button>
        </div>
        <table class="eq-table eq-highlight-rows client-patch-files">
          <thead>
          <tr>
            <th>In the zip</th>
            <th>Do</th>
            <th>Goes to folder</th>
            <th>What it is</th>
          </tr>
          </thead>
          <tbody>
          <tr v-for="e in zip.entries" :key="e.entry" :class="e.kind === 'refused' ? 'text-muted' : ''">
            <td><code>{{ e.path }}</code> <span class="text-muted">{{ size(e.size) }}</span></td>
            <td>
              <b-form-select v-if="e.kind !== 'refused'" size="sm" v-model="e.action" :options="zipActions" style="width: 190px"/>
              <span v-else>&mdash;</span>
            </td>
            <td>
              <b-form-input v-if="e.kind !== 'refused'" size="sm" v-model="e.folder"
                            :placeholder="e.needs_folder ? 'choose: / for next to eqgame.exe' : '(next to eqgame.exe)'"
                            :state="e.action === 'add' && e.needs_folder && !e.folderSet ? false : null"
                            @input="e.folderSet = true" style="max-width: 240px"/>
            </td>
            <td>
              <b-badge :variant="kindVariant(e.kind)" class="mr-1">{{ e.kind }}</b-badge>
              <span :class="e.kind === 'refused' ? 'text-warning' : ''">{{ e.note }}</span>
            </td>
          </tr>
          </tbody>
        </table>
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
          <th>Players</th>
          <th>From</th>
          <th class="text-right">Size</th>
          <th>Updated</th>
          <th></th>
        </tr>
        </thead>
        <tbody>
        <tr v-for="f in contents.files" :key="f.path"
            :class="f.remove ? 'client-patch-removed' : f.ignore ? 'client-patch-ignored' : f.beta ? 'client-patch-beta' : ''">
          <td><code>{{ f.path }}</code></td>
          <td class="text-nowrap">
            <span v-if="f.kind === 'database'" class="text-muted">add (from the database)</span>
            <div v-else class="btn-group btn-group-sm">
              <b-button size="sm" :variant="mode(f) === 'add' ? 'success' : 'outline-secondary'" :disabled="f.kind === 'removal'"
                        @click="setMode(f, 'add')" title="Send it to players, live and beta">Add</b-button>
              <b-button size="sm" :variant="mode(f) === 'beta' ? 'info' : 'outline-secondary'" :disabled="f.kind === 'removal'"
                        @click="setMode(f, 'beta')" title="Send it to the beta channel only; promoting to live leaves it out">Beta</b-button>
              <b-button size="sm" :variant="mode(f) === 'ignore' ? 'warning' : 'outline-secondary'" :disabled="f.kind === 'removal'"
                        @click="setMode(f, 'ignore')" title="Keep it in Spire only: neither sent nor deleted, on live or beta">Ignore</b-button>
              <b-button size="sm" :variant="mode(f) === 'remove' ? 'danger' : 'outline-secondary'"
                        @click="setMode(f, 'remove')" title="Delete it from players' clients if they have it, live and beta">Remove</b-button>
            </div>
          </td>
          <td>{{ f.source }}</td>
          <td class="text-right">{{ f.size ? size(f.size) : "" }}</td>
          <td>{{ when(f.updated) }}</td>
          <td>
            <b-button v-if="f.kind === 'file' || f.kind === 'removal'" size="sm" variant="outline-danger"
                      @click="remove(f)" :title="f.kind === 'removal' ? 'Stop removing it from players' : 'Forget it: neither sent nor removed'">
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
import PublishResult, {changeCount} from "./PublishResult";

export default {
  name: "StarfallClientPatch",
  components: { EqWindowSimple, InfoErrorBanner, PublishResult },
  data() {
    return {
      contents: null,
      patcher: null,         // null when this user may not build the patcher
      patcherChannel: "live",
      patcherSettings: { live: {}, beta: {} }, // edited per channel
      patcherTimer: null,
      published: null,     // { live: {manifest, filelist_url}, beta: ... }
      publishing: "",      // the channel being published to
      publishResult: null,
      promotePreview: null,
      downloadChannel: "live",
      folder: "",
      zip: null,
      removalPath: "",
      zipActions: [
        { value: "add", text: "Add" },
        { value: "beta", text: "Add, beta only" },
        { value: "ignore", text: "Add, don't send (Ignore)" },
        { value: "remove", text: "Remove if found" },
        { value: "", text: "Leave out" },
      ],
      hold: "", // how uploads are held back from players: "", "beta" or "ignore"
      holds: [
        { value: "", text: "Live and beta" },
        { value: "beta", text: "Beta only" },
        { value: "ignore", text: "Neither (Ignore)" },
      ],
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
    // the shown channel's patcher, and the settings being edited for it
    pc() {
      return this.patcher.channels[this.patcherChannel]
    },
    ps() {
      return this.patcherSettings[this.patcherChannel]
    },
    async loadPatcher() {
      try {
        const r      = await SpireApi.v1().get("patcher-build")
        const first  = !this.patcher
        this.patcher = r.data
        if (first) {
          this.patcherSettings = {
            live: Object.assign({}, r.data.channels.live.settings),
            beta: Object.assign({}, r.data.channels.beta.settings),
          }
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
    patcherPath(action) {
      return "patcher-build/" + action + "?channel=" + this.patcherChannel
    },
    async savePatcherSettings() {
      const ch = this.patcherChannel
      try {
        const r                  = await SpireApi.v1().put(this.patcherPath("settings"), this.ps())
        this.patcher             = r.data
        this.patcherSettings[ch] = Object.assign({}, r.data.channels[ch].settings)
        this.notification        = "The " + ch + " patcher's settings are saved"
      } catch (e) {
        this.showError(e)
      }
    },
    async buildPatcher() {
      this.error = ""
      try {
        await SpireApi.v1().post(this.patcherPath("build"))
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
        await SpireApi.v1().post(this.patcherPath("upload"), form)
        this.notification = f.name + " staged as the " + this.patcherChannel + " patcher"
      } catch (err) {
        this.showError(err)
      }
      await this.loadPatcher()
    },
    downloadStaged() {
      const q = SpireApi.getAccessTokenQueryString()
      window.open(SpireApi.getBaseV1Path() + "/" + this.patcherPath("staged.exe") + (q.jwt ? "&jwt=" + encodeURIComponent(q.jwt) : ""), "_blank")
    },
    async releasePatcher() {
      const s   = this.pc().staged
      const who = this.patcherChannel === "beta" ? "tester's beta patcher" : "player's patcher"
      if (!confirm("Release the " + this.patcherChannel + " patcher " + s.version + "? Every " + who + " updates itself to it the next time it runs. Try it first.")) {
        return
      }
      try {
        await SpireApi.v1().post(this.patcherPath("release"))
        this.notification = "The " + this.patcherChannel + " patcher " + s.version + " is released"
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
    manifestSize(m) {
      return m.files.reduce((t, f) => t + f.size, 0)
    },
    betaOnlyCount() {
      return this.published.beta.manifest.files.filter((f) => f.beta).length
    },
    changeCount,
    async publish(channel) {
      this.publishing     = channel
      this.publishResult  = null
      this.promotePreview = null
      this.error          = ""
      try {
        const r            = await SpireApi.v1().post("client-files/patch/publish?channel=" + channel)
        this.publishResult = r.data
        ;(r.data.manifest.notes || []).forEach((n) => (this.notification = n))
      } catch (e) {
        this.showError(e)
      }
      this.publishing = ""
      await this.load()
    },
    publishLive() {
      if (confirm("Publish straight to Live? Every player gets what Spire holds now (Beta only files left out), without it being tried on beta first.")) {
        this.publish("live")
      }
    },
    async previewPromote() {
      this.error         = ""
      this.publishResult = null
      try {
        const r             = await SpireApi.v1().get("client-files/patch/promote")
        this.promotePreview = r.data
      } catch (e) {
        this.showError(e)
      }
    },
    async promote() {
      this.publishing = "live"
      this.error      = ""
      try {
        const r             = await SpireApi.v1().post("client-files/patch/promote")
        this.publishResult  = r.data
        this.promotePreview = null
      } catch (e) {
        this.showError(e)
      }
      this.publishing = ""
      await this.load()
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
      window.open(SpireApi.getBaseV1Path() + "/client-files/patch?channel=" + this.downloadChannel + (q.jwt ? "&jwt=" + encodeURIComponent(q.jwt) : ""), "_blank")
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
        if (this.hold) {
          form.append(this.hold, "1")
        }
        const r      = await SpireApi.v1().post("client-assets/upload", form)
        this.results = r.data.results || []
      } catch (e) {
        this.showError(e)
      }
      this.uploading = false
      await this.load()
    },
    mode(f) {
      return f.remove ? "remove" : f.ignore ? "ignore" : f.beta ? "beta" : "add"
    },
    async setMode(f, mode) {
      if (this.mode(f) === mode) {
        return
      }
      if (mode === "remove" && !confirm("Remove " + f.path + " from players? The next publish deletes it from their clients if they have it (beta first, then live when you promote).")) {
        return
      }
      try {
        await SpireApi.v1().put("client-assets/mode", { path: f.path, mode: mode })
      } catch (e) {
        this.showError(e)
      }
      await this.load()
    },
    async addRemoval() {
      const path = this.removalPath.trim()
      if (!path || !confirm("Delete " + path + " from players' clients if they have it, at the next publish?")) {
        return
      }
      try {
        const r           = await SpireApi.v1().post("client-assets/removal", { path: path })
        this.notification = r.data.result
        this.removalPath  = ""
      } catch (e) {
        this.showError(e)
      }
      await this.load()
    },

    // ------------------------------------------------------------ zip import
    async onPickZip(e) {
      const f = e.target.files[0]
      e.target.value = ""
      if (!f) {
        return
      }
      this.uploading = true
      this.error     = ""
      this.results   = []
      try {
        const form = new FormData()
        form.append("file", f, f.name)
        const r = await SpireApi.v1().post("client-assets/zip", form)
        r.data.entries.forEach((e) => {
          e.action    = e.kind === "refused" || e.needs_folder ? "" : this.hold || "add"
          e.folderSet = false
        })
        this.zip = r.data
      } catch (err) {
        this.showError(err)
      }
      this.uploading = false
    },
    kindVariant(kind) {
      return { models: "info", sheet: "primary", file: "secondary", refused: "warning" }[kind] || "secondary"
    },
    zipPicks() {
      if (!this.zip) {
        return []
      }
      return this.zip.entries.filter((e) => e.action && e.kind !== "refused").map((e) => ({
        entry: e.entry,
        // "/" chooses next to eqgame.exe
        folder: (e.folder || "").trim() === "/" ? "" : (e.folder || "").trim(),
        remove: e.action === "remove",
        ignore: e.action === "ignore",
        beta: e.action === "beta",
      }))
    },
    async importZip() {
      const missing = this.zip.entries.filter((e) => e.action === "add" && e.needs_folder && !e.folderSet)
      if (missing.length) {
        this.error = "Choose a folder for " + missing.map((e) => e.path).join(", ") + " (/ for next to eqgame.exe)"
        return
      }
      this.uploading = true
      this.error     = ""
      try {
        const r      = await SpireApi.v1().post("client-assets/zip/" + this.zip.id + "/import", { picks: this.zipPicks() })
        this.results = r.data.results
        if (!r.data.results.some((x) => x.error)) {
          this.zip = null
        }
      } catch (e) {
        this.showError(e)
      }
      this.uploading = false
      await this.load()
    },

    async remove(f) {
      const what = f.kind === "removal"
        ? "Stop deleting " + f.path + " from players' clients?"
        : "Forget " + f.path + "? Spire drops its copy, and it is neither sent nor deleted: players keep theirs."
      if (!confirm(what)) {
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

.client-patch-channels {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(420px, 1fr));
  gap: 12px;
}

.client-patch-beta-panel {
  border-color: rgba(23, 162, 184, .55);
}

.client-patch-hint {
  font-size: 13px;
}

.client-patch-preview {
  border-top: 1px solid rgba(255, 255, 255, .15);
  padding-top: 8px;
}

.client-patch-beta code {
  color: #17a2b8;
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

.client-patch-zip {
  border: 1px solid rgba(255, 193, 7, .45);
  border-radius: 8px;
  padding: 12px;
}

.client-patch-ignored td {
  opacity: .6;
}

.client-patch-removed code {
  text-decoration: line-through;
  color: #dc3545;
}

.client-patch-files td {
  vertical-align: middle;
}
</style>
