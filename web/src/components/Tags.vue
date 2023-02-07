<template>
  <v-simple-table dense>
    <thead>
      <tr>
        <th class="text-left" width="20%">Name</th>
        <th class="text-left" width="50%">Query</th>
        <th class="text-left" width="10%">Status</th>
        <th colspan="2" class="text-left" width="20%">Converters</th>
      </tr>
    </thead>
    <tbody>
      <template v-for="tagType in tagTypes">
        <tr :key="tagType.key">
          <th colspan="5">
            <v-icon>mdi-{{ tagType.icon }}</v-icon>
            {{ tagType.title }}
          </th>
        </tr>
        <tr v-for="tag in groupedTags[tagType.key]" :key="tag.Name">
          <td>
            <v-icon>mdi-circle-small</v-icon
            ><v-chip :color="tag.Color" small>{{
              tag.Name.substring(1 + tagType.key.length)
            }}</v-chip>
          </td>
          <td>
            <div class="tag_definition" :title="tag.Definition">
              {{ tag.Definition }}
            </div>
          </td>
          <td>
            Matching {{ tag.MatchingCount }} Streams<span
              v-if="tag.UncertainCount != 0"
              >, {{ tag.UncertainCount }} uncertain</span
            ><span v-if="tag.Referenced">, Referenced by another tag</span>
          </td>
          <td>
            <span>{{ converterList[tag.Name] }}</span>
          </td>
          <td style="text-align: right">
            <v-tooltip bottom>
              <template #activator="{ on, attrs }">
                <v-btn
                  v-bind="attrs"
                  v-on="on"
                  icon
                  exact
                  :to="{
                    name: 'search',
                    query: {
                      q: $options.filters.tagForURI(tag.Name),
                    },
                  }"
                  ><v-icon>mdi-magnify</v-icon></v-btn
                >
              </template>
              <span>Show Streams</span>
            </v-tooltip>
            <v-tooltip bottom>
              <template #activator="{ on, attrs }">
                <v-btn
                  v-bind="attrs"
                  v-on="on"
                  icon
                  @click="setQuery(tag.Definition)"
                  ><v-icon>mdi-form-textbox</v-icon></v-btn
                >
              </template>
              <span>Use Query</span>
            </v-tooltip>
            <v-tooltip bottom>
              <template #activator="{ on, attrs }">
                <v-btn
                  v-bind="attrs"
                  v-on="on"
                  icon
                  @click="showTagColorChangeDialog(tag.Name)"
                  ><v-icon>mdi-palette</v-icon></v-btn
                >
              </template>
              <span>Change Color</span>
            </v-tooltip>
            <v-tooltip bottom>
              <template #activator="{ on, attrs }">
                <v-btn
                  v-bind="attrs"
                  v-on="on"
                  icon
                  @click="showTagSetConvertersDialog(tag.Name)"
                  ><v-icon>mdi-file-replace-outline</v-icon></v-btn
                >
              </template>
              <span>Attach Converter</span>
            </v-tooltip>
            <v-tooltip bottom>
              <template #activator="{ on, attrs }">
                <v-btn
                  v-bind="attrs"
                  v-on="on"
                  :disabled="tag.Referenced"
                  icon
                  @click="confirmTagDeletion(tag.Name)"
                  ><v-icon>mdi-delete</v-icon></v-btn
                >
              </template>
              <span>Delete</span>
            </v-tooltip>
          </td>
        </tr>
      </template>
    </tbody>
  </v-simple-table>
</template>

<style scoped>
.tag_definition {
  word-break: break-all;
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 3;
  overflow: hidden;
}
</style>

<script>
import { mapActions, mapGetters, mapState } from "vuex";
import { EventBus } from "./EventBus";

export default {
  name: "Tags",
  data() {
    return {
      tagTypes: [
        {
          title: "Services",
          icon: "cloud-outline",
          key: "service",
        },
        {
          title: "Tags",
          icon: "tag-multiple-outline",
          key: "tag",
        },
        {
          title: "Marks",
          icon: "checkbox-multiple-outline",
          key: "mark",
        },
        {
          title: "Generated",
          icon: "robot-outline",
          key: "generated",
        },
      ],
    };
  },
  mounted() {
    this.updateTags();
  },
  computed: {
    ...mapState(["tags"]),
    ...mapGetters(["groupedTags"]),
    converterList() {
      return this.tags.reduce((acc, tag) => {
        acc[tag.Name] = tag.Converters.join(", ");
        return acc;
      }, {});
    },
  },
  methods: {
    ...mapActions(["updateTags"]),
    showTagSetConvertersDialog(tagId) {
      EventBus.$emit("showTagSetConvertersDialog", { tagId });
    },
    confirmTagDeletion(tagId) {
      EventBus.$emit("showTagDeleteDialog", { tagId });
    },
    setQuery(query) {
      EventBus.$emit("setSearchTerm", { searchTerm: query });
    },
    showTagColorChangeDialog(tagId) {
      EventBus.$emit("showTagColorChangeDialog", { tagId });
    },
  },
};
</script>
