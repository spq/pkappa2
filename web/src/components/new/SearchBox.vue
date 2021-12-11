<template>
  <v-text-field
    autofocus
    hide-details
    flat
    prepend-inner-icon="mdi-magnify"
    v-model="searchBox"
    @keyup.enter="search(null)"
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
</template>

<script>
import { EventBus } from "./EventBus";

export default {
  name: "SearchBox",
  data() {
    return {
      searchBox: this.$route.query.q,
    };
  },
  created() {
    EventBus.$on("setSearchTerm", this.setSearchTerm);
  },
  watch: {
    "$route.query.q": function (term) {
      this.searchBox = term;
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
    search(type) {
      let q = {};
      if (!type) {
        type = this.$route.name == "graph" ? "graph" : "search";
        if (type == "graph") q = JSON.parse(JSON.stringify(this.$route.query));
      }
      q.q = this.searchBox;
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