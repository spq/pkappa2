<template>
  <div>
    <v-text-field
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
      @keydown.esc.exact="menuOpen = false"
      ref="searchBox"
    >
      <template #append>
        <v-menu offset-y right bottom>
          <template #activator="{ on, attrs }">
            <v-btn small icon v-on="on" v-bind="attrs"
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
      :position-x="menuPosX"
      :position-y="menuPosY"
      ref="suggestionMenu"
      v-model="menuOpen"
      absolute
      dense
    >
      <v-list>
        <v-list-item-group
          :value="selectedAutocompleteIndex"
          color="primary"
          mandatory
        >
          <v-list-item
            v-for="(item, index) in autocompleteItems"
            :key="index"
            @click="applyAutocomplete(index)"
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
import {addSearch, getTermAt} from './searchHistory';
import { mapGetters } from "vuex";
import suggest from '../../parser/suggest'

export default {
  name: "SearchBox",
  data() {
    return {
      searchBox: this.$route.query.q,
      historyIndex: -1,
      pendingSearch: '',
      typingDelay: null,
      autocompleteItems: [],
      suggestionStart: 0,
      suggestionEnd: 0,
      menuOpen: false,
      menuPosX: 0,
      menuPosY: 0,
      selectedAutocompleteIndex: 0,
    };
  },
  computed: {
    ...mapGetters(["groupedTags"]),
  },
  watch: {
    "$route.query.q": function (term) {
      this.setSearchBox(term);
    },
    autocompleteItems() {
      this.menuOpen = this.autocompleteItems.length > 0;
      if (this.menuOpen) {
        this.selectedAutocompleteIndex = 0;
        const cursorIndex = this.$refs.searchBox.$refs.input.selectionStart;
        const fontWidth = 7.05; // @TODO: Calculate the absolute cursor position correctly
        this.menuPosX = cursorIndex * fontWidth + this.$refs.searchBox.$el.getBoundingClientRect().left;
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
    this.menuPosY = this.$refs.searchBox.$el.getBoundingClientRect().bottom;
  },
  beforeDestroy() {
    document.body.removeEventListener("keydown", this._keyListener);
  },
  methods: {
    onTab() {
      if (this.menuOpen) {
        this.applyAutocomplete();
      } else {
        this.startAutocompleteSearch();
      }
    },
    onInput(updatedText) {
      this.setSearchBox(updatedText);
      this.startAutocompleteSearch();
    },
    onEnter() {
      if (this.menuOpen) {
        this.applyAutocomplete();
      } else {
        this.search(null);
      }
    },
    applyAutocomplete(index = null) {
      let replace = this.autocompleteItems[index ?? this.selectedAutocompleteIndex];
      if (null == replace) {
        return;
      }
      replace = this.$options.filters.tagNameForURI(replace);
      this.searchBox = this.searchBox.substring(0, this.suggestionStart) + replace + this.searchBox.substring(this.suggestionEnd);
      this.menuOpen = false;
    },
    startAutocompleteSearch() {
      const val = this.searchBox;
      this.typingDelay = setTimeout(() => {
        const cursorPosition = this.$refs.searchBox.$refs.input.selectionStart;
        const suggestionResult = suggest(val, cursorPosition, this.groupedTags);
        this.autocompleteItems = suggestionResult.suggestions;
        this.suggestionStart = suggestionResult.start;
        this.suggestionEnd = suggestionResult.end;
      }, 200);
    },
    abortAutocompleteSearch() {
      if (this.typingDelay) {
        clearTimeout(this.typingDelay);
        this.autocompleteItems = [];
        this.typingDelay = null;
      }
    },
    arrowUp() {
      if (this.menuOpen) {
        this.menuUp();
      } else {
        this.historyUp();
      }
    },
    arrowDown() {
      if (this.menuOpen) {
        this.menuDown();
      } else {
        this.historyDown();
      }
    },
    menuDown() {
      this.selectAutocompleteIndex(this.selectedAutocompleteIndex + 1);
    },
    menuUp() {
      this.selectAutocompleteIndex(this.selectedAutocompleteIndex - 1);
    },
    selectAutocompleteIndex(index) {
      this.selectedAutocompleteIndex = Math.min(Math.max(index, 0), this.autocompleteItems.length - 1);
    },
    historyUp() {
      if (this.historyIndex === -1) {
        this.pendingSearch = this.searchBox;
      }
      this.historyIndex++;
      let term = getTermAt(this.historyIndex);
      if (this.pendingSearch === term) {
        this.historyIndex++;
        term = getTermAt(this.historyIndex);
      }
      this.setSearchBox(term);
    },
    historyDown() {
      if (this.historyIndex === -1) {
        return;
      }
      this.historyIndex--;
      this.setSearchBox(this.historyIndex === -1 ? this.pendingSearch : getTermAt(this.historyIndex));
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
      this.abortAutocompleteSearch();
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
