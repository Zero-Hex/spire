<template>
  <div>
    <starfall-table-editor :def="def" :list-height="600" empty-text="No NPCs hidden: every inspect shows what the rules allow"/>
  </div>
</template>

<script>
import StarfallTableEditor      from "@/components/starfall/StarfallTableEditor";
import {MOB_INSPECT_VISIBILITY} from "@/app/starfall/tables/world";

export default {
  name: "StarfallMobInspect",
  components: { StarfallTableEditor },
  data() {
    return { def: MOB_INSPECT_VISIBILITY }
  },
}
</script>
