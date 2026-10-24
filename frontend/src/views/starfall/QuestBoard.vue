<template>
  <div>
    <eq-window-simple title="Starfall Task Board">
      <div class="d-flex align-items-center flex-wrap">
        <div class="mr-3">
          Quests listed by board NPCs (plugins/quest_board.pl). Changes show the next time a player opens the board.
        </div>
        <b-form-select
          v-model="board"
          class="form-control form-control-sm"
          style="max-width: 250px"
        >
          <option :value="null">All boards</option>
          <option v-for="b in boards" :key="b" :value="b">{{ b }}</option>
        </b-form-select>
      </div>
    </eq-window-simple>

    <starfall-table-editor
      class="mt-3"
      :def="def"
      :filter="board ? { board: board } : {}"
      :title="board ? 'Quests on the ' + board + ' board' : 'Quests'"
      empty-text="No quests yet"
      @changed="loadBoards()"
    />
  </div>
</template>

<script>
import EqWindowSimple      from "@/components/eq-ui/EQWindowSimple";
import StarfallTableEditor from "@/components/starfall/StarfallTableEditor";
import {SpireApi}          from "@/app/api/spire-api";
import {SpireQueryBuilder} from "@/app/api/spire-query-builder";
import {QUEST_BOARD}       from "@/app/starfall/tables/questboard";

export default {
  name: "StarfallQuestBoard",
  components: { EqWindowSimple, StarfallTableEditor },
  data() {
    return {
      def: QUEST_BOARD,
      board: null,
      boards: [],
    }
  },
  methods: {
    async loadBoards() {
      try {
        const r     = await SpireApi.v1().get("custom_quest_boards", {
          params: (new SpireQueryBuilder()).select(["board"]).groupBy(["board"]).limit(1000).get()
        })
        this.boards = (r.data || []).map((x) => x.board).filter((x) => x).sort()
      } catch (e) {
        this.boards = []
      }
    },
  },
  mounted() {
    this.loadBoards()
  },
}
</script>
