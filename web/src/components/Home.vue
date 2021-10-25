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
          <TabGraph/>
        </v-tab-item>
        <v-tab-item :key="3">
          <template v-if="searchRunning">
            <v-progress-linear indeterminate></v-progress-linear>
          </template>
          <template v-else-if="searchResponse != null">
            <v-alert
              color="red"
              type="error"
              v-if="searchResponse.Error != null"
              >{{ searchResponse.Error }}</v-alert
            >
            <v-simple-table class="streams-table" dense v-else>
              <template v-slot:default>
                <thead>
                  <tr>
                    <th class="text-left">ID</th>
                    <th class="text-left">Time</th>
                    <th class="text-left">Client</th>
                    <th class="text-left">Bytes</th>
                    <th class="text-left">Server</th>
                    <th class="text-left">Bytes</th>
                  </tr>
                </thead>
                <tbody>
                  <tr
                    v-for="(stream, index) in searchResponse.Results"
                    :key="stream.ID"
                    @click="getStream(index)"
                  >
                    <td>{{ stream.ID }}</td>
                    <td>{{ stream.FirstPacket }}</td>
                    <td>{{ stream.Client.Host }}:{{ stream.Client.Port }}</td>
                    <td>{{ stream.Client.Bytes }}</td>
                    <td>{{ stream.Server.Host }}:{{ stream.Server.Port }}</td>
                    <td>{{ stream.Server.Bytes }}</td>
                  </tr>
                </tbody>
              </template>
            </v-simple-table>
            <v-card class="mr-auto d-flex" tile>
              <div class="mr-auto">
                <v-text-field
                  v-model="newTagName"
                  hint="Save query as tag"
                  prepend-inner-icon="mdi-tag"
                  dense
                  @keyup.enter="
                    addTag({ name: newTagName, query: searchQuery })
                  "
                  ><template #append>
                    <v-btn
                      type="submit"
                      value="Save"
                      icon
                      :loading="tagAddStatus != null && tagAddStatus.inProgress"
                      @click="addTag({ name: newTagName, query: searchQuery })"
                    >
                      <v-icon>mdi-content-save</v-icon>
                    </v-btn>
                  </template></v-text-field
                >
              </div>
              <div>
                <v-pagination
                  :value="searchPage + 1"
                  :length="searchPage + (nextSearchPage != null ? 2 : 1)"
                  @input="switchSearchPage"
                ></v-pagination>
              </div>
            </v-card>
          </template>
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
import TabStream from "./TabStream.vue";
import TabGraph from "./TabGraph.vue";
import { mapMutations, mapGetters, mapActions, mapState } from "vuex";

export default {
  name: "Home",
  components: { SearchBar, TabTags, HelpPage, TabStream, TabGraph },
  data() {
    return {
      tabs: 0,
      newTagName: "",
    };
  },
  computed: {
    ...mapGetters([
      "searchResponse",
      "searchRunning",
      "streamData",
      "status",
      "prevStreamIndex",
      "nextStreamIndex",
      "streamLoading",
      "searchPage",
      "nextSearchPage",
    ]),
    ...mapState(["searchQuery", "tagAddStatus"]),
  },
  created() {
    this.updateStatus();
    this.updateTags();
  },
  methods: {
    ...mapMutations([]),
    ...mapActions([
      "searchStreams",
      "switchSearchPage",
      "getStream",
      "updateStatus",
      "updateTags",
      "addTag",
    ]),
  },
  watch: {
    searchRunning() {
      this.tabs = 3;
    },
    streamLoading() {
      this.$vuetify.goTo(0, {});
      this.tabs = 4;
    },
    tagAddStatus(val) {
      if (val.inProgress) return;
      if (val.error != null) {
        alert(val.error.response.data);
        return;
      }
      this.tabs = 1;
      this.newTagName = "";
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