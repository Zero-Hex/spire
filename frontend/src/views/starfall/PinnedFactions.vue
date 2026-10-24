<template>
  <div>
    <starfall-table-editor :def="def" :list-height="600" empty-text="No pinned factions"/>
  </div>
</template>

<script>
import StarfallTableEditor     from "@/components/starfall/StarfallTableEditor";
import {FACTION_WINDOW_PINNED} from "@/app/starfall/tables/world";

export default {
  name: "StarfallPinnedFactions",
  components: { StarfallTableEditor },
  data() {
    return { def: FACTION_WINDOW_PINNED }
  },
}
</script>
