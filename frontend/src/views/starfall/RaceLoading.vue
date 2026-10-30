<template>
  <div>
    <eq-window-simple title="Race Loading">
      <div>
        Which race models the client loads: a zone's <code>&lt;zone&gt;_chr.txt</code> lists the models that zone loads,
        <code>Resources/GlobalLoad_chr.txt</code> models every zone loads, and <code>Resources/GlobalLoad.txt</code> the
        archives every zone loads. Files are edited where Spire keeps them for
        the client patch, and players get them when you publish on
        <router-link :to="clientPatchRoute">Client Patch</router-link>.
      </div>

      <div class="mt-3 d-flex align-items-center flex-wrap">
        <span class="mr-2">File</span>
        <b-form-select size="sm" v-model="path" :options="fileOptions()" style="max-width: 320px" class="mr-3"
                       @change="open(path)"/>
        <span class="mr-2">or a new zone file</span>
        <b-form-input size="sm" v-model="newZone" placeholder="zone short name" style="max-width: 170px" class="mr-2"
                      @keyup.enter="newFile()"/>
        <b-button size="sm" variant="outline-warning" class="mr-3" @click="newFile()" :disabled="!newZone.trim()">New</b-button>
        <b-button size="sm" variant="outline-secondary" @click="$refs.picker.click()">
          <i class="fa fa-upload mr-1"></i> Upload the client's copy
        </b-button>
        <input ref="picker" type="file" accept=".txt" multiple style="display: none" @change="upload">
      </div>
      <div class="text-muted mt-1 hint">
        Upload a zone's <code>_chr.txt</code>, <code>GlobalLoad_chr.txt</code> or <code>GlobalLoad.txt</code> from your client to edit it: Spire's copy
        replaces the player's whole file, so start from the real one.
      </div>
      <info-error-banner class="mt-2" :notification="notification" :error="error"
                         @dismiss-error="error = ''" @dismiss-notification="notification = ''"/>
    </eq-window-simple>

    <div class="row mt-3" v-if="file">
      <!-- the file -->
      <div class="col-lg-6">
        <eq-window-simple :title="file.path + (dirty ? ' (not saved)' : '')">
          <div class="d-flex align-items-center mb-2">
            <b-button size="sm" variant="success" class="mr-2" @click="save()" :disabled="!dirty || saving">
              <i :class="'fa ' + (saving ? 'fa-cog fa-spin' : 'fa-save') + ' mr-1'"></i> Save
            </b-button>
            <b-button size="sm" variant="outline-secondary" class="mr-3" @click="open(file.path)" :disabled="!dirty || saving">
              Undo changes
            </b-button>
            <span v-if="file.kind === 'chr'" class="text-muted">{{ entries().length }} race models</span>
            <span v-else class="text-muted">{{ summary() }}</span>
          </div>
          <div v-if="isGlobal()" class="text-warning hint mb-2">
            Every zone loads these. Removing a stock line takes those models out of every zone.
          </div>
          <div v-if="!held" class="text-warning hint mb-2">
            New file: the stock client <span v-if="stockZone()">has a {{ file.path }}, and this one replaces it,
            races and all</span><span v-else>has no file for this zone</span>.
          </div>
          <!-- models two archives both define: the one loaded first shows -->
          <div v-if="file.kind === 'globalload' && replaced().length" class="race-replaced mb-2">
            <b>Models with two versions</b>
            <span class="text-muted hint">
              Two archives here define the same models, and the one loaded first appears to win (inferred from the
              client's behaviour: check on Beta).
            </span>
            <div v-for="r in replaced()" :key="r.key" class="d-flex align-items-center mt-1 flex-wrap">
              <span class="mr-2">
                <b>{{ r.codes.map((c) => (raceName(c) || c) + " (" + c + ")").join(", ") }}</b>:
                <code>{{ r.first }}</code> <span class="text-success">shown</span>
                over <code>{{ r.second }}</code>
              </span>
              <b-button size="sm" variant="outline-warning" @click="swapVersions(r)"
                        :title="'Move ' + r.second + ' above ' + r.first + ' so its versions load first'">
                <i class="fa fa-exchange mr-1"></i> Use {{ r.second }}'s
              </b-button>
            </div>
          </div>
          <div v-if="file.kind === 'globalload' && hiddenCount()" class="hint mb-2">
            <a href="#" @click.prevent="showOther = !showOther">
              {{ showOther ? "Hide" : "Show" }} {{ hiddenCount() }} line(s) that load no races
            </a>
            <span class="text-muted">(equipment, objects, grass)</span>
          </div>
          <table class="eq-table eq-highlight-rows race-lines">
            <tbody>
            <template v-for="(l, i) in file.lines">
              <tr v-if="shown(l)" :key="'l' + i" :class="info(l).kind === 'races' && info(l).models.length > 1 ? 'race-openable' : ''"
                  @click="toggle(i, l)">
                <td class="race-pic-cell">
                  <div class="race-pic">
                    <span v-if="picture(l)" :class="'race-models-ctn-' + picture(l) + ' race-pic-sprite'"/>
                  </div>
                </td>
                <td v-if="!l.archive"><code class="text-muted">{{ l.raw }}</code></td>
                <td v-else>
                  <b v-if="file.kind === 'chr'">{{ l.code.toUpperCase() }}</b>
                  <code class="ml-1">{{ l.archive }}</code>
                  <b-badge v-if="!l.raw" variant="success" class="ml-1">new</b-badge>
                  <b-badge v-if="chrCheck(l) === 'disabled' || info(l).kind === 'disabled'" variant="secondary" class="ml-1"
                           :title="'The archive name starts with _, so the client finds nothing: the line is switched off. Remove the _ to load ' + l.archive.replace(/^_+/, '') + '.'">disabled</b-badge>
                  <b-badge v-if="chrCheck(l) === 'missing'" variant="danger" class="ml-1"
                           title="Neither the RoF2 client nor Spire has this archive: the line loads nothing">archive not in the client</b-badge>
                  <b-badge v-if="chrCheck(l) === 'nocode'" variant="danger" class="ml-1"
                           :title="'The archive holds no ' + (l.code || '').toUpperCase() + ' model: the line loads nothing'">no {{ (l.code || "").toUpperCase() }} in it</b-badge>
                  <b-badge v-if="file.kind === 'chr' && !isGlobal() && globalChr[(l.code || '').toUpperCase()]" variant="info" class="ml-1"
                           title="GlobalLoad_chr.txt already loads this model in every zone">global</b-badge>
                  <b-badge v-if="info(l).kind === 'missing'" variant="danger" class="ml-1"
                           title="Neither the RoF2 client nor Spire has this archive: the line loads nothing">not in the client</b-badge>
                  <b-badge v-if="info(l).kind === 'other'" variant="dark" class="ml-1">no races</b-badge>
                  <b-badge v-if="info(l).unnamed.length" variant="warning" class="ml-1"
                           :title="info(l).unnamed.join(', ')">{{ info(l).unnamed.length }} not in Spire's race list</b-badge>
                  <div class="text-muted hint">
                    <span v-if="file.kind === 'chr'">{{ racesOf(l) || "not in Spire's race list" }}</span>
                    <span v-else-if="info(l).kind === 'races'">
                      <i v-if="info(l).models.length > 1" :class="'fa fa-caret-' + (expanded[i] ? 'down' : 'right') + ' mr-1'"></i>
                      {{ info(l).models.length }} race model{{ info(l).models.length === 1 ? "" : "s" }}: {{ modelNames(info(l)) }}
                    </span>
                    <span v-else-if="info(l).kind === 'animations'">
                      animations for {{ info(l).animations.map((c) => (raceName(c) || c) + " (" + c + ")").join(", ") }}
                    </span>
                    <span v-else-if="info(l).kind === 'missing'">the client has no {{ info(l).file }}</span>
                    <span v-else-if="info(l).kind === 'disabled'">switched off: would load {{ l.archive.replace(/^_+/, "") }}</span>
                    <span v-else-if="info(l).kind === 'held'">an archive Spire holds for the patch</span>
                    <span v-else>equipment or objects, no race models</span>
                  </div>
                </td>
                <td class="text-right">
                  <div class="btn-group btn-group-sm mr-1">
                    <b-button size="sm" variant="outline-secondary" @click.stop="moveLine(i, -1)" title="Move up: the client loads lines in order">
                      <i class="fa fa-arrow-up"></i>
                    </b-button>
                    <b-button size="sm" variant="outline-secondary" @click.stop="moveLine(i, 1)" title="Move down">
                      <i class="fa fa-arrow-down"></i>
                    </b-button>
                  </div>
                  <b-button size="sm" variant="outline-danger" @click.stop="removeLine(i)" title="Remove">
                    <i class="fa fa-times"></i>
                  </b-button>
                </td>
              </tr>
              <!-- the races in an archive -->
              <tr v-if="shown(l) && expanded[i]" :key="'o' + i" class="race-models-row">
                <td colspan="3">
                  <div class="race-model-grid">
                    <div v-for="c in info(l).models" :key="c" class="race-model">
                      <div class="race-pic">
                        <span v-if="codeRace[c] && pictures[codeRace[c]]"
                              :class="'race-models-ctn-' + pictures[codeRace[c]] + ' race-pic-sprite'"/>
                      </div>
                      <div>
                        <b>{{ c }}</b>
                        <div class="hint" :class="raceName(c) ? 'text-muted' : 'text-warning'">
                          {{ raceName(c) || "not in Spire's race list" }}
                        </div>
                      </div>
                    </div>
                  </div>
                </td>
              </tr>
            </template>
            </tbody>
          </table>
        </eq-window-simple>
      </div>

      <!-- races to add -->
      <div class="col-lg-6">
        <eq-window-simple title="Add races">
          <b-form-input size="sm" v-model="search" placeholder="Search races by name, ID or model code (e.g. elemental, 75, ELE)"
                        class="mb-2" autofocus/>
          <div class="text-muted hint mb-2">
            Click a model to add it.
            <span v-if="file.kind === 'globalload'">GlobalLoad.txt loads whole archives: adding one loads every race in it.</span>
            <b-badge variant="info">global</b-badge> archives are already loaded in every zone.
          </div>
          <div v-if="search.trim().length < 2" class="text-muted">Type at least two letters.</div>
          <div v-for="r in results()" :key="r.race_id" class="race-result">
            <div class="d-flex align-items-center">
              <span class="race-pic mr-2">
                <span v-if="pictures[r.race_id]" :class="'race-models-ctn-' + pictures[r.race_id] + ' race-pic-sprite'"/>
              </span>
              <div>
                <b>{{ r.description }}</b> <span class="text-muted">({{ r.race_id }})</span>
                <div>
                  <span v-for="s in options(r)" :key="s.key">
                    <span v-if="s.added" class="race-added mr-1 mb-1" :title="s.title">
                      <i class="fa fa-check mr-1"></i>{{ s.label }}
                    </span>
                    <b-button v-else size="sm" class="mr-1 mb-1" variant="outline-success" :title="s.title"
                              @click="add(s); $event.target.blur()">
                      {{ s.label }}
                      <b-badge v-if="s.global" variant="info" class="ml-1">global</b-badge>
                    </b-button>
                  </span>
                  <span v-if="!options(r).length" class="text-muted hint">
                    No RoF2 archive holds its model: add one by hand below if you have it.
                  </span>
                </div>
              </div>
            </div>
          </div>

          <div class="mt-3 d-flex align-items-center flex-wrap">
            <span class="mr-2">Or by hand</span>
            <b-form-input v-if="file.kind === 'chr'" size="sm" v-model="manualCode" placeholder="code" maxlength="3"
                          style="max-width: 80px" class="mr-2"/>
            <b-form-input size="sm" v-model="manualArchive"
                          :placeholder="file.kind === 'chr' ? 'archive (rtn, tnm_chr)' : 'archive (Global_chr, rpt.eqg)'"
                          style="max-width: 220px" class="mr-2"/>
            <b-button size="sm" variant="outline-success" @click="addManual()"
                      :disabled="!manualArchive.trim() || (file.kind === 'chr' && !manualCode.trim())">Add</b-button>
          </div>
          <div class="text-muted hint mt-1">
            For races Spire's list lacks, such as your own models. A <code>_chr.txt</code> names the archive without
            its extension; GlobalLoad.txt keeps <code>.eqg</code> and drops <code>.s3d</code>.
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
import EqAssets        from "@/app/eq-assets/eq-assets";
import {ROUTE}         from "@/routes";

// an archive as each file names it
function base(file) {
  return file.replace(/\.(s3d|eqg)$/i, "")
}

function globalLoadName(file) {
  return /\.eqg$/i.test(file) ? file : base(file)
}

export default {
  name: "StarfallRaceLoading",
  components: { EqWindowSimple, InfoErrorBanner },
  data() {
    return {
      files: [],
      stockZones: {},
      path: null,
      file: null,
      held: true,
      dirty: false,
      saving: false,
      newZone: "",
      search: "",
      manualCode: "",
      manualArchive: "",
      races: [],
      pictures: {},    // race id: its first picture in the race sprites
      codeRaces: {},   // model code: race names
      codeRace: {},    // model code: the first race id with it
      rof2Archives: {}, // model code: the RoF2 archives holding it
      rof2Contents: {}, // archive file, lower case: { models, animations }
      rof2Files: {},    // every archive the RoF2 client has, lower case
      heldArchives: {}, // archives Spire holds for the patch, lower case without extension
      globalChr: {},    // model codes GlobalLoad_chr.txt loads in every zone
      showOther: false, // GlobalLoad.txt lines that load no races
      expanded: {},     // line index: its races shown
      globalArchives: {}, // archive, lower case without extension: loaded in every zone
      archiveRaces: {}, // archive, lower case without extension: race names
      notification: "",
      error: "",
      clientPatchRoute: ROUTE.STARFALL_CLIENT_PATCH,
    }
  },
  methods: {
    showError(e) {
      this.error = e && e.response && e.response.data && e.response.data.error ? e.response.data.error : String(e)
    },
    fileOptions() {
      const out = [{ value: null, text: this.files.length ? "Choose a file..." : "No race files uploaded yet" }]
      this.files.forEach((f) => out.push({ value: f.path, text: f.path + (f.beta ? " (beta only)" : f.ignore ? " (ignored)" : "") }))
      if (this.file && !this.held) {
        out.push({ value: this.file.path, text: this.file.path + " (new)" })
      }
      return out
    },
    async loadFiles() {
      try {
        const r         = await SpireApi.v1().get("client-assets/race-files")
        this.files      = (r.data.files || []).sort((a, b) => a.path.localeCompare(b.path))
        this.stockZones = {}
        ;(r.data.stock_chr_zones || []).forEach((z) => (this.stockZones[z] = true))
      } catch (e) {
        this.showError(e)
      }
    },
    confirmLeave() {
      return !this.dirty || confirm("Drop the changes to " + this.file.path + "?")
    },
    async open(path) {
      if (!path || (this.file && this.file.path !== path && !this.confirmLeave())) {
        this.path = this.file ? this.file.path : null
        return
      }
      this.error = ""
      try {
        const r    = await SpireApi.v1().get("client-assets/race-file", { params: { path: path } })
        this.file  = r.data
        this.held  = true
        this.dirty = false
        this.path  = r.data.path
        this.expanded = {}
      } catch (e) {
        this.showError(e)
      }
    },
    newFile() {
      const zone = this.newZone.trim().toLowerCase().replace(/_chr\.txt$/, "")
      if (zone === "globalload") {
        this.error = "GlobalLoad_chr.txt lives in Resources: upload the client's copy to edit it"
        return
      }
      if (!/^[a-z0-9_]+$/.test(zone) || !this.confirmLeave()) {
        return
      }
      const path = zone + "_chr.txt"
      if (this.files.some((f) => f.path.toLowerCase() === path)) {
        this.open(this.files.find((f) => f.path.toLowerCase() === path).path)
        return
      }
      if (this.stockZones[zone] && !confirm("The stock client has " + path + ". A new one replaces it, races and all: upload the client's copy instead to add to it. Start an empty one anyway?")) {
        return
      }
      this.file    = { path: path, kind: "chr", lines: [], crlf: false, read: 0 }
      this.held    = false
      this.dirty   = true
      this.path    = path
      this.newZone = ""
    },
    stockZone() {
      return this.file && this.stockZones[this.file.path.toLowerCase().replace(/_chr\.txt$/, "")]
    },
    async upload(e) {
      const files = Array.from(e.target.files || [])
      e.target.value = ""
      if (!files.length) {
        return
      }
      const form = new FormData()
      for (const f of files) {
        const name = f.name.toLowerCase()
        if (name === "globalload.txt" || name === "globalload_chr.txt") {
          form.append("files", f, f.name)
          form.append("paths", "Resources/" + f.name)
        } else if (name.endsWith("_chr.txt")) {
          form.append("files", f, f.name)
          form.append("paths", f.name)
        } else {
          this.error = f.name + " is not a _chr.txt, GlobalLoad_chr.txt or GlobalLoad.txt"
          return
        }
      }
      try {
        const r           = await SpireApi.v1().post("client-assets/upload", form)
        const results     = r.data.results || []
        this.notification = results.map((x) => x.error || x.result).join("; ")
        await this.loadFiles()
        await this.loadGlobalChr()
        if (results.length === 1 && !results[0].error) {
          const name = files[0].name.toLowerCase()
          const kept = this.files.find((f) => f.path.toLowerCase().endsWith(name))
          if (kept) {
            await this.open(kept.path)
          }
        }
      } catch (err) {
        this.showError(err)
      }
    },
    entries() {
      return this.file.lines.filter((l) => l.archive)
    },

    // ------------------------------------------------------------ races
    async loadRaces() {
      try {
        const r    = await SpireApi.v1().get("/static-map/race-inventory-map.json")
        this.races = r.data.races || []
        const codeRaces    = {}
        const codeRace     = {}
        const archiveRaces = {}
        this.races.forEach((race) => {
          [race.male_gender_model_code, race.female_gender_model_code, race.neutral_gender_model_code].filter((c) => c).forEach((code) => {
            const c      = code.toUpperCase()
            codeRaces[c] = codeRaces[c] || new Set()
            codeRaces[c].add(race.description)
            codeRace[c] = codeRace[c] || race.race_id
          })
          ;(race.sources || []).forEach((s) => {
            const a = base(s.source_file).toLowerCase()
            archiveRaces[a] = archiveRaces[a] || new Set()
            archiveRaces[a].add(race.description)
            ;(s.models || []).forEach((m) => {
              const c      = m.code.toUpperCase()
              codeRaces[c] = codeRaces[c] || new Set()
              codeRaces[c].add(race.description)
              codeRace[c] = codeRace[c] || race.race_id
            })
          })
        })
        const globalArchives = {}
        this.races.forEach((race) => (race.sources || []).forEach((s) => {
          if (s.is_global) {
            globalArchives[base(s.source_file).toLowerCase()] = true
          }
        }))
        // what the RoF2 client's archives hold, by model code
        const scan = await SpireApi.v1().get("/static-map/rof2-race-archives.json")
        const rof2 = scan.data.codes || {}
        this.rof2Contents = scan.data.archives || {}
        const files       = {}
        ;(scan.data.files || []).forEach((f) => (files[f] = true))
        this.rof2Files = files
        Object.keys(rof2).forEach((code) => rof2[code].forEach((file) => {
          const a         = base(file).toLowerCase()
          archiveRaces[a] = archiveRaces[a] || new Set()
          ;(codeRaces[code] || []).forEach((name) => archiveRaces[a].add(name))
        }))
        this.rof2Archives   = rof2
        this.globalArchives = globalArchives
        this.codeRaces      = codeRaces
        this.codeRace       = codeRace
        this.archiveRaces   = archiveRaces
      } catch (e) {
        this.showError(e)
      }
      try {
        // the race sprites: the first picture of each race, texture 0 first
        const models   = await EqAssets.getNpcModels()
        const pictures = {}
        models.forEach((m) => {
          const key = m.fileName.replace(".png", "").replace("CTN_", "").replaceAll("_", "-")
          const id  = parseInt(m.raceId)
          if (!pictures[id] || (key.split("-")[2] === "0" && pictures[id].split("-")[2] !== "0")) {
            pictures[id] = key
          }
        })
        this.pictures = pictures
      } catch (e) {
        // no pictures: the names still show
      }
    },
    // whether a _chr.txt line can load: "missing" when no archive of its
    // name is in the client or Spire, "nocode" when the client's archive
    // holds no model of its code, "" when it can (or Spire cannot tell)
    chrCheck(l) {
      if (this.file.kind !== "chr" || !l.archive || !l.code || !Object.keys(this.rof2Files).length) {
        return ""
      }
      if (l.archive.startsWith("_")) {
        return "disabled"
      }
      const name = base(l.archive).toLowerCase()
      const file = [name + ".s3d", name + ".eqg"].find((f) => this.rof2Files[f])
      if (!file) {
        return this.heldArchives[name] ? "" : "missing"
      }
      const c = this.rof2Contents[file]
      return c && c.models.includes(l.code.toUpperCase()) ? "" : (this.heldArchives[name] ? "" : "nocode")
    },
    // a file every zone loads
    isGlobal() {
      return this.file && this.file.path.toLowerCase().startsWith("resources/")
    },
    // the models GlobalLoad_chr.txt loads in every zone, as Spire holds it
    async loadGlobalChr() {
      const held = this.files.find((f) => f.path.toLowerCase() === "resources/globalload_chr.txt")
      const out  = {}
      if (held) {
        try {
          const r = await SpireApi.v1().get("client-assets/race-file", { params: { path: held.path } })
          r.data.lines.forEach((l) => {
            if (l.code) {
              out[l.code.toUpperCase()] = true
            }
          })
        } catch (e) {
          // not read: nothing is marked
        }
      }
      this.globalChr = out
    },
    async loadHeld() {
      try {
        const r     = await SpireApi.v1().get("client-assets")
        const index = r.data.index || r.data
        const held  = {}
        Object.values(index.archives || {}).forEach((a) => (held[base(a.file).toLowerCase()] = true))
        Object.values(index.files || {}).forEach((f) => (held[base(f.path.split("/").pop()).toLowerCase()] = true))
        this.heldArchives = held
      } catch (e) {
        // not known: nothing is marked held
      }
    },
    // what a GlobalLoad.txt line loads: race models, animations, other
    // things (equipment, objects), or nothing (missing from the client)
    info(l) {
      const empty = { kind: "", file: "", models: [], animations: [], unnamed: [] }
      if (this.file.kind !== "globalload" || !l.archive) {
        return empty
      }
      const name = l.archive.toLowerCase()
      if (name.startsWith("_")) {
        return { kind: "disabled", file: "", models: [], animations: [], unnamed: [] }
      }
      const file = /\.eqg$/.test(name) ? name : [name + ".s3d", name + ".eqg"].find((f) => this.rof2Files[f]) || name + ".s3d"
      const c    = this.rof2Contents[file] || { models: [], animations: [] }
      const out  = { file: file, models: c.models, animations: c.animations, unnamed: c.models.filter((m) => !this.raceName(m)) }
      if (!this.rof2Files[file]) {
        out.kind = this.heldArchives[base(name)] ? "held" : "missing"
      } else if (c.models.length) {
        out.kind = "races"
      } else if (c.animations.length) {
        out.kind = "animations"
      } else {
        out.kind = "other"
      }
      return out
    },
    shown(l) {
      return this.file.kind !== "globalload" || this.showOther || this.info(l).kind !== "other" || !l.raw
    },
    // pairs of race archives that both define models, in load order: the
    // first one's versions show
    replaced() {
      const firstAt = {} // model code: index of the first line defining it
      const pairs   = {}
      this.file.lines.forEach((l, i) => {
        const info = this.info(l)
        if (info.kind !== "races") {
          return
        }
        info.models.forEach((c) => {
          if (firstAt[c] === undefined) {
            firstAt[c] = i
            return
          }
          if (base(this.file.lines[firstAt[c]].archive).toLowerCase() === base(l.archive).toLowerCase()) {
            return // the same archive listed twice: one version
          }
          const key = firstAt[c] + "-" + i
          pairs[key] = pairs[key] || { key: key, firstAt: firstAt[c], secondAt: i, codes: [] }
          pairs[key].codes.push(c)
        })
      })
      return Object.values(pairs).map((p) => Object.assign(p, {
        first: this.file.lines[p.firstAt].archive,
        second: this.file.lines[p.secondAt].archive,
      }))
    },
    // the second archive's versions win: its line moves just above the first's
    swapVersions(r) {
      const lines = this.file.lines.slice()
      const moved = lines.splice(r.secondAt, 1)[0]
      lines.splice(r.firstAt, 0, moved)
      this.file.lines = lines
      this.expanded   = {}
      this.dirty      = true
    },
    hiddenCount() {
      return this.file.lines.filter((l) => this.info(l).kind === "other").length
    },
    summary() {
      const kinds = {}
      this.file.lines.forEach((l) => {
        const k  = this.info(l).kind
        kinds[k] = (kinds[k] || 0) + 1
      })
      const parts = [(kinds.races || 0) + " race archives"]
      if (kinds.animations) {
        parts.push(kinds.animations + " of animations")
      }
      if (kinds.other) {
        parts.push(kinds.other + " without races")
      }
      if (kinds.missing) {
        parts.push(kinds.missing + " missing from the client")
      }
      return parts.join(", ")
    },
    toggle(i, l) {
      if (this.info(l).kind === "races" && this.info(l).models.length > 1) {
        this.$set(this.expanded, i, !this.expanded[i])
      }
    },
    raceName(code) {
      const names = this.codeRaces[code.toUpperCase()]
      return names ? Array.from(names).join(" / ") : ""
    },
    modelNames(info) {
      const names = []
      info.models.forEach((c) => (this.codeRaces[c] || new Set([c])).forEach((n) => {
        if (!names.includes(n)) {
          names.push(n)
        }
      }))
      return names.slice(0, 6).join(", ") + (names.length > 6 ? " and " + (names.length - 6) + " more" : "")
    },
    racesOf(l) {
      const names = this.file.kind === "chr"
        ? this.codeRaces[(l.code || "").toUpperCase()]
        : this.archiveRaces[base(l.archive).toLowerCase()]
      if (!names) {
        return ""
      }
      const list = Array.from(names)
      return list.slice(0, 6).join(", ") + (list.length > 6 ? " and " + (list.length - 6) + " more" : "")
    },
    picture(l) {
      const code = this.file.kind === "chr" ? (l.code || "") : (this.info(l).models.find((c) => this.codeRace[c]) || "")
      const id   = this.codeRace[code.toUpperCase()]
      return id ? this.pictures[id] : ""
    },
    results() {
      const q = this.search.trim().toLowerCase()
      if (q.length < 2 && !/^\d+$/.test(q)) {
        return []
      }
      return this.races.filter((r) =>
        r.description.toLowerCase().includes(q) ||
        String(r.race_id) === q ||
        [r.male_gender_model_code, r.female_gender_model_code, r.neutral_gender_model_code].some((c) => c && c.toLowerCase() === q)
      ).slice(0, 40)
    },
    has(code, archive) {
      return this.file.lines.some((l) => l.archive && base(l.archive).toLowerCase() === base(archive).toLowerCase() &&
        (this.file.kind !== "chr" || (l.code || "").toUpperCase() === code.toUpperCase()))
    },
    // the lines a race could be added as: a model per archive for a zone
    // file, an archive for GlobalLoad.txt
    // the lines a race could be added as: a model per archive for a zone
    // file, an archive for GlobalLoad.txt. Which archives hold a model comes
    // from the RoF2 client's own archives; the race list fills in the rest.
    options(race) {
      const pairs = []
      const codes = new Set([race.male_gender_model_code, race.female_gender_model_code, race.neutral_gender_model_code]
        .filter((c) => c).map((c) => c.toUpperCase()))
      ;(race.sources || []).forEach((s) => (s.models || []).forEach((m) => codes.add(m.code.toUpperCase())))
      codes.forEach((c) => (this.rof2Archives[c] || []).forEach((file) => pairs.push({ code: c, file: file })))
      ;(race.sources || []).forEach((s) => (s.models || []).forEach((m) => pairs.push({ code: m.code.toUpperCase(), file: s.source_file })))
      if (this.file.kind === "globalload") {
        // an archive the race list names, with no code: still the archive
        ;(race.sources || []).forEach((s) => pairs.push({ code: "", file: s.source_file }))
      }

      const out  = []
      const seen = {}
      pairs.forEach((p) => {
        const chr     = this.file.kind === "chr"
        const archive = chr ? base(p.file) : globalLoadName(p.file)
        const key     = (chr ? p.code : "") + "," + archive.toLowerCase()
        if (seen[key] || (chr && !p.code)) {
          return
        }
        seen[key] = true
        const listed = (race.sources || []).find((s) => s.source_file.toLowerCase() === p.file.toLowerCase())
        out.push({
          key: key,
          code: chr ? p.code : "",
          archive: archive,
          label: (chr ? p.code + " in " : "") + archive,
          global: this.globalArchives[base(p.file).toLowerCase()] || (chr && !this.isGlobal() && this.globalChr[p.code]) || false,
          added: this.has(chr ? p.code : "", archive),
          title: p.file + (listed && listed.loaded_via_description ? " (" + listed.loaded_via_description + ")" : ""),
        })
      })
      return out
    },
    add(option) {
      this.addLine(option.code, option.archive)
    },
    addManual() {
      this.addLine(this.manualCode.trim(), this.manualArchive.trim())
      this.manualCode    = ""
      this.manualArchive = ""
    },
    addLine(code, archive) {
      if (this.has(code, archive)) {
        return
      }
      this.file.lines.push(this.file.kind === "chr" ? { code: code.toUpperCase(), archive: archive } : { archive: archive })
      this.dirty = true
    },
    // swap with the next line shown above or below
    moveLine(i, step) {
      let j = i + step
      while (j >= 0 && j < this.file.lines.length && !this.shown(this.file.lines[j])) {
        j += step
      }
      if (j < 0 || j >= this.file.lines.length) {
        return
      }
      const lines = this.file.lines.slice()
      ;[lines[i], lines[j]] = [lines[j], lines[i]]
      this.file.lines = lines
      this.expanded   = {}
      this.dirty      = true
    },
    removeLine(i) {
      const l = this.file.lines[i]
      if (this.isGlobal() && l.raw && !confirm("Remove " + l.archive + " from " + this.file.path + "? Every zone stops loading it.")) {
        return
      }
      this.file.lines.splice(i, 1)
      this.expanded = {}
      this.dirty = true
    },
    async save() {
      this.saving = true
      this.error  = ""
      try {
        const r           = await SpireApi.v1().put("client-assets/race-file", this.file)
        this.file         = r.data.file
        this.held         = true
        this.dirty        = false
        this.notification = r.data.result + ". Publish on Client Patch to send it to players."
        await this.loadFiles()
        await this.loadGlobalChr()
        this.path = this.file.path
      } catch (e) {
        this.showError(e)
      }
      this.saving = false
    },
  },
  async mounted() {
    await Promise.all([this.loadFiles(), this.loadRaces(), this.loadHeld()])
    await this.loadGlobalChr()
  },
  beforeRouteLeave(to, from, next) {
    next(this.confirmLeave())
  },
}
</script>

<style scoped>
.hint {
  font-size: 13px;
}

.race-lines td {
  vertical-align: middle;
}

/* the race sprites are drawn large: shown at a third */
.race-pic-cell {
  width: 64px;
}

.race-lines td:nth-child(2) {
  width: 100%;
}

.race-replaced {
  border: 1px solid rgba(255, 193, 7, .35);
  border-radius: 6px;
  padding: 8px;
}

.race-openable {
  cursor: pointer;
}

.race-model-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 6px;
}

.race-model {
  display: flex;
  align-items: center;
  gap: 8px;
}

.race-pic {
  width: 56px;
  height: 56px;
  display: inline-block;
  overflow: hidden;
  position: relative;
  vertical-align: middle;
  flex-shrink: 0;
}

/* out of the layout, so its full size takes no room */
.race-pic-sprite {
  position: absolute;
  left: 0;
  top: 0;
  transform: scale(.33);
  transform-origin: top left;
}

.race-added {
  display: inline-block;
  font-size: 13px;
  padding: 3px 8px;
  border: 1px solid rgba(255, 255, 255, .2);
  border-radius: 4px;
  opacity: .7;
}

.race-result {
  border-bottom: 1px solid rgba(255, 255, 255, .1);
  padding: 6px 0;
}
</style>
