<template>
  <div>
    <v-progress-linear
      indeterminate
      v-if="streamData == null && streamLoading"
    ></v-progress-linear>
    <template v-if="streamData != null">
      <v-card>
        <v-container fluid>
          <v-row>
            <v-col>
              <v-card-subtitle>Client</v-card-subtitle>
              <v-card-text>
                <v-row
                  >{{ streamData.Stream.Client.Host }}:{{
                    streamData.Stream.Client.Port
                  }}
                  ({{ streamData.Stream.Client.Bytes }} Bytes)</v-row
                >
              </v-card-text>
            </v-col>
            <v-col>
              <v-card-subtitle>First packet</v-card-subtitle>

              <v-card-text>
                <v-row>{{ streamData.Stream.FirstPacket }}</v-row>
              </v-card-text>
            </v-col>
            <v-col>
              <v-card-subtitle>Last Packet</v-card-subtitle>

              <v-card-text>
                <v-row>{{ streamData.Stream.LastPacket }}</v-row>
              </v-card-text>
            </v-col>
            <v-col>
              <v-card-subtitle>Protocol</v-card-subtitle>

              <v-card-text>
                <v-row>{{ streamData.Stream.Protocol }}</v-row>
              </v-card-text>
            </v-col>
            <v-col>
              <v-card-subtitle>Server</v-card-subtitle>

              <v-card-text>
                <v-row
                  >{{ streamData.Stream.Server.Host }}:{{
                    streamData.Stream.Server.Port
                  }}
                  ({{ streamData.Stream.Server.Bytes }} Bytes)</v-row
                >
              </v-card-text>
            </v-col>
          </v-row>
        </v-container>

        <v-card-actions>
          <v-btn
            text
            :href="'/api/download/' + streamData.Stream.ID + '.pcap'"
            target="_blank"
          >
            <v-icon dense>mdi-download</v-icon>Download PCAP
          </v-btn>
          <v-menu offset-y :close-on-content-click="false">
            <template v-slot:activator="{ on, attrs }">
              <v-btn
                text
                v-bind="attrs"
                v-on="on"
                :loading="
                  markTagUpdateStatus != null && markTagUpdateStatus.inProgress
                "
              >
                <v-icon dense>mdi-bookmark</v-icon>Mark
              </v-btn>
            </template>
            <v-list>
              <v-list-item
                ><v-text-field
                  label="Add"
                  dense
                  v-model="newMarkName"
                  @keyup.enter="newMark"
                  ><template #append>
                    <v-btn
                      type="submit"
                      value="Add"
                      icon
                      :loading="
                        markTagUpdateStatus != null &&
                        markTagUpdateStatus.inProgress
                      "
                      @click="newMark"
                      ><v-icon>mdi-content-save</v-icon></v-btn
                    ></template
                  ></v-text-field
                ></v-list-item
              >
              <v-list-item-group multiple v-model="marks">
                <template v-for="tag in tags">
                  <v-list-item
                    v-if="tag.Name.startsWith('mark/')"
                    :key="tag.Name"
                    :value="tag.Name"
                    v-slot:default="{ active }"
                  >
                    <v-list-item-action>
                      <v-checkbox :input-value="active"></v-checkbox>
                    </v-list-item-action>
                    <v-list-item-content>
                      <v-list-item-title>{{
                        tag.Name.substring(5)
                      }}</v-list-item-title>
                    </v-list-item-content>
                  </v-list-item>
                </template>
              </v-list-item-group>
            </v-list>
          </v-menu>
        </v-card-actions>
      </v-card>
      <v-container grid-list-md fluid class="grey lighten-4">
        <v-tabs slot="extension" v-model="dataTab" left>
          <v-tab :key="0"> ASCII </v-tab>
          <v-tab :key="1"> HEXDUMP </v-tab>
          <v-tab :key="2"> RAW </v-tab>
        </v-tabs>
        <v-tabs-items style="width: 100%" v-model="dataTab">
          <v-tab-item :key="0"
            ><v-card
              ><v-card-text
                ><span
                  v-for="(chunk, index) in streamData.Data"
                  :key="index"
                  :style="
                    chunk.Direction != 0
                      ? 'font-family: monospace,monospace; color: #000080; background-color: #eeedfc;'
                      : 'font-family: monospace,monospace; color: #800000; background-color: #faeeed;'
                  "
                  v-html="
                    $options.filters.inlineAscii(chunk.Content)
                  " /></v-card-text></v-card
          ></v-tab-item>
          <v-tab-item :key="1"
            ><v-card
              ><v-card-text>
                <pre
                  v-for="(chunk, index) in streamData.Data"
                  :key="index"
                  :style="
                    chunk.Direction != 0
                      ? 'margin-left: 2em; color: #000080; background-color: #eeedfc;'
                      : 'color: #800000; background-color: #faeeed;'
                  "
                  >{{ chunk.Content | hexdump }}</pre
                >
              </v-card-text></v-card
            ></v-tab-item
          >
          <v-tab-item :key="2"
            ><v-card
              ><v-card-text
                ><span
                  v-for="(chunk, index) in streamData.Data"
                  :key="index"
                  :style="
                    chunk.Direction != 0
                      ? 'font-family: monospace,monospace; color: #000080; background-color: #eeedfc;'
                      : 'font-family: monospace,monospace; color: #800000; background-color: #faeeed;'
                  "
                >
                  {{ chunk.Content | inlineHex
                  }}<br /></span></v-card-text></v-card
          ></v-tab-item>
        </v-tabs-items>
      </v-container>
    </template>
  </div>
</template>

<script>
import { mapGetters, mapState, mapActions } from "vuex";

export default {
  data() {
    return {
      dataTab: 0,
      newMarkName: "",
    };
  },
  computed: {
    ...mapState(["tags", "markTagUpdateStatus"]),
    ...mapGetters(["streamData", "streamLoading"]),
    marks: {
      get() {
        return this.streamData.Tags;
      },
      set(val) {
        const old = this.streamData.Tags;
        const added = val.filter((x) => !old.includes(x));
        for (const tag of added) {
          this.markTagAdd({
            name: tag,
            streams: [this.streamData.Stream.ID],
          });
        }

        const removed = old.filter((x) => !val.includes(x));
        for (const tag of removed) {
          let found = false;
          for (const t of this.tags) {
            if (t.Name == tag) {
              found = true;
              break;
            }
          }
          if (!found) continue;
          this.markTagDel({
            name: tag,
            streams: [this.streamData.Stream.ID],
          });
        }

        this.streamData.Tags = val;
      },
    },
  },
  methods: {
    ...mapActions(["markTagNew", "markTagAdd", "markTagDel", "updateTags"]),
    newMark: function () {
      this.markTagNew({
        name: this.newMarkName,
        streams: [this.streamData.Stream.ID],
      });
      this.updateTags();
      this.streamData.Tags.push("mark/" + this.newMarkName);
      this.newMarkName = "";
    },
  },
  filters: {
    inlineAscii(b64) {
      const ui8 = Uint8Array.from(
        atob(b64)
          .split("")
          .map((char) => char.charCodeAt(0))
      );
      var str = [].slice
        .call(ui8)
        .map(function (i, idx, arr) {
          if (i == 0x0d && idx + 1 < arr.length && arr[idx + 1] == 0x0a)
            return "";
          if (i == 0x0a) return "<br/>";
          if (/[ -~]/.test(String.fromCharCode(i))) {
            return "&#x" + ("00" + i.toString(16)).substr(-2) + ";";
          }
          return ".";
        })
        .join("");
      return str;
    },
    inlineHex(b64) {
      const ui8 = Uint8Array.from(
        atob(b64)
          .split("")
          .map((char) => char.charCodeAt(0))
      );
      var str = [].slice
        .call(ui8)
        .map(function (i) {
          var h = i.toString(16);
          if (h.length < 2) {
            h = "0" + h;
          }
          return h;
        })
        .join("");
      return str;
    },
    hexdump(b64) {
      const ui8 = Uint8Array.from(
        atob(b64)
          .split("")
          .map((char) => char.charCodeAt(0))
      );
      var str = [].slice
        .call(ui8)
        .map(function (i) {
          var h = i.toString(16);
          if (h.length < 2) {
            h = "0" + h;
          }
          return h;
        })
        .join("")
        .match(/.{1,2}/g)
        .join(" ")
        .match(/.{1,48}/g)
        .map(function (str) {
          while (str.length < 48) {
            str += " ";
          }
          var ascii = str
            .replace(/ /g, "")
            .match(/.{1,2}/g)
            .map(function (ch) {
              var c = String.fromCharCode(parseInt(ch, 16));
              if (!/[ -~]/.test(c)) {
                c = ".";
              }
              return c;
            })
            .join("");
          while (ascii.length < 16) {
            ascii += " ";
          }
          return str + " |" + ascii + "|";
        })
        .join("\n");
      return str;
    },
  },
};
</script>