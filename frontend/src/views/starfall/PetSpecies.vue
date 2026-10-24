<template>
  <div>
    <eq-window-simple title="Starfall Pets">
      <div class="d-flex align-items-center flex-wrap">
        <div class="mr-3">
          Zones cache these tables. After editing, reload them on the server
          (the same as <code>#reload pet_species global</code>).
        </div>
        <b-button size="sm" variant="outline-warning" @click="reload()" :disabled="reloading">
          <i class="fa fa-refresh"></i> Reload pet species
        </b-button>
      </div>
      <info-error-banner
        class="mt-2"
        :notification="notification"
        :error="error"
        @dismiss-error="error = ''"
        @dismiss-notification="notification = ''"
      />
    </eq-window-simple>

    <div class="row mt-3">
      <div class="col-lg-7 col-sm-12">
        <starfall-table-editor
          :def="defs.species"
          :list-height="300"
          @select="species = $event"
          @deleted="species = null"
        />
      </div>
      <div class="col-lg-5 col-sm-12">
        <starfall-table-editor
          :def="defs.growth"
          :filter="species ? { species_id: species.id } : {}"
          :waiting="!species"
          :title="species ? 'Growth per level: ' + species.name : 'Growth per level'"
          :empty-text="species ? 'No growth row: this species stays at its base stats' : 'Select a species'"
        />
        <starfall-table-editor
          class="mt-3"
          :def="defs.classGrowth"
        />
      </div>
    </div>
  </div>
</template>

<script>
import EqWindowSimple                                       from "@/components/eq-ui/EQWindowSimple";
import InfoErrorBanner                                      from "@/components/InfoErrorBanner";
import StarfallTableEditor                                  from "@/components/starfall/StarfallTableEditor";
import {SpireApi}                                           from "@/app/api/spire-api";
import {PET_CLASS_GROWTH, PET_SPECIES, PET_SPECIES_GROWTH} from "@/app/starfall/tables/pets";

export default {
  name: "StarfallPetSpecies",
  components: { EqWindowSimple, InfoErrorBanner, StarfallTableEditor },
  data() {
    return {
      species: null,
      reloading: false,
      notification: "",
      error: "",
      defs: {
        species: PET_SPECIES,
        growth: PET_SPECIES_GROWTH,
        classGrowth: PET_CLASS_GROWTH,
      },
    }
  },
  methods: {
    // the reload command is looked up from the server's own list, so this
    // keeps working whatever world names it
    async reload() {
      this.reloading    = true
      this.error        = ""
      this.notification = ""
      try {
        const types = await SpireApi.v1().get("eqemuserver/reload-types")
        const type  = ((types.data && types.data.data) || []).find((t) => /pet species/i.test(t.description))
        if (!type) {
          this.error = "The server has no Pet Species reload. Is it running a Starfall build?"
        } else {
          await SpireApi.v1().post("eqemuserver/reload/" + type.command)
          this.notification = "Pet species reloaded on every zone"
        }
      } catch (e) {
        this.error = e.response && e.response.data && e.response.data.error ? e.response.data.error : String(e)
      }
      this.reloading = false
    },
  },
}
</script>
