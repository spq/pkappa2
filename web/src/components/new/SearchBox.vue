<template>
  <v-combobox
    autofocus
    hide-details
    flat
    no-filter
    hide-no-data
    prepend-inner-icon="mdi-magnify"
    v-model="searchBox"
    @keyup.enter="search(null)"
    @keydown.up.prevent="historyUp"
    @keydown.down.prevent="historyDown"
    ref="searchBox"
    :search-input.sync="autocompleteValue"
    :items="autocompleteItems"
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
  </v-combobox>
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
      autocompleteValue: null,
    };
  },
  created() {
    EventBus.$on("setSearchTerm", this.setSearchTerm);
  },
  computed: {
    ...mapGetters(["groupedTags"]),
  },
  watch: {
    "$route.query.q": function (term) {
      this.searchBox = term;
    },
    autocompleteValue(val) {
      if (this.typingDelay) {
        clearTimeout(this.typingDelay);
        this.typingDelay = null;
      }
      this.typingDelay = setTimeout(() => {
        const cursorPosition = this.$refs.searchBox.$refs.input.selectionStart;
        this.autocompleteItems = suggest(val, cursorPosition, this.groupedTags);
      }, 200);
    },
  },
  mounted() {
    this._keyListener = function (e) {
      if (["input", "textarea"].includes(e.target.tagName.toLowerCase()))
        return;
      if (e.key != "/") return;

      e.preventDefault();
      this.$refs.searchBox.focus();
    };
    document.body.addEventListener("keydown", this._keyListener.bind(this));
  },
  beforeDestroy() {
    document.body.removeEventListener("keydown", this._keyListener);
  },
  methods: {
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
      this.searchBox = term;
    },
    historyDown() {
      if (this.historyIndex === -1) {
        return;
      }
      this.historyIndex--;
      this.searchBox = this.historyIndex === -1 ? this.pendingSearch : getTermAt(this.historyIndex);
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
    setSearchTerm({ searchTerm }) {
      this.searchBox = searchTerm;
    },
    createTag(tagType, tagQuery) {
      EventBus.$emit("showCreateTagDialog", { tagType, tagQuery });
    },
  },
};
</script>