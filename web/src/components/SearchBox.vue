<template>
  <div>
    <v-text-field
      id="searchBoxField"
      autofocus
      flat
      variant="underlined"
      color="primary"
      prepend-inner-icon="mdi-magnify"
      :model-value="searchBox"
      @update:model-value="onInput"
      @click.stop
      @keyup.enter="onEnter"
      @keydown.up.prevent="arrowUp"
      @keydown.down.prevent="arrowDown"
      @keydown.tab.exact.prevent.stop="onTab"
      @keydown.esc.exact="suggestionMenuOpen = false"
      @focus="searchBoxOptionsMenuOpen = true"
    >
      <template #append-inner>
        <v-menu location="bottom">
          <template #activator="{ props }">
            <v-btn size="small" icon v-bind="props" class="mt-n2"
              ><v-icon>mdi-dots-vertical</v-icon></v-btn
            >
          </template>
          <v-list density="compact">
            <v-list-item
              prepend-icon="mdi-magnify"
              link
              @click="search('search')"
            >
              <v-list-item-title>Search</v-list-item-title>
            </v-list-item>
            <v-list-item
              prepend-icon="mdi-finance"
              link
              @click="search('graph')"
            >
              <v-list-item-title>Graph</v-list-item-title>
            </v-list-item>
            <v-list-item
              prepend-icon="mdi-cloud-outline"
              link
              @click="createTag('service', searchBox)"
            >
              <v-list-item-title>Save as Service</v-list-item-title>
            </v-list-item>
            <v-list-item
              prepend-icon="mdi-tag-multiple-outline"
              link
              @click="createTag('tag', searchBox)"
            >
              <v-list-item-title>Save as Tag</v-list-item-title>
            </v-list-item>
          </v-list>
        </v-menu>
      </template>
    </v-text-field>
    <v-menu
      v-model="searchBoxOptionsMenuOpen"
      :close-on-content-click="false"
      open-on-focus
      absolute
      :target="[searchBoxFieldRect.left, searchBoxFieldRect.bottom]"
      :min-width="searchBoxFieldRect.width"
      :max-width="searchBoxFieldRect.width"
    >
      <v-card>
        <v-btn-toggle
          v-model="queryTimeLimit"
          color="primary"
          density="compact"
          group
        >
          <v-btn variant="text" value="-5m:">Limit to last 5m</v-btn>
          <v-btn variant="text" value="-1h:">Limit to last 1h</v-btn>
        </v-btn-toggle>
      </v-card>
    </v-menu>
    <v-menu
      v-model="suggestionMenuOpen"
      :target="[suggestionMenuPosX, suggestionMenuPosY]"
      absolute
      density="compact"
    >
      <v-list :value="suggestionSelectedIndex" color="primary" mandatory>
        <v-list-item
          v-for="(item, index) in suggestionItems"
          :key="index"
          :active="index === suggestionSelectedIndex"
          active-class="selected-suggestion"
          :style="{ backgroundColor: suggestionColor(suggestionType, item) }"
          @click="applySuggestion(index)"
        >
          <v-list-item-title>{{ item }}</v-list-item-title>
        </v-list-item>
      </v-list>
    </v-menu>
  </div>
</template>

<script lang="ts" setup>
import { EventBus } from "./EventBus";
import { addSearch, getTermAt } from "./searchHistory";
import suggest from "@/parser/suggest";
import analyze from "@/parser/analyze";
import {
  computed,
  nextTick,
  ref,
  onMounted,
  onBeforeUnmount,
  watch,
} from "vue";
import { useRoute, useRouter } from "vue-router";
import { useRootStore } from "@/stores";
import { tagNameForURI } from "@/filters";

const store = useRootStore();
const route = useRoute();
const router = useRouter();
const config = computed(() => store.clientConfig);
const searchBoxField = ref<HTMLInputElement | null>(null);
const searchBox = ref<string>((route.query.q as string) ?? "");
const historyIndex = ref(-1);
const pendingSearch = ref("");
const typingDelay = ref<number | null>(null);
const suggestionItems = ref<string[]>([]);
const suggestionStart = ref(0);
const suggestionEnd = ref(0);
const suggestionType = ref("tag");
const suggestionSelectedIndex = ref(0);
const suggestionMenuOpen = ref(false);
const searchBoxOptionsMenuOpen = ref(false);
const suggestionMenuPosX = ref(0);
const suggestionMenuPosY = ref(0);
const queryTimeLimit = computed({
  get(): string | undefined {
    const ltime = analyze(searchBox.value).ltime?.[0] ?? {};
    const v = ltime?.pieces?.value;
    if (v !== undefined && ["-5m:", "-1h:"].includes(v)) return v;
    return undefined;
  },
  set(val: string | undefined) {
    const q = searchBox.value ?? "";
    const ltime = analyze(q).ltime?.[0] ?? {};
    const old = ltime?.pieces?.value;
    if (old === val) return;
    const infix = val ? `ltime:${val}` : "";
    if (old === undefined) {
      if (q === "" || q.endsWith(" ")) {
        searchBox.value = `${q}${infix}`;
      } else {
        searchBox.value = `${q} ${infix}`;
      }
    } else {
      const prefix = q.slice(0, ltime.start);
      const suffix = q.slice(ltime.start + ltime.len);
      searchBox.value = `${prefix}${infix}${suffix}`;
    }
    searchBoxField.value?.focus();
    nextTick(() => {
      searchBoxOptionsMenuOpen.value = true;
    }).catch((err: Error) => {
      EventBus.emit(
        "showError",
        `Failed to open search box options: ${err.message}`,
      );
    });
  },
});
const tagColors = computed(() => {
  const tags: { [key: string]: { [key: string]: string } } = {};
  if (store.tags == null) return tags;
  store.tags.forEach((tag) => {
    const type = tag.Name.split("/", 1)[0];
    const name = tag.Name.substr(type.length + 1);
    if (!(type in tags)) {
      tags[type] = {};
    }
    tags[type][name] = tag.Color;
  });
  return tags;
});
const searchBoxFieldRect = computed(() => {
  if (!searchBoxField.value) {
    return {
      width: 0,
      height: 0,
      left: 0,
      right: 0,
      top: 0,
      bottom: 0,
    };
  }
  return searchBoxField.value.getBoundingClientRect();
});

EventBus.on("setSearchTerm", setSearchTerm);

watch(
  route,
  () => {
    setSearchBox(route.query.q as string);
  },
  { immediate: true },
);
watch(
  suggestionItems,
  () => {
    suggestionMenuOpen.value = suggestionItems.value.length > 0;
    if (suggestionMenuOpen.value) {
      suggestionSelectedIndex.value = 0;
      const searchBoxElement = searchBoxField.value;
      const cursorIndex = searchBoxElement?.selectionStart ?? null;
      if (cursorIndex === null) return;
      const fontWidth = 7.05; // @TODO: Calculate the absolute cursor position correctly
      suggestionMenuPosX.value =
        cursorIndex * fontWidth +
        (searchBoxElement?.getBoundingClientRect().left ?? 0);
    }
  },
  { immediate: true },
);

onMounted(() => {
  searchBoxField.value = document.getElementById(
    "searchBoxField",
  ) as HTMLInputElement;
  store.updateConverters().catch((err: Error) => {
    EventBus.emit("showError", `Failed to update converters: ${err.message}`);
  });

  const keyListener = (e: KeyboardEvent) => {
    if (e.target === null || !(e.target instanceof Element)) return;
    if (["input", "textarea"].includes(e.target.tagName.toLowerCase())) return;
    if (e.key != "/") return;
    e.preventDefault();
    searchBoxField.value?.focus();
  };
  document.body.addEventListener("keydown", keyListener);
  onBeforeUnmount(() => {
    document.body.removeEventListener("keydown", keyListener);
  });
  suggestionMenuPosY.value = searchBoxFieldRect.value.bottom ?? 0;
});

function onTab() {
  if (suggestionMenuOpen.value) {
    applySuggestion();
  } else {
    startSuggestionSearch();
  }
}

function onInput(updatedText: string) {
  historyIndex.value = -1;
  setSearchBox(updatedText);
  startSuggestionSearch();
}

function onEnter() {
  if (suggestionMenuOpen.value) {
    applySuggestion();
  } else {
    search(null);
  }
}

function setSearchBox(value: string) {
  searchBox.value = value;
  abortSuggestionSearch();
}

function setSearchTerm(searchTerm: string) {
  setSearchBox(searchTerm);
}

function applySuggestion(index: number | null = null) {
  let replace = suggestionItems.value[index ?? suggestionSelectedIndex.value];
  if (replace === null || searchBox.value === null) {
    return;
  }
  replace = tagNameForURI(replace);
  const prefix = searchBox.value.substring(0, suggestionStart.value);
  const suffix = searchBox.value.substring(suggestionEnd.value);
  searchBox.value = prefix + replace + suffix;
  suggestionMenuOpen.value = false;
}

function startSuggestionSearch() {
  const val = searchBox.value;
  typingDelay.value = window.setTimeout(() => {
    const cursorPosition = searchBoxField.value?.selectionStart ?? 0;
    const suggestionResult = suggest(
      val,
      cursorPosition,
      store.groupedTags,
      store.converters,
    );
    suggestionItems.value = suggestionResult.suggestions;
    suggestionStart.value = suggestionResult.start;
    suggestionEnd.value = suggestionResult.end;
    suggestionType.value = suggestionResult.type;
  }, 200);
}

function abortSuggestionSearch() {
  if (typingDelay.value) {
    clearTimeout(typingDelay.value);
    suggestionItems.value = [];
    typingDelay.value = null;
  }
}

function suggestionColor(type: string, item: string) {
  if (type === "data") {
    return "#ffffff";
  }
  return tagColors.value[type][item];
}

function arrowUp() {
  if (suggestionMenuOpen.value) {
    menuUp();
  } else {
    historyUp();
  }
}

function arrowDown() {
  if (suggestionMenuOpen.value) {
    menuDown();
  } else {
    historyDown();
  }
}

function menuDown() {
  selectSuggestionIndex(suggestionSelectedIndex.value + 1);
}

function menuUp() {
  selectSuggestionIndex(suggestionSelectedIndex.value - 1);
}

function selectSuggestionIndex(index: number) {
  suggestionSelectedIndex.value = Math.min(
    Math.max(index, 0),
    suggestionItems.value.length - 1,
  );
}

function historyUp() {
  if (historyIndex.value === -1) {
    pendingSearch.value = searchBox.value;
  }
  const term = getTermAt(historyIndex.value + 1);
  if (term == null) {
    return;
  }
  historyIndex.value++;
  if (pendingSearch.value === term) {
    historyUp();
    return;
  }
  setSearchBox(term);
}

function historyDown() {
  if (historyIndex.value === -1) {
    return;
  }
  historyIndex.value--;
  setSearchBox(
    historyIndex.value === -1
      ? pendingSearch.value
      : getTermAt(historyIndex.value),
  );
}

function search(type: string | null) {
  let q: typeof route.query = {};
  if (!type) {
    type = route.name == "graph" ? "graph" : "search";
    if (type == "graph")
      q = JSON.parse(JSON.stringify(route.query)) as typeof route.query;
  }
  q.q = searchBox.value;
  if (config.value?.AutoInsertLimitToQuery) {
    q.manualSearch = "true";
  }
  addSearch(searchBox.value);
  historyIndex.value = -1;
  const newQuery = {
    name: type,
    query: q,
  };
  void router.push(newQuery).catch((e) => console.warn(e));
}

function createTag(tagType: string, tagQuery: string) {
  EventBus.emit("showCreateTagDialog", tagType, tagQuery, []);
}
</script>

<style scoped>
.selected-suggestion div.v-list-item-title {
  color: black;
  font-weight: bolder;
}
</style>
