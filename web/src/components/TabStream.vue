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
            Download PCAP
          </v-btn>
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
import { mapGetters } from "vuex";

export default {
  data() {
    return {
      dataTab: 0,
    };
  },
  computed: {
    ...mapGetters(["streamData", "streamLoading"]),
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