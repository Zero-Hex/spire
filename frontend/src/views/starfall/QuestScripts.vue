<template>
  <div>
    <div class="row">
      <!-- the server's quests folder -->
      <div class="col-lg-3 col-sm-12">
        <eq-window-simple title="Quests">
          <div class="quest-crumbs mb-2">
            <a href="#" @click.prevent="openDir('')">quests</a>
            <span v-for="c in crumbs()" :key="c.path">
              / <a href="#" @click.prevent="openDir(c.path)">{{ c.name }}</a>
            </span>
          </div>
          <b-form-input size="sm" v-model="filter" placeholder="Filter this folder" class="mb-2"/>

          <div class="quest-list">
            <div v-if="listing.dir" class="quest-entry" @click="openDir(parentDir())">
              <i class="fa fa-level-up mr-1"></i> ..
            </div>
            <div
              v-for="d in filtered(listing.dirs)"
              :key="'d' + d.path"
              class="quest-entry"
              @click="openDir(d.path)"
            >
              <i class="fa fa-folder mr-1"></i> {{ d.name }}
            </div>
            <div
              v-for="f in filtered(listing.files)"
              :key="'f' + f.path"
              :class="'quest-entry ' + (file && file.path === f.path ? 'quest-entry-open' : '')"
              @click="openFile(f.path)"
            >
              <i :class="'fa ' + (f.name.toLowerCase().endsWith('.lua') ? 'fa-moon-o' : 'fa-file-code-o') + ' mr-1'"></i> {{ f.name }}
            </div>
            <div v-if="listing.dirs.length + listing.files.length === 0" class="text-muted">Empty folder</div>
          </div>

          <div class="d-flex mt-2" v-if="listing.dir">
            <b-form-input size="sm" v-model="newName" placeholder="New_NPC_Name.pl" class="mr-2"/>
            <b-button size="sm" variant="outline-warning" @click="newFile()">New</b-button>
          </div>
        </eq-window-simple>
      </div>

      <!-- the open script -->
      <div class="col-lg-9 col-sm-12">
        <eq-window-simple :title="file ? file.path : 'Quest Scripts'">
          <div v-if="!file" class="text-muted">
            Open a script from the quests folder. The <b>Window Forge</b> tab edits its Script Windows and
            [keyword] dialogue; the <b>Script</b> tab edits the text. Saving keeps a backup of the old file, and Spire's
            quest hot reload (Admin, Quest Hot Reload) puts the change in game.
          </div>

          <div v-if="file">
            <div class="d-flex align-items-center flex-wrap">
              <div class="btn-group mr-3">
                <b-button size="sm" :variant="mode === 'forge' ? 'warning' : 'outline-warning'" @click="setMode('forge')">
                  Window Forge
                </b-button>
                <b-button size="sm" :variant="mode === 'text' ? 'warning' : 'outline-warning'" @click="setMode('text')">
                  Script
                </b-button>
              </div>
              <b-button size="sm" variant="outline-success" class="mr-2" @click="save()" :disabled="saving">
                <i class="fa fa-save"></i> {{ saveLabel() }}
              </b-button>
              <b-button size="sm" variant="outline-secondary" class="mr-2" @click="reload()">
                <i class="fa fa-refresh"></i> Reload from disk
              </b-button>
              <b-dropdown size="sm" variant="outline-secondary" text="Backups" class="mr-2" @show="loadBackups()">
                <b-dropdown-item v-if="backups.length === 0" disabled>No backups yet</b-dropdown-item>
                <b-dropdown-item v-for="b in backups" :key="b" @click="openBackup(b)">{{ b }}</b-dropdown-item>
              </b-dropdown>
              <span v-if="dirty()" class="text-warning">Unsaved changes</span>
            </div>

            <div v-if="conflict" class="mt-2 text-warning">
              {{ file.path }} changed on disk since you opened it.
              <b-button size="sm" variant="outline-danger" class="ml-2" @click="save(true)">Overwrite it</b-button>
              <b-button size="sm" variant="outline-secondary" class="ml-1" @click="reload()">Load the disk version</b-button>
            </div>

            <info-error-banner
              class="mt-2"
              :notification="notification"
              :error="error"
              @dismiss-error="error = ''"
              @dismiss-notification="notification = ''"
            />
          </div>
        </eq-window-simple>

        <!-- the Forge stays loaded while a file is open; Script hides it -->
        <iframe
          v-show="file && mode === 'forge'"
          ref="forge"
          :src="forgeUrl"
          title="Window Forge"
          class="quest-forge mt-2"
          @load="onForgeLoad"
        />

        <div v-show="file && mode === 'text'" class="mt-2">
          <editor
            ref="ace"
            v-model="text"
            @init="aceInit"
            :lang="lang()"
            theme="terminal"
            width="100%"
            height="80vh"
          />
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import EqWindowSimple  from "@/components/eq-ui/EQWindowSimple";
import InfoErrorBanner from "@/components/InfoErrorBanner";
import {SpireApi}      from "@/app/api/spire-api";

const sleep = (ms) => new Promise((r) => setTimeout(r, ms))

export default {
  name: "StarfallQuestScripts",
  components: { EqWindowSimple, InfoErrorBanner, editor: require("vue2-ace-editor") },
  data() {
    return {
      listing: { dir: "", dirs: [], files: [] },
      filter: "",
      newName: "",

      file: null,         // { path, content, modified } as on disk
      mode: "forge",
      text: "",           // the Script tab
      textBaseline: "",   // what the Script tab held when last loaded or saved
      forgeBaseline: "",  // the Forge's script right after import or save
      forgeOutput: "",    // the Forge's script now
      forgeOk: false,     // the open script imported into the Forge
      forgeLoaded: false,
      isNew: false,

      backups: [],
      saving: false,
      conflict: false,
      notification: "",
      error: "",
    }
  },
  computed: {
    forgeUrl() {
      const q = SpireApi.getAccessTokenQueryString()
      return SpireApi.getBaseV1Path() + "/quest-files/window-forge" + (q.jwt ? "?jwt=" + encodeURIComponent(q.jwt) : "")
    },
  },
  methods: {
    showError(e) {
      this.error = e && e.response && e.response.data && e.response.data.error ? e.response.data.error : String(e && e.message || e)
    },
    isLua(path) {
      return (path || "").toLowerCase().endsWith(".lua")
    },
    lang() {
      return this.file && this.isLua(this.file.path) ? "lua" : "perl"
    },

    // ------------------------------------------------------------ folders
    crumbs() {
      const parts = (this.listing.dir || "").split("/").filter((p) => p)
      return parts.map((name, i) => ({ name: name, path: parts.slice(0, i + 1).join("/") }))
    },
    parentDir() {
      return (this.listing.dir || "").split("/").slice(0, -1).join("/")
    },
    filtered(list) {
      const f = this.filter.trim().toLowerCase()
      return f ? list.filter((e) => e.name.toLowerCase().includes(f)) : list
    },
    async openDir(dir) {
      try {
        const r      = await SpireApi.v1().get("quest-files/list", { params: { dir: dir } })
        this.listing = r.data
        this.filter  = ""
      } catch (e) {
        this.showError(e)
      }
    },

    // ------------------------------------------------------------ the Forge
    async forgeWindow() {
      for (let i = 0; i < 100; i++) {
        const w = this.$refs.forge && this.$refs.forge.contentWindow
        try {
          if (this.forgeLoaded && w && w.document.getElementById("importBtn")) {
            return w
          }
        } catch (e) {
          throw new Error("The Window Forge must be served from the same address as Spire")
        }
        await sleep(100)
      }
      throw new Error("The Window Forge did not load")
    },
    // Spire is dark; the Forge otherwise follows the system's light/dark setting
    onForgeLoad() {
      this.forgeLoaded = true
      try {
        this.$refs.forge.contentWindow.document.documentElement.setAttribute("data-theme", "dark")
      } catch (e) {
        // another origin (dev server): leave it as it is
      }
    },
    readForge(w) {
      const out = w.document.getElementById("codeOut")
      return out ? out.textContent : ""
    },
    // loads a script into the Forge with its own Import; false when the Forge
    // cannot read it (no Script Window and no [keyword] dialogue)
    async importIntoForge(content) {
      const w = await this.forgeWindow()
      const d = w.document
      if (w.location.hash) {
        w.location.hash = "" // the editor, not the converter
      }

      d.getElementById("importText").value = content
      const confirm = w.confirm
      w.confirm     = () => true // replacing the Forge's project is what opening a file means
      try {
        d.getElementById("importBtn").click()
      } finally {
        w.confirm = confirm
      }

      const report = d.getElementById("importReport")
      const failed = report && report.querySelector(".err")
      if (failed) {
        return false
      }
      d.getElementById("importBox").hidden = true

      // keep the file's own form, raw $client->Window* calls or the plugin,
      // counting calls in code only (templates mention both in comments)
      const code   = content.split("\n").filter((l) => !/^\s*#/.test(l)).join("\n")
      const plugin = (code.match(/plugin::ShowScriptWindow\s*\(/g) || []).length
      const raw    = (code.match(/->\s*WindowBegin\s*\(/g) || []).length
      const form   = plugin > raw ? "plugin" : (raw > 0 ? "raw" : null)
      if (form) {
        const tab = d.querySelector('.tab[data-tab="' + form + '"]')
        if (tab) {
          tab.click()
        }
      }

      this.watchForge(w)
      this.forgeOutput   = this.readForge(w)
      this.forgeBaseline = this.forgeOutput
      return true
    },
    watchForge(w) {
      if (this.forgeObserver) {
        this.forgeObserver.disconnect()
      }
      const out          = w.document.getElementById("codeOut")
      this.forgeObserver = new w.MutationObserver(() => {
        this.forgeOutput = out.textContent
      })
      this.forgeObserver.observe(out, { childList: true, characterData: true, subtree: true })
    },

    // ------------------------------------------------------------ open
    dirty() {
      if (!this.file) {
        return false
      }
      if (this.isNew) {
        return true
      }
      return this.mode === "forge" && this.forgeOk ? this.forgeOutput !== this.forgeBaseline : this.text !== this.textBaseline
    },
    async openFile(path) {
      if (this.dirty() && !confirm("Discard the unsaved changes to " + this.file.path + "?")) {
        return
      }
      try {
        const r = await SpireApi.v1().get("quest-files/file", { params: { path: path } })
        await this.load(r.data, false)
      } catch (e) {
        this.showError(e)
      }
    },
    async load(script, isNew) {
      this.file         = script
      this.isNew        = isNew
      this.conflict     = false
      this.error        = ""
      this.notification = ""
      this.backups      = []
      this.text         = script.content
      this.textBaseline = script.content

      this.forgeOk = await this.importIntoForge(script.content).catch((e) => {
        this.showError(e)
        return false
      })
      if (this.forgeOk) {
        this.mode = "forge"
        if (this.isLua(script.path)) {
          this.notification = "The Forge converted this Lua dialogue to Perl. Saving from the Forge writes a .pl and moves the .lua to backups."
        }
      } else {
        this.mode = "text"
        this.$nextTick(() => this.$refs.ace.editor.resize())
        if (!isNew) {
          this.notification = "No Script Window or [keyword] dialogue for the Forge to edit in this script: it is open as text."
        }
      }
    },
    async newFile() {
      let name = this.newName.trim()
      if (!name) {
        return
      }
      if (!/\.(pl|lua)$/i.test(name)) {
        name += ".pl"
      }
      if (this.listing.files.find((f) => f.name.toLowerCase() === name.toLowerCase())) {
        this.error = name + " already exists"
        return
      }
      if (this.dirty() && !confirm("Discard the unsaved changes to " + this.file.path + "?")) {
        return
      }
      const path   = (this.listing.dir ? this.listing.dir + "/" : "") + name
      this.newName = ""
      await this.load({ path: path, content: this.isLua(name) ? "" : "sub EVENT_SAY {\n}\n", modified: "0" }, true)
    },
    async reload() {
      if (this.dirty() && !confirm("Discard the unsaved changes and load " + this.file.path + " from disk?")) {
        return
      }
      if (this.isNew) {
        return
      }
      await this.openFileForce(this.file.path)
    },
    async openFileForce(path) {
      try {
        const r = await SpireApi.v1().get("quest-files/file", { params: { path: path } })
        await this.load(r.data, false)
      } catch (e) {
        this.showError(e)
      }
    },

    // ------------------------------------------------------------ tabs
    // the two tabs edit the same script: switching carries the edits over
    async setMode(mode) {
      if (mode === this.mode) {
        return
      }
      if (mode === "text") {
        if (this.forgeOk && this.forgeOutput !== this.forgeBaseline) {
          this.text = this.forgeOutput
        }
        this.mode = "text"
        this.$nextTick(() => this.$refs.ace.editor.resize()) // Ace measures wrong while hidden
        return
      }

      if (this.text !== this.textBaseline || !this.forgeOk) {
        const ok = await this.importIntoForge(this.text).catch(() => false)
        if (!ok) {
          this.error = "The Forge cannot read this script: it needs a Script Window or classic [keyword] dialogue."
          return
        }
        this.forgeOk      = true
        this.textBaseline = this.text
      }
      this.mode = "forge"
    },

    // ------------------------------------------------------------ save
    saveTarget() {
      // the Forge writes Perl: a Lua script saved from it becomes a .pl
      if (this.mode === "forge" && this.forgeOk && this.isLua(this.file.path)) {
        return this.file.path.replace(/\.lua$/i, ".pl")
      }
      return this.file.path
    },
    saveLabel() {
      return this.saveTarget() !== this.file.path ? "Save as " + this.saveTarget().split("/").pop() : "Save"
    },
    async save(overwrite) {
      const content = this.mode === "forge" && this.forgeOk ? this.forgeOutput : this.text
      const target  = this.saveTarget()
      const retire  = target !== this.file.path ? this.file.path : ""
      if (retire && !confirm("Save as " + target + "? The zone loads a .lua ahead of a .pl, so " + this.file.path + " is moved to backups.")) {
        return
      }

      this.saving       = true
      this.error        = ""
      this.notification = ""
      try {
        // file times are strings: nanoseconds do not fit a JavaScript number
        let modified = this.isNew || retire ? "0" : this.file.modified
        if (overwrite) {
          const cur = await SpireApi.v1().get("quest-files/file", { params: { path: target } }).catch(() => null)
          modified  = cur ? cur.data.modified : "0"
        }

        const r       = await SpireApi.v1().put("quest-files/file", { path: target, content: content, modified: modified, retire: retire })
        this.file     = r.data
        this.isNew    = false
        this.conflict = false

        this.text          = r.data.content
        this.textBaseline  = r.data.content
        this.forgeBaseline = this.forgeOutput
        this.notification  = "Saved " + r.data.path + (retire ? "; " + retire + " moved to backups" : "")
        await this.openDir(this.listing.dir)
      } catch (e) {
        if (e.response && e.response.status === 409) {
          this.conflict = true
        } else {
          this.showError(e)
        }
      }
      this.saving = false
    },

    // ------------------------------------------------------------ backups
    async loadBackups() {
      if (!this.file || this.isNew) {
        this.backups = []
        return
      }
      try {
        const r      = await SpireApi.v1().get("quest-files/backups", { params: { path: this.file.path } })
        this.backups = r.data.backups || []
      } catch (e) {
        this.backups = []
      }
    },
    async openBackup(name) {
      try {
        const r   = await SpireApi.v1().get("quest-files/backup", { params: { path: this.file.path, name: name } })
        this.text = r.data.content
        this.mode = "text"
        this.notification = "Backup " + name + " is in the Script tab. Save to restore it."
      } catch (e) {
        this.showError(e)
      }
    },

    aceInit() {
      require("brace/ext/language_tools")
      require("brace/theme/terminal")
      require("brace/mode/lua")
      require("brace/mode/perl")
      this.$refs.ace.editor.setFontSize(14)
    },
    beforeUnload(e) {
      if (this.dirty()) {
        e.preventDefault()
        e.returnValue = ""
      }
    },
  },
  async mounted() {
    window.addEventListener("beforeunload", this.beforeUnload)
    await this.openDir("")
  },
  beforeDestroy() {
    window.removeEventListener("beforeunload", this.beforeUnload)
    if (this.forgeObserver) {
      this.forgeObserver.disconnect()
    }
  },
  beforeRouteLeave(to, from, next) {
    if (this.dirty() && !confirm("Leave without saving " + this.file.path + "?")) {
      return next(false)
    }
    next()
  },
}
</script>

<style scoped>
.quest-list {
  max-height: 70vh;
  overflow-y: auto;
}

.quest-entry {
  padding: 2px 6px;
  cursor: pointer;
  border-radius: 4px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.quest-entry:hover {
  background: rgba(255, 255, 255, .08);
}

.quest-entry-open {
  background: rgba(255, 193, 7, .15);
}

.quest-crumbs {
  word-break: break-all;
}

.quest-forge {
  width: 100%;
  height: 85vh;
  border: 1px solid rgba(255, 255, 255, .15);
  border-radius: 6px;
  background: #12141b;
}
</style>
