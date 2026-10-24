<template>
  <div>
    <eq-window-simple title="Starfall Help Window">
      The in-game help window's tree. The server reads it each time a player opens or browses the window.
    </eq-window-simple>

    <div class="row mt-3">
      <div class="col-lg-4 col-sm-12">
        <starfall-table-editor
          :def="defs.categories"
          @select="category = $event"
          @deleted="category = null"
        />
      </div>
      <div class="col-lg-8 col-sm-12">
        <starfall-table-editor
          :def="defs.entries"
          :filter="category ? { category_id: category.id } : {}"
          :waiting="!category"
          :title="category ? 'Entries in ' + category.name : 'Entries'"
          empty-text="Select a category"
        />
      </div>
    </div>
  </div>
</template>

<script>
import EqWindowSimple                 from "@/components/eq-ui/EQWindowSimple";
import StarfallTableEditor            from "@/components/starfall/StarfallTableEditor";
import {HELP_CATEGORIES, HELP_ENTRIES} from "@/app/starfall/tables/help";

export default {
  name: "StarfallHelpWindow",
  components: { EqWindowSimple, StarfallTableEditor },
  data() {
    return {
      category: null,
      defs: {
        categories: HELP_CATEGORIES,
        entries: HELP_ENTRIES,
      },
    }
  },
}
</script>
