<template>
  <div>
    <eq-window-simple title="Starfall Collections">
      Pick a category, then a collection to edit its items. Progress is shared by every character on an account.
      The server reads these tables as players deposit, so changes apply without a reload.
    </eq-window-simple>

    <div class="row mt-3">
      <div class="col-lg-4 col-sm-12">
        <starfall-table-editor
          :def="defs.categories"
          @select="category = $event; collection = null"
          @deleted="category = null; collection = null"
        />
      </div>
      <div class="col-lg-5 col-sm-12">
        <starfall-table-editor
          :def="defs.collections"
          :filter="category ? { category_id: category.id } : {}"
          :waiting="!category"
          :title="category ? 'Collections in ' + category.name : 'Collections'"
          empty-text="Select a category"
          @select="collection = $event"
          @deleted="collection = null"
        />
      </div>
      <div class="col-lg-3 col-sm-12">
        <starfall-table-editor
          :def="defs.items"
          :filter="collection ? { collection_id: collection.id } : {}"
          :waiting="!collection"
          :title="collection ? 'Items in ' + collection.name : 'Items'"
          :empty-text="collection ? 'No items: this collection can never complete' : 'Select a collection'"
        />
      </div>
    </div>
  </div>
</template>

<script>
import EqWindowSimple                                         from "@/components/eq-ui/EQWindowSimple";
import StarfallTableEditor                                    from "@/components/starfall/StarfallTableEditor";
import {COLLECTION_CATEGORIES, COLLECTION_ITEMS, COLLECTIONS} from "@/app/starfall/tables/collections";

export default {
  name: "StarfallCollections",
  components: { EqWindowSimple, StarfallTableEditor },
  data() {
    return {
      category: null,
      collection: null,
      defs: {
        categories: COLLECTION_CATEGORIES,
        collections: COLLECTIONS,
        items: COLLECTION_ITEMS,
      },
    }
  },
}
</script>
