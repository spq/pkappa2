<template>
  <v-list
    density="compact"
    nav
    v-model:opened="tagTypesKeys"
    open-strategy="multiple"
  >
    <v-list-item
      variant="flat"
      link
      density="compact"
      exact
      :to="{ name: 'home' }"
    >
      <template #prepend>
        <v-icon size="small">mdi-help-circle-outline</v-icon>
      </template>
      <v-list-item-title>Help</v-list-item-title>
    </v-list-item>
    <v-list-item
      link
      density="compact"
      exact
      :to="{ name: 'search', query: { q: '' } }"
    >
      <template #prepend>
        <v-icon size="small">mdi-all-inclusive</v-icon>
      </template>
      <v-list-item-title>All Streams</v-list-item-title>

      <template #append>
        <v-list-item-action v-if="status != null"
          ><v-chip variant="flat" size="x-small">{{
            status.StreamCount
          }}</v-chip></v-list-item-action
        >
      </template>
    </v-list-item>
    <v-list-group
      v-for="tagType in tagTypes"
      :key="tagType.key"
      link
      density="compact"
      :value="tagType.key"
    >
      <template #activator="{ props }">
        <v-list-item v-bind="props" link>
          <template #prepend>
            <v-icon size="small">mdi-{{ tagType.icon }}</v-icon>
          </template>
          <v-list-item-title>{{ tagType.title }}</v-list-item-title>
        </v-list-item>
      </template>
      <template v-for="tag in groupedTags[tagType.key]" :key="tag.Name">
        <v-hover v-slot="{ isHovering, props: hoverProps }">
          <v-list-item
            link
            density="compact"
            exact
            v-bind="hoverProps"
            :style="{ backgroundColor: tag.Color }"
            :to="{
              name: 'search',
              query: {
                q: tagForURI(tag.Name),
              },
            }"
          >
            <v-list-item-title
              class="text-truncate"
              style="max-width: 110px"
              :style="{ color: getContrastTextColor(tag.Color) }"
              :title="tag.Name.substring(tagType.key.length + 1)"
              >{{
                tag.Name.substring(tagType.key.length + 1)
              }}</v-list-item-title
            >
            <template #append>
              <v-menu location="bottom left" open-on-hover>
                <template #activator="{ props }">
                  <v-list-item-action v-bind="props">
                    <v-btn
                      v-if="isHovering"
                      icon
                      size="x-small"
                      variant="plain"
                      :style="{
                        color: getContrastTextColor(tag.Color),
                      }"
                    >
                      <v-icon>mdi-dots-vertical</v-icon>
                    </v-btn>
                    <v-chip v-else size="x-small" variant="flat"
                      >{{ tag.MatchingCount
                      }}{{ tag.UncertainCount != 0 ? "+" : "" }}</v-chip
                    >
                  </v-list-item-action>
                </template>
                <v-list density="compact">
                  <v-list-item
                    link
                    exact
                    :to="{
                      name: 'search',
                      query: {
                        q: tagForURI(tag.Name),
                      },
                    }"
                    prepend-icon="mdi-magnify"
                  >
                    <v-list-item-title>Show Streams</v-list-item-title>
                  </v-list-item>
                  <v-list-item
                    prepend-icon="mdi-clipboard-list-outline"
                    link
                    @click="showTagDetailsDialog(tag.Name)"
                  >
                    <v-list-item-title>Details</v-list-item-title>
                  </v-list-item>
                  <v-list-item
                    prepend-icon="mdi-form-textbox"
                    link
                    @click="setQuery(tag.Definition)"
                  >
                    <v-list-item-title>Use Query</v-list-item-title>
                  </v-list-item>
                  <v-list-item
                    prepend-icon="mdi-palette"
                    link
                    @click="showTagColorChangeDialog(tag.Name)"
                  >
                    <v-list-item-title>Change Color</v-list-item-title>
                  </v-list-item>
                  <v-list-item
                    prepend-icon="mdi-rename-outline"
                    link
                    @click="showTagNameChangeDialog(tag.Name)"
                  >
                    <v-list-item-title>Change Name</v-list-item-title>
                  </v-list-item>
                  <v-list-item
                    prepend-icon="mdi-text-search-variant"
                    link
                    @click="showTagDefinitionChangeDialog(tag.Name)"
                  >
                    <v-list-item-title>Change Definition</v-list-item-title>
                  </v-list-item>
                  <v-list-item
                    prepend-icon="mdi-file-replace-outline"
                    link
                    @click="showTagSetConvertersDialog(tag.Name)"
                  >
                    <v-list-item-title>Attach converter</v-list-item-title>
                  </v-list-item>
                  <v-list-item
                    prepend-icon="mdi-delete-outline"
                    link
                    :disabled="tag.Referenced"
                    @click="confirmTagDeletion(tag.Name)"
                  >
                    <v-list-item-title>Delete</v-list-item-title>
                  </v-list-item>
                </v-list>
              </v-menu>
            </template>
          </v-list-item>
        </v-hover>
      </template>
    </v-list-group>
    <v-list-group v-model="moreOpen" link dense subgroup>
      <template #activator="{ props }">
        <v-list-item v-bind="props" link density="compact">
          <template #prepend>
            <v-icon size="small"
              >mdi-chevron-{{ moreOpen ? "up" : "down" }}</v-icon
            >
          </template>

          <v-list-item-title>More</v-list-item-title>
        </v-list-item>
      </template>
      <v-list-item
        link
        density="compact"
        exact
        :to="{
          name: 'status',
        }"
      >
        <v-list-item-title>Status</v-list-item-title>
      </v-list-item>
      <v-list-item
        link
        density="compact"
        exact
        :to="{
          name: 'pcaps',
        }"
      >
        <v-list-item-title>PCAPs</v-list-item-title>
      </v-list-item>
      <v-list-item
        link
        density="compact"
        exact
        :to="{
          name: 'tags',
        }"
      >
        <v-list-item-title>Manage Tags</v-list-item-title>
      </v-list-item>
      <v-list-item
        link
        density="compact"
        exact
        :to="{
          name: 'converters',
        }"
      >
        <v-list-item-title>Manage Converters</v-list-item-title>
      </v-list-item>
      <v-list-item
        link
        density="compact"
        exact
        :to="{
          name: 'pcap-over-ip',
        }"
      >
        <v-list-item-title>Manage PCAP-over-IP</v-list-item-title>
      </v-list-item>

      <v-btn-toggle v-model="colorscheme" mandatory class="pl-9 pt-2">
        <v-btn>
          <v-icon>mdi-weather-sunny</v-icon>
        </v-btn>
        <v-btn>
          <v-icon>mdi-cog-outline</v-icon>
        </v-btn>
        <v-btn>
          <v-icon>mdi-weather-night</v-icon>
        </v-btn>
      </v-btn-toggle>
    </v-list-group>
  </v-list>
</template>

<script lang="ts" setup>
import { useRoute } from "vue-router";
import {
  setColorScheme,
  getColorSchemeFromStorage,
  ColorSchemeConfiguration,
} from "@/lib/darkmode";
import { EventBus } from "./EventBus";
import { useRootStore } from "@/stores";
import { tagForURI } from "@/filters";
import { computed, onMounted, ref, watch } from "vue";
import { getContrastTextColor } from "@/lib/colors";

type ColorSchemeButtonTriState = 0 | 1 | 2;

const store = useRootStore();
const route = useRoute();
const schemeInitialisations: Record<
  ColorSchemeConfiguration,
  ColorSchemeButtonTriState
> = {
  light: 0,
  system: 1,
  dark: 2,
};
const colorscheme = ref<ColorSchemeButtonTriState>(
  schemeInitialisations[getColorSchemeFromStorage()],
);
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
const tagTypesKeys = ref(tagTypes.map((tagType) => tagType.key));

const moreOpen =
  route.name !== null &&
  route.name !== undefined &&
  ["converters", "status", "tags", "pcaps"].includes(route.name.toString()); // FIXME: type route
const groupedTags = computed(() => store.groupedTags);
const status = computed(() => store.status);

watch(colorscheme, () => {
  const schemes: Record<ColorSchemeButtonTriState, ColorSchemeConfiguration> = {
    0: "light",
    1: "system",
    2: "dark",
  };
  setColorScheme(schemes[colorscheme.value]);
});

onMounted(() => {
  store
    .updateTags()
    .then(() => {
      if (store.tags?.length === 0) EventBus.emit("showCTFWizard");
    })
    .catch((err: Error) => {
      EventBus.emit("showError", `Failed to update tags: ${err.message}`);
    });
  store.updateStatus().catch((err: Error) => {
    EventBus.emit("showError", `Failed to update status: ${err.message}`);
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

function showTagDefinitionChangeDialog(tagId: string) {
  EventBus.emit("showTagDefinitionChangeDialog", tagId);
}

function showTagNameChangeDialog(tagId: string) {
  EventBus.emit("showTagNameChangeDialog", tagId);
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
