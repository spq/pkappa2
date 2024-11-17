<template>
  <div>
    <ToolBar>
      <v-tooltip bottom>
        <template #activator="{ on, attrs }">
          <v-btn v-bind="attrs" icon v-on="on" @click="updateTags">
            <v-icon>mdi-refresh</v-icon>
          </v-btn>
        </template>
        <span>Refresh</span>
      </v-tooltip>
      <v-tooltip bottom>
        <template #activator="{ on, attrs }">
          <v-btn v-bind="attrs" icon v-on="on" @click="setupCTF">
            <v-icon>mdi-wizard-hat</v-icon>
          </v-btn>
        </template>
        <span>CTF Setup Wizards</span>
      </v-tooltip>
    </ToolBar>
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
              ><v-chip :color="tag.Color" small :text-color="isDarkColor(tag.Color) ? 'white' : 'black'">{{
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
                    icon
                    exact
                    :to="{
                      name: 'search',
                      query: {
                        q: tagForURI(tag.Name),
                      },
                    }"
                    v-on="on"
                    ><v-icon>mdi-magnify</v-icon></v-btn
                  >
                </template>
                <span>Show Streams</span>
              </v-tooltip>
              <v-tooltip bottom>
                <template #activator="{ on, attrs }">
                  <v-btn
                    v-bind="attrs"
                    icon
                    v-on="on"
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
                    icon
                    v-on="on"
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
                    :disabled="tag.Referenced"
                    icon
                    v-on="on"
                    @click="showTagNameChangeDialog(tag.Name)"
                    ><v-icon>mdi-rename-outline</v-icon></v-btn
                  >
                </template>
                <span>Change Name</span>
              </v-tooltip>
              <v-tooltip bottom>
                <template #activator="{ on, attrs }">
                  <v-btn
                    v-bind="attrs"
                    icon
                    v-on="on"
                    @click="showTagDefinitionChangeDialog(tag.Name)"
                    ><v-icon>mdi-text-search-variant</v-icon></v-btn
                  >
                </template>
                <span>Change Definition</span>
              </v-tooltip>
              <v-tooltip bottom>
                <template #activator="{ on, attrs }">
                  <v-btn
                    v-bind="attrs"
                    icon
                    v-on="on"
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
                    :disabled="tag.Referenced"
                    icon
                    v-on="on"
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
  </div>
</template>

<script lang="ts" setup>
import { computed, onMounted } from "vue";
import { EventBus } from "./EventBus";
import { useRootStore } from "@/stores";
import { tagForURI } from "@/filters";
import ToolBar from "./ToolBar.vue";
import { isDarkColor } from "@/lib/colors"

const tagTypes = [
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
];
const store = useRootStore();
const groupedTags = computed(() => store.groupedTags);
const tags = computed(() => store.tags);
const converterList = computed(() => {
  if (tags.value === null) return {};
  return tags.value.reduce((acc: { [key: string]: string }, tag) => {
    acc[tag.Name] = tag.Converters.join(", ");
    return acc;
  }, {});
});

onMounted(() => {
  updateTags();
});

function updateTags() {
  store.updateTags().catch((err: string) => {
    EventBus.emit("showError", `Failed to update tags: ${err}`);
  });
}

function setupCTF() {
  EventBus.emit("showCTFWizard");
}

function showTagSetConvertersDialog(tagId: string) {
  EventBus.emit("showTagSetConvertersDialog", tagId);
}

function confirmTagDeletion(tagId: string) {
  EventBus.emit("showTagDeleteDialog", tagId);
}

function setQuery(query: string) {
  EventBus.emit("setSearchTerm", query);
}

function showTagColorChangeDialog(tagId: string) {
  EventBus.emit("showTagColorChangeDialog", tagId);
}

function showTagDefinitionChangeDialog(tagId: string) {
  EventBus.emit("showTagDefinitionChangeDialog", tagId);
}

function showTagNameChangeDialog(tagId: string) {
  EventBus.emit("showTagNameChangeDialog", tagId);
}
</script>

<style scoped>
.tag_definition {
  word-break: break-all;
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 3;
  overflow: hidden;
}
</style>
