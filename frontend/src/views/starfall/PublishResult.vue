<template>
  <div class="mt-2">
    <div v-if="Object.keys(lists()).length === 0" class="text-muted">{{ empty || "Nothing changed since the last publish." }}</div>
    <div v-for="(list, kind) in lists()" :key="kind">
      <span :class="warn(kind) ? 'text-warning' : 'text-success'">{{ kind }} ({{ list.length }}):</span>
      <code v-for="f in list" :key="f" class="mr-2">{{ f }}</code>
    </div>
  </div>
</template>

<script>
// what a publish or promote changed (or would change)
const LABELS = {
  added: "added",
  changed: "changed",
  beta_only: "beta only changed",
  removed: "no longer sent",
  deleted: "deleted from players",
  undeleted: "no longer deleted",
}

export function changeCount(diff) {
  return Object.keys(LABELS).filter((k) => diff && diff[k] && diff[k].length).length
}

export default {
  name: "PublishResult",
  props: {
    diff: { type: Object, required: true },
    empty: { type: String, default: "" },
  },
  methods: {
    lists() {
      const out = {}
      for (const kind of Object.keys(LABELS)) {
        if (this.diff[kind] && this.diff[kind].length) {
          out[LABELS[kind]] = this.diff[kind]
        }
      }
      return out
    },
    warn(kind) {
      return kind === "deleted from players" || kind === "no longer sent"
    },
  },
}
</script>
