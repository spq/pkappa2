<template>
  <v-simple-table dense>
    <thead>
      <tr>
        <th class="text-left">Name</th>
        <th class="text-left">Query</th>
        <th colspan="2" class="text-left">Status</th>
      </tr>
    </thead>
    <tbody>
      <tr v-for="tagType in tagTypes" :key="tagType.key">
        <th colspan="4">
          <v-icon>mdi-{{ tagType.icon }}</v-icon>
          {{ tagType.title }}
        </th>
      </tr>
      <tr v-for="tag in groupedTags[tagType.key]" :key="tag.Name">
        <td>
          <v-icon>mdi-circle-small</v-icon
          ><v-chip :color="tag.Color" small>{{ tag.Name.substring(1 + tagType.key.length) }}</v-chip>
        </td>
        <td>{{ tag.Definition }}</td>
        <td>
          Matching {{ tag.MatchingCount }} Streams<span
            v-if="tag.UncertainCount != 0"
            >, {{ tag.UncertainCount }} uncertain</span
          ><span v-if="tag.Referenced">, Referenced by another tag</span>
        </td>
        <td align="right">
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
                    q: tagForURI(tag.Name),
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
    </tbody>
  </v-simple-table>
</template>

<script>
import { mapActions, mapGetters, mapState } from "vuex";
import { EventBus } from "./EventBus";
import {tagForURI} from "@/filters/tagForURI";

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
  },
  methods: {
      tagForURI,
    ...mapActions(["updateTags"]),
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
