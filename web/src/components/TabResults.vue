<template>
  <div>
    <template v-if="searchRunning">
      <v-progress-linear indeterminate></v-progress-linear>
    </template>
    <template v-else-if="searchResponse != null">
      <v-alert color="red" type="error" v-if="searchResponse.Error != null">{{
        searchResponse.Error
      }}</v-alert>
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
            hint="Save query as tag(default) or service"
            dense
            @keyup.enter="addTag({ name: 'tag/' + newTagName, query: searchQuery })"
            ><template #append>
              <v-btn
                type="submit"
                value="Save"
                icon
                :loading="tagAddStatus != null && tagAddStatus.inProgress"
                @click="
                  addTag({ name: 'tag/' + newTagName, query: searchQuery })
                "
              >
                <v-icon>mdi-tag</v-icon>
              </v-btn>
              <v-btn
                type="button"
                value="Save as Service"
                icon
                :loading="tagAddStatus != null && tagAddStatus.inProgress"
                @click="
                  addTag({ name: 'service/' + newTagName, query: searchQuery })
                "
              >
                <v-icon>mdi-anchor</v-icon>
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
  </div>
</template>

<script>
import { mapGetters, mapActions, mapState } from "vuex";

export default {
  data() {
    return {
      newTagName: "",
    };
  },
  computed: {
    ...mapGetters([
      "searchResponse",
      "searchRunning",
      "searchPage",
      "nextSearchPage",
    ]),
    ...mapState(["searchQuery", "tagAddStatus"]),
  },
  methods: {
    ...mapActions(["switchSearchPage", "getStream", "addTag"]),
  },
  watch: {
    tagAddStatus(val) {
      if (val.inProgress) return;
      if (val.error != null) {
        alert(val.error.response.data);
        return;
      }
      this.newTagName = "";
      this.$emit("showTagTab");
    },
  },
};
</script>