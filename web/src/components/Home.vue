<template>
  <div>
    <SearchBar defaultQuery="time:-1h:" v-on:search-submitted="searchStreams" />
    <v-container grid-list-md fluid class="grey lighten-4">
      <v-tabs slot="extension" v-model="tabs" left>
        <v-tab :key="0" @click="updateStatus()">
          <v-icon>mdi-information</v-icon> STATUS
        </v-tab>
        <v-tab :key="1" @click="updateTags()">
          <v-icon>mdi-tag-multiple</v-icon> TAGS
        </v-tab>
        <v-tab :key="2"> <v-icon>mdi-chart-areaspline</v-icon> GRAPH </v-tab>
        <v-tab :key="3" v-if="searchResponse != null || searchRunning">
          <template v-if="searchRunning"> SEARCHING </template>
          <template v-else-if="searchResponse.Error == null">
            {{ searchResponse.Results.length
            }}{{ searchResponse.MoreResults ? "+" : "" }} RESULT<template
              v-if="searchResponse.Results.length != 1"
              >S</template
            >
          </template>
          <template v-else> <v-icon>mdi-alert</v-icon> ERROR </template>
        </v-tab>
        <template v-if="streamLoading || streamData != null">
          <v-tab :key="4">
            STREAM {{ streamLoading ? "..." : streamData.Stream.ID }}
          </v-tab>
          <template v-if="tabs == 4">
            <v-spacer />
            <v-tab
              @click="getStream(prevStreamIndex)"
              :disabled="prevStreamIndex == null"
              :key="5"
              ><v-icon>mdi-chevron-left</v-icon></v-tab
            >
            <v-tab
              @click="getStream(nextStreamIndex)"
              :disabled="nextStreamIndex == null"
              :key="6"
              ><v-icon>mdi-chevron-right</v-icon></v-tab
            >
          </template>
        </template>
      </v-tabs>
      <v-tabs-items style="width: 100%" v-model="tabs">
        <v-tab-item :key="0">
          <v-card>
            <v-simple-table>
              <tbody>
                <tr v-for="(value, name) in status" :key="name">
                  <th>{{ name }}</th>
                  <td width="100%">{{ value }}</td>
                </tr>
              </tbody>
            </v-simple-table>
          </v-card>
          <br />
          <HelpPage />
        </v-tab-item>
        <v-tab-item :key="1">
          <TabTags v-on:searchStreams="searchStreams" />
        </v-tab-item>
        <v-tab-item :key="2">
          <TabGraph />
        </v-tab-item>
        <v-tab-item :key="3">
          <TabResults v-on:showTagTab="tabs = 1" />
        </v-tab-item>
        <v-tab-item :key="4">
          <TabStream />
        </v-tab-item>
      </v-tabs-items>
    </v-container>
  </div>
</template>

<script>
import SearchBar from "./SearchBar.vue";
import HelpPage from "./HelpPage.vue";
import TabTags from "./TabTags.vue";
import TabGraph from "./TabGraph.vue";
import TabResults from "./TabResults.vue";
import TabStream from "./TabStream.vue";

import { mapGetters, mapActions, mapState } from "vuex";

export default {
  name: "Home",
  components: { SearchBar, TabTags, HelpPage, TabGraph, TabResults, TabStream },
  data() {
    return {
      tabs: 0,
    };
  },
  created() {
    this.updateStatus();
    this.updateTags();
  },
  computed: {
    ...mapGetters(["prevStreamIndex", "nextStreamIndex"]),
    ...mapState([
      "streamData",
      "status",
      "streamLoading",
      "searchRunning",
      "searchResponse",
    ]),
  },
  methods: {
    ...mapActions(["updateStatus", "updateTags", "searchStreams", "getStream"]),
  },
  watch: {
    searchRunning() {
      this.tabs = 3;
    },
    streamLoading() {
      this.$vuetify.goTo(0, {});
      this.tabs = 4;
    },
  },
};
</script>

<style>
.v-tabs__content {
  padding-bottom: 2px;
}
.streams-table tbody tr :hover {
  cursor: pointer;
}
</style>