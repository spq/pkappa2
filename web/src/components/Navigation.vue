<template>
  <v-list dense nav>
    <v-list-item link dense exact :to="{ name: 'home' }">
      <v-list-item-icon></v-list-item-icon>
      <v-list-item-icon>
        <v-icon dense>mdi-help-circle-outline</v-icon>
      </v-list-item-icon>
      <v-list-item-content>
        <v-list-item-title>Help</v-list-item-title>
      </v-list-item-content>
    </v-list-item>
    <v-list-item link dense exact :to="{ name: 'search', query: { q: '' } }">
      <v-list-item-icon></v-list-item-icon>
      <v-list-item-icon>
        <v-icon dense>mdi-all-inclusive</v-icon>
      </v-list-item-icon>
      <v-list-item-content>
        <v-list-item-title>All Streams</v-list-item-title>
      </v-list-item-content>
      <v-list-item-action v-if="status != null"
        ><v-chip x-small>{{ status.StreamCount }}</v-chip></v-list-item-action
      >
    </v-list-item>
    <v-list-group
      v-for="tagType in tagTypes"
      :key="tagType.key"
      link
      dense
      :value="true"
      sub-group
    >
      <template #activator>
        <v-list-item-icon>
          <v-icon dense>mdi-{{ tagType.icon }}</v-icon>
        </v-list-item-icon>
        <v-list-item-content>
          <v-list-item-title>{{ tagType.title }}</v-list-item-title>
        </v-list-item-content>
      </template>
      <template v-for="tag in groupedTags[tagType.key]">
        <v-hover
          v-slot="{ hover }"
          :key="tag.Name"
          :style="{ backgroundColor: tag.Color }"
        >
          <v-list-item
            link
            dense
            exact
            :to="{
              name: 'search',
              query: {
                q: $options.filters?.tagForURI(tag.Name),
              },
            }"
          >
            <v-list-item-content>
              <v-list-item-title
                class="text-truncate"
                style="max-width: 110px"
                :title="tag.Name.substr(tagType.key.length + 1)"
                >{{
                  tag.Name.substr(tagType.key.length + 1)
                }}</v-list-item-title
              >
            </v-list-item-content>
            <v-menu offset-y bottom open-on-hover right>
              <template #activator="{ on, attrs }">
                <v-list-item-action v-bind="attrs" v-on="on">
                  <v-btn v-if="hover" icon x-small>
                    <v-icon>mdi-dots-vertical</v-icon>
                  </v-btn>
                  <v-chip v-else x-small
                    >{{ tag.MatchingCount
                    }}{{ tag.UncertainCount != 0 ? "+" : "" }}</v-chip
                  >
                </v-list-item-action>
              </template>
              <v-list dense>
                <v-list-item
                  link
                  exact
                  :to="{
                    name: 'search',
                    query: {
                      q: $options.filters?.tagForURI(tag.Name),
                    },
                  }"
                >
                  <v-list-item-icon>
                    <v-icon>mdi-magnify</v-icon>
                  </v-list-item-icon>
                  <v-list-item-title>Show Streams</v-list-item-title>
                </v-list-item>
                <v-list-item link @click="showTagDetailsDialog(tag.Name)">
                  <v-list-item-icon>
                    <v-icon>mdi-clipboard-list-outline</v-icon>
                  </v-list-item-icon>
                  <v-list-item-title>Details</v-list-item-title>
                </v-list-item>
                <v-list-item link @click="setQuery(tag.Definition)">
                  <v-list-item-icon>
                    <v-icon>mdi-form-textbox</v-icon>
                  </v-list-item-icon>
                  <v-list-item-title>Use Query</v-list-item-title>
                </v-list-item>
                <v-list-item link @click="showTagColorChangeDialog(tag.Name)">
                  <v-list-item-icon>
                    <v-icon>mdi-palette</v-icon>
                  </v-list-item-icon>
                  <v-list-item-title>Change Color</v-list-item-title>
                </v-list-item>
                <v-list-item link @click="showTagSetConvertersDialog(tag.Name)">
                  <v-list-item-icon>
                    <v-icon>mdi-file-replace-outline</v-icon>
                  </v-list-item-icon>
                  <v-list-item-title>Attach converter</v-list-item-title>
                </v-list-item>
                <v-list-item
                  link
                  :disabled="tag.Referenced"
                  @click="confirmTagDeletion(tag.Name)"
                >
                  <v-list-item-icon>
                    <v-icon>mdi-delete-outline</v-icon>
                  </v-list-item-icon>
                  <v-list-item-title>Delete</v-list-item-title>
                </v-list-item>
              </v-list>
            </v-menu>
          </v-list-item>
        </v-hover>
      </template>
    </v-list-group>
    <v-list-group v-model="moreOpen" link dense sub-group>
      <template #activator>
        <v-list-item-icon>
          <v-icon dense>mdi-chevron-{{ moreOpen ? "up" : "down" }}</v-icon>
        </v-list-item-icon>
        <v-list-item-content>
          <v-list-item-title>More</v-list-item-title>
        </v-list-item-content>
      </template>
      <v-list-item
        link
        dense
        exact
        :to="{
          name: 'status',
        }"
      >
        <v-list-item-content>
          <v-list-item-title>Status</v-list-item-title>
        </v-list-item-content>
      </v-list-item>
      <v-list-item
        link
        dense
        exact
        :to="{
          name: 'pcaps',
        }"
      >
        <v-list-item-content>
          <v-list-item-title>PCAPs</v-list-item-title>
        </v-list-item-content>
      </v-list-item>
      <v-list-item
        link
        dense
        exact
        :to="{
          name: 'tags',
        }"
      >
        <v-list-item-content>
          <v-list-item-title>Manage Tags</v-list-item-title>
        </v-list-item-content>
      </v-list-item>
      <v-list-item
        link
        dense
        exact
        :to="{
          name: 'converters',
        }"
      >
        <v-list-item-content>
          <v-list-item-title>Manage Converters</v-list-item-title>
        </v-list-item-content>
      </v-list-item>
    </v-list-group>
  </v-list>
</template>

<script lang="ts" setup>
import { useRoute } from "vue-router/composables";
import { EventBus } from "./EventBus";
import { useRootStore } from "@/stores";
import { computed, onMounted } from "vue";

const store = useRootStore();
const route = useRoute();
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

const moreOpen =
  route.name !== null &&
  route.name !== undefined &&
  ["converters", "status", "tags", "pcaps"].includes(route.name); // FIXME: type route
const groupedTags = computed(() => store.groupedTags);
const status = computed(() => store.status);

onMounted(() => {
  store.updateTags().catch((err: string) => {
    EventBus.emit("showError", `Failed to update tags: ${err}`);
  });
  store.updateStatus().catch((err: string) => {
    EventBus.emit("showError", `Failed to update status: ${err}`);
  });
});

function showTagSetConvertersDialog(tagId: string) {
  EventBus.emit("showTagSetConvertersDialog", tagId);
}

function confirmTagDeletion(tagId: string) {
  EventBus.emit("showTagDeleteDialog", tagId);
}

function showTagDetailsDialog(tagId: string) {
  EventBus.emit("showTagDetailsDialog", tagId);
}

function setQuery(query: string) {
  EventBus.emit("setSearchTerm", query);
}

function showTagColorChangeDialog(tagId: string) {
  EventBus.emit("showTagColorChangeDialog", tagId);
}
</script>

<style>
.v-application--is-ltr
  .v-navigation-drawer
  .v-list-item__icon.v-list-group__header__prepend-icon {
  display: none;
}

.v-application--is-ltr .v-navigation-drawer .v-list-item__icon:first-child {
  display: none;
}

.v-application--is-ltr
  .v-navigation-drawer
  .v-list-group--sub-group
  .v-list-group__header {
  padding-left: 8px;
}

.v-application--is-ltr .v-navigation-drawer .v-list-item__action {
  margin-top: 0;
  margin-bottom: 0;
}
</style>
