<template>
  <div>
    <ToolBar>
      <v-tooltip bottom>
        <template #activator="{ on, attrs }">
          <v-btn
            v-bind="attrs"
            v-on="on"
            icon
            :to="{
              name: 'search',
              query: { q: $route.query.q, p: $route.query.p },
            }"
          >
            <v-icon>mdi-arrow-left</v-icon>
          </v-btn>
        </template>
        <span>Back to Search Results</span>
      </v-tooltip>
      <v-tooltip bottom>
        <template #activator="{ on, attrs }">
          <v-btn v-bind="attrs" v-on="on" icon @click="fetchStream()">
            <v-icon>mdi-refresh</v-icon>
          </v-btn>
        </template>
        <span>Refresh</span>
      </v-tooltip>
      <v-tooltip bottom>
        <template #activator="{ on, attrs }">
          <v-btn
            link
            exact
            v-bind="attrs"
            v-on="on"
            icon
            :to="{
              name: 'search',
              query: {
                q: selectionQuery
              }
            }"
            ><v-icon>mdi-selection-search</v-icon></v-btn
          >
        </template>
        <span>Search Selection</span>
      </v-tooltip>
      <v-menu offset-y right bottom
        ><template #activator="{ on: onMenu, attrs }">
          <v-tooltip bottom>
            <template #activator="{ on: onTooltip }">
              <v-btn v-bind="attrs" v-on="{ ...onMenu, ...onTooltip }" icon>
                <v-icon>mdi-checkbox-multiple-outline</v-icon>
              </v-btn>
            </template>
            <span>Marks</span>
          </v-tooltip>
        </template>
        <v-list dense v-if="stream.stream != null">
          <template v-for="tag of groupedTags.mark">
            <v-list-item
              :key="tag.Name"
              link
              @click="
                markStream(tag.Name, !stream.stream.Tags.includes(tag.Name))
              "
            >
              <v-list-item-action>
                <v-icon
                  >mdi-{{
                    stream.stream.Tags.includes(tag.Name)
                      ? "checkbox-outline"
                      : "checkbox-blank-outline"
                  }}</v-icon
                >
              </v-list-item-action>
              <v-list-item-content>
                <v-list-item-title>{{
                  tag.Name | tagify("name")
                }}</v-list-item-title>
              </v-list-item-content>
            </v-list-item>
          </template>
          <v-divider />
          <v-list-item link @click="createMark">
            <v-list-item-action />
            <v-list-item-title>Create new</v-list-item-title>
          </v-list-item>
        </v-list>
      </v-menu>
      <v-tooltip bottom>
        <template #activator="{ on, attrs }">
          <v-btn
            v-bind="attrs"
            v-on="on"
            icon
            :href="`/api/download/${streamId}.pcap`"
            ><v-icon>mdi-download</v-icon></v-btn
          >
        </template>
        <span>Download PCAP</span>
      </v-tooltip>
      <v-btn-toggle mandatory dense borderless v-model="presentation">
        <v-tooltip bottom>
          <template #activator="{ on, attrs }">
            <v-btn v-bind="attrs" v-on="on" value="ascii">
              <v-icon>mdi-text-long</v-icon>
            </v-btn>
          </template>
          <span>ASCII</span>
        </v-tooltip>
        <v-tooltip bottom>
          <template #activator="{ on, attrs }">
            <v-btn v-bind="attrs" v-on="on" value="hexdump">
              <v-icon>mdi-format-columns</v-icon>
            </v-btn>
          </template>
          <span>HEXDUMP</span>
        </v-tooltip>
        <v-tooltip bottom>
          <template #activator="{ on, attrs }">
            <v-btn v-bind="attrs" v-on="on" value="raw">
              <v-icon>mdi-hexadecimal</v-icon>
            </v-btn>
          </template>
          <span>RAW</span>
        </v-tooltip>
      </v-btn-toggle>
      <v-spacer />
      <div v-if="streamIndex != null">
        <span class="text-caption"
          >{{ streams.result.Offset + streamIndex + 1 }} of
          {{
            streams.result.MoreResults
              ? "many"
              : streams.result.Results.length + streams.result.Offset
          }}</span
        >
        <v-tooltip bottom>
          <template #activator="{ on }">
            <span v-on="on">
              <v-btn
                icon
                ref="prevStream"
                :disabled="prevStreamId == null"
                :to="
                  prevStreamId == null
                    ? null
                    : {
                        name: 'stream',
                        query: { q: $route.query.q, p: $route.query.p },
                        params: { streamId: prevStreamId },
                      }
                "
              >
                <v-icon>mdi-chevron-left</v-icon>
              </v-btn>
            </span>
          </template>
          <span>Previous Stream</span>
        </v-tooltip>
        <v-tooltip bottom>
          <template #activator="{ on }">
            <span v-on="on">
              <v-btn
                icon
                ref="nextStream"
                :disabled="nextStreamId == null"
                :to="
                  nextStreamId == null
                    ? null
                    : {
                        name: 'stream',
                        query: { q: $route.query.q, p: $route.query.p },
                        params: { streamId: nextStreamId },
                      }
                "
              >
                <v-icon>mdi-chevron-right</v-icon>
              </v-btn>
            </span>
          </template>
          <span>Next Stream</span>
        </v-tooltip>
      </div>
    </ToolBar>
    <v-skeleton-loader
      type="table-thead, table-tbody"
      v-if="stream.running || !(stream.stream || stream.error)"
    ></v-skeleton-loader>
    <v-alert type="error" dense v-else-if="stream.error">{{
      stream.error
    }}</v-alert>
    <div v-else>
      <v-container fluid>
        <v-row>
          <v-col cols="1" class="text-subtitle-2">Client:</v-col>
          <v-col cols="2" class="text-body-2"
            >{{ stream.stream.Stream.Client.Host }}:{{
              stream.stream.Stream.Client.Port
            }}
            ({{ stream.stream.Stream.Client.Bytes }} Bytes)</v-col
          >
          <v-col cols="1" class="text-subtitle-2">First Packet:</v-col>
          <v-col
            cols="3"
            class="text-body-2"
            :title="stream.stream.Stream.FirstPacket | formatDateLong"
            >{{ stream.stream.Stream.FirstPacket | formatDate }}</v-col
          >
          <v-col cols="1" class="text-subtitle-2"
            >{{ tags.service.length == 0 ? "Protocol" : "Service" }}:</v-col
          >
          <v-col cols="1" class="text-subtitle-2">Tags:</v-col>
          <v-col cols="3" class="text-body-2"
            ><v-chip small v-for="name in tags.tag" :key="`tag/${name}`">{{
              name
            }}</v-chip></v-col
          >
        </v-row>
        <v-row>
          <v-col cols="1" class="text-subtitle-2">Server:</v-col>
          <v-col cols="2" class="text-body-2"
            >{{ stream.stream.Stream.Server.Host }}:{{
              stream.stream.Stream.Server.Port
            }}
            ({{ stream.stream.Stream.Server.Bytes }} Bytes)</v-col
          >
          <v-col cols="1" class="text-subtitle-2">Last Packet:</v-col>
          <v-col
            cols="3"
            class="text-body-2"
            :title="stream.stream.Stream.LastPacket | formatDateLong"
            >{{ stream.stream.Stream.LastPacket | formatDate }}</v-col
          >
          <v-col cols="1" class="text-body-2"
            ><span v-if="tags.service.length == 0">{{
              stream.stream.Stream.Protocol
            }}</span
            ><span v-else
              ><v-chip
                small
                v-for="name in tags.service"
                :key="`service/${name}`"
                >{{ name }}</v-chip
              >
              ({{ stream.stream.Stream.Protocol }})</span
            ></v-col
          >
          <v-col cols="1" class="text-subtitle-2">Marks:</v-col>
          <v-col cols="3" class="text-body-2"
            ><v-chip small v-for="name in tags.mark" :key="`mark/${name}`">{{
              name
            }}</v-chip></v-col
          >
        </v-row>
      </v-container>
      <StreamData
        :data="stream.stream.Data"
        :presentation="presentation"
        ref="streamData"
      ></StreamData>
    </div>
  </div>
</template>

<script>
import { EventBus } from "./EventBus";
import StreamData from "./StreamData.vue";
import {registerSelectionListener, destroySelectionListener} from './streamSelector';

import { mapActions, mapGetters, mapState } from "vuex";
import ToolBar from "./ToolBar.vue";

export default {
  name: "Stream",
  components: { StreamData, ToolBar },
  data() {
    let p = "ascii";
    if (localStorage.streamPresentation) {
      p = localStorage.streamPresentation;
    }
    return {
      presentation: p,
      selectionQuery: '',
    };
  },
  computed: {
    ...mapState(["stream", "streams"]),
    ...mapGetters(["groupedTags"]),
    tags() {
      let res = {
        service: [],
        tag: [],
        mark: [],
      };
      for (const tag of this.stream.stream.Tags) {
        const type = tag.split("/", 1)[0];
        const name = tag.substr(type.length + 1);
        res[type].push(name);
      }
      return res;
    },
    streamId() {
      return parseInt(this.$route.params.streamId, 10);
    },
    streamIndex() {
      if (this.streams.result == null) return null;
      const id = this.streamId;
      let i = 0;
      for (let r of this.streams.result.Results) {
        if (r.Stream.ID == id) return i;
        i++;
      }
      return null;
    },
    nextStreamId() {
      const index = this.streamIndex;
      if (index == null) return null;
      if (index + 1 >= this.streams.result.Results.length) return null;
      return this.streams.result.Results[index + 1].Stream.ID;
    },
    prevStreamId() {
      const index = this.streamIndex;
      if (index == null || index == 0) return null;
      return this.streams.result.Results[index - 1].Stream.ID;
    },
  },
  mounted() {
    this.fetchStream();
    registerSelectionListener(this);

    const handle = (e, streamId) => {
      if (streamId == null) return;
      e.preventDefault();
      this.$router.push({
        name: "stream",
        query: { q: this.$route.query.q, p: this.$route.query.p },
        params: { streamId },
      });
    };
    const handlers = {
      j: (e) => {
        handle(e, this.prevStreamId);
      },
      k: (e) => {
        handle(e, this.nextStreamId);
      },
    };
    this._keyListener = function (e) {
      if (["input", "textarea"].includes(e.target.tagName.toLowerCase()))
        return;

      if (!Object.keys(handlers).includes(e.key)) return;
      handlers[e.key](e);
    }.bind(this);
    window.addEventListener("keydown", this._keyListener);
  },
  beforeDestroy() {
    window.removeEventListener("keydown", this._keyListener);
    destroySelectionListener();
  },
  methods: {
    ...mapActions(["fetchStreamNew", "markTagAdd", "markTagDel", "searchStreamsNew"]),
    fetchStream() {
      if (this.streamId != null) this.fetchStreamNew({ id: this.streamId });
    },
    createMark() {
      EventBus.$emit("showCreateTagDialog", {
        tagType: "mark",
        tagStreams: [this.streamId],
      });
    },
    markStream(tagId, value) {
      if (value)
        this.markTagAdd({ name: tagId, streams: [this.streamId] }).catch(
          (err) => {
            EventBus.$emit("showError", { message: err });
          }
        );
      else
        this.markTagDel({ name: tagId, streams: [this.streamId] }).catch(
          (err) => {
            EventBus.$emit("showError", { message: err });
          }
        );
    },
  },
  watch: {
    $route: "fetchStream",
    presentation(v) {
      localStorage.streamPresentation = v;
    },
  },
};
</script>