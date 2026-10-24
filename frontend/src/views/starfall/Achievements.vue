<template>
  <div>
    <eq-window-simple title="Starfall Achievements">
      Pick a category, then an achievement to edit its objectives and rewards.
      The server reads these tables as players act, so changes apply without a reload.
    </eq-window-simple>

    <div class="row mt-3">
      <div class="col-lg-4 col-sm-12">
        <starfall-table-editor
          :def="defs.categories"
          @select="category = $event; achievement = null"
          @deleted="category = null; achievement = null"
        />
      </div>
      <div class="col-lg-8 col-sm-12">
        <starfall-table-editor
          :def="defs.achievements"
          :filter="category ? { category_id: category.id } : {}"
          :waiting="!category"
          :title="category ? 'Achievements in ' + category.name : 'Achievements'"
          empty-text="Select a category"
          @select="achievement = $event"
          @deleted="achievement = null"
        />
      </div>
    </div>

    <div class="row mt-3" v-if="achievement">
      <div class="col-lg-6 col-sm-12">
        <starfall-table-editor
          :def="defs.objectives"
          :filter="{ achievement_id: achievement.id }"
          :title="'Objectives for ' + achievement.name"
          empty-text="No objectives: this achievement can never complete"
        />
      </div>
      <div class="col-lg-6 col-sm-12">
        <starfall-table-editor
          :def="defs.rewards"
          :filter="{ achievement_id: achievement.id }"
          :title="'Rewards for ' + achievement.name"
          empty-text="No rewards"
        />
      </div>
    </div>
  </div>
</template>

<script>
import EqWindowSimple      from "@/components/eq-ui/EQWindowSimple";
import StarfallTableEditor from "@/components/starfall/StarfallTableEditor";
import {
  ACHIEVEMENT_CATEGORIES,
  ACHIEVEMENT_OBJECTIVES,
  ACHIEVEMENT_REWARDS,
  ACHIEVEMENTS
}                          from "@/app/starfall/tables/achievements";

export default {
  name: "StarfallAchievements",
  components: { EqWindowSimple, StarfallTableEditor },
  data() {
    return {
      category: null,
      achievement: null,
      defs: {
        categories: ACHIEVEMENT_CATEGORIES,
        achievements: ACHIEVEMENTS,
        objectives: ACHIEVEMENT_OBJECTIVES,
        rewards: ACHIEVEMENT_REWARDS,
      },
    }
  },
}
</script>
