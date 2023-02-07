<template>
  <div>
    <v-text-field
      ref="searchBox"
      autofocus
      hide-details
      flat
      prepend-inner-icon="mdi-magnify"
      :value="searchBox"
      @input="onInput"
      @click.stop
      @keyup.enter="onEnter"
      @keydown.up.prevent="arrowUp"
      @keydown.down.prevent="arrowDown"
      @keydown.tab.exact.prevent.stop="onTab"
      @keydown.esc.exact="suggestionMenuOpen = false"
    >
      <template #append>
        <v-menu offset-y right bottom>
          <template #activator="{ on, attrs }">
            <v-btn small icon v-bind="attrs" v-on="on"
              ><v-icon>mdi-dots-vertical</v-icon></v-btn
            >
          </template>
          <v-list dense>
            <v-list-item link @click="search('search')">
              <v-list-item-icon><v-icon>mdi-magnify</v-icon></v-list-item-icon>
              <v-list-item-title>Search</v-list-item-title>
            </v-list-item>
            <v-list-item link @click="search('graph')">
              <v-list-item-icon><v-icon>mdi-finance</v-icon></v-list-item-icon>
              <v-list-item-title>Graph</v-list-item-title>
            </v-list-item>
            <v-list-item link @click="createTag('service', searchBox)">
              <v-list-item-icon
                ><v-icon>mdi-cloud-outline</v-icon></v-list-item-icon
              >
              <v-list-item-title>Save as Service</v-list-item-title>
            </v-list-item>
            <v-list-item link @click="createTag('tag', searchBox)">
              <v-list-item-icon
                ><v-icon>mdi-tag-multiple-outline</v-icon></v-list-item-icon
              >
              <v-list-item-title>Save as Tag</v-list-item-title>
            </v-list-item>
          </v-list>
        </v-menu>
      </template>
    </v-text-field>
    <v-menu
      ref="suggestionMenu"
      v-model="suggestionMenuOpen"
      :position-x="suggestionMenuPosX"
      :position-y="suggestionMenuPosY"
      absolute
      dense
    >
      <v-list>
        <v-list-item-group
          :value="suggestionSelectedIndex"
          color="primary"
          mandatory
        >
          <v-list-item
            v-for="(item, index) in suggestionItems"
            :key="index"
            active-class="font-white"
            :style="{ backgroundColor: tagColors[suggestionType][item] }"
            @click="applySuggestion(index)"
          >
            <v-list-item-title>{{ item }}</v-list-item-title>
          </v-list-item>
        </v-list-item-group>
      </v-list>
    </v-menu>
  </div>
</template>

<script>
import { EventBus } from "./EventBus";
import { addSearch, getTermAt } from "./searchHistory";
import { mapGetters, mapState } from "vuex";
import suggest from "../parser/suggest";

export default {
  name: "SearchBox",
  data() {
    return {
      searchBox: this.$route.query.q,
      historyIndex: -1,
      pendingSearch: "",
      typingDelay: null,
      suggestionItems: [],
      suggestionStart: 0,
      suggestionEnd: 0,
      suggestionType: "tag",
      suggestionSelectedIndex: 0,
      suggestionMenuOpen: false,
      suggestionMenuPosX: 0,
      suggestionMenuPosY: 0,
    };
  },
  computed: {
    ...mapState(["tags"]),
    ...mapGetters(["groupedTags"]),
    tagColors() {
      const tags = {};
      this.tags.forEach((tag) => {
        const type = tag.Name.split("/", 1)[0];
        const name = tag.Name.substr(type.length + 1);
        if (!(type in tags)) {
          tags[type] = {};
        }
        tags[type][name] = tag.Color;
      });

      return tags;
    },
  },
  watch: {
    "$route.query.q": function (term) {
      this.setSearchBox(term);
    },
    suggestionItems() {
      this.suggestionMenuOpen = this.suggestionItems.length > 0;
      if (this.suggestionMenuOpen) {
        this.suggestionSelectedIndex = 0;
        const cursorIndex = this.$refs.searchBox.$refs.input.selectionStart;
        const fontWidth = 7.05; // @TODO: Calculate the absolute cursor position correctly
        this.suggestionMenuPosX =
          cursorIndex * fontWidth +
          this.$refs.searchBox.$el.getBoundingClientRect().left;
      }
    },
  },
  created() {
    EventBus.$on("setSearchTerm", this.setSearchTerm);
  },
  mounted() {
    this._keyListener = (e) => {
      if (["input", "textarea"].includes(e.target.tagName.toLowerCase()))
        return;
      if (e.key != "/") return;

      e.preventDefault();
      this.$refs.searchBox.focus();
    };
    document.body.addEventListener("keydown", this._keyListener.bind(this));
    this.suggestionMenuPosY =
      this.$refs.searchBox.$el.getBoundingClientRect().bottom;
  },
  beforeDestroy() {
    document.body.removeEventListener("keydown", this._keyListener);
  },
  methods: {
    onTab() {
      if (this.suggestionMenuOpen) {
        this.applySuggestion();
      } else {
        this.startSuggestionSearch();
      }
    },
    onInput(updatedText) {
      this.historyIndex = -1;
      this.setSearchBox(updatedText);
      this.startSuggestionSearch();
    },
    onEnter() {
      if (this.suggestionMenuOpen) {
        this.applySuggestion();
      } else {
        this.search(null);
      }
    },
    applySuggestion(index = null) {
      let replace = this.suggestionItems[index ?? this.suggestionSelectedIndex];
      if (null == replace) {
        return;
      }
      replace = this.$options.filters.tagNameForURI(replace);
      const prefix = this.searchBox.substring(0, this.suggestionStart);
      const suffix = this.searchBox.substring(this.suggestionEnd);
      this.searchBox = prefix + replace + suffix;
      this.suggestionMenuOpen = false;
    },
    startSuggestionSearch() {
      const val = this.searchBox;
      this.typingDelay = setTimeout(() => {
        const cursorPosition = this.$refs.searchBox.$refs.input.selectionStart;
        const suggestionResult = suggest(val, cursorPosition, this.groupedTags);
        this.suggestionItems = suggestionResult.suggestions;
        this.suggestionStart = suggestionResult.start;
        this.suggestionEnd = suggestionResult.end;
        this.suggestionType = suggestionResult.type;
      }, 200);
    },
    abortSuggestionSearch() {
      if (this.typingDelay) {
        clearTimeout(this.typingDelay);
        this.suggestionItems = [];
        this.typingDelay = null;
      }
    },
    arrowUp() {
      if (this.suggestionMenuOpen) {
        this.menuUp();
      } else {
        this.historyUp();
      }
    },
    arrowDown() {
      if (this.suggestionMenuOpen) {
        this.menuDown();
      } else {
        this.historyDown();
      }
    },
    menuDown() {
      this.selectSuggestionIndex(this.suggestionSelectedIndex + 1);
    },
    menuUp() {
      this.selectSuggestionIndex(this.suggestionSelectedIndex - 1);
    },
    selectSuggestionIndex(index) {
      this.suggestionSelectedIndex = Math.min(
        Math.max(index, 0),
        this.suggestionItems.length - 1
      );
    },
    historyUp() {
      if (this.historyIndex === -1) {
        this.pendingSearch = this.searchBox;
      }
      let term = getTermAt(this.historyIndex + 1);
      if (term == null) {
        return;
      }
      this.historyIndex++;
      if (this.pendingSearch === term) {
        this.historyUp();
        return;
      }
      this.setSearchBox(term);
    },
    historyDown() {
      if (this.historyIndex === -1) {
        return;
      }
      this.historyIndex--;
      this.setSearchBox(
        this.historyIndex === -1
          ? this.pendingSearch
          : getTermAt(this.historyIndex)
      );
    },
    search(type) {
      let q = {};
      if (!type) {
        type = this.$route.name == "graph" ? "graph" : "search";
        if (type == "graph") q = JSON.parse(JSON.stringify(this.$route.query));
      }
      q.q = this.searchBox;
      addSearch(this.searchBox);
      this.historyIndex = -1;
      this.$router.push({
        name: type,
        query: q,
      });
    },
    setSearchBox(value) {
      this.searchBox = value;
      this.abortSuggestionSearch();
    },
    setSearchTerm({ searchTerm }) {
      this.setSearchBox(searchTerm);
    },
    createTag(tagType, tagQuery) {
      EventBus.$emit("showCreateTagDialog", { tagType, tagQuery });
    },
  },
};
</script>

<style scoped>
.font-white {
  color: black;
  font-weight: bold;
}
</style>
