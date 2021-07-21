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
        <v-tab :key="2" v-if="searchResponse != null || searchRunning">
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
          <v-tab :key="3">
            STREAM {{ streamLoading ? "..." : streamData.Stream.ID }}
          </v-tab>
          <template v-if="tabs == 3">
            <v-spacer />
            <v-tab
              @click="getStream(prevStreamIndex)"
              :disabled="prevStreamIndex == null"
              :key="4"
              ><v-icon>mdi-chevron-left</v-icon></v-tab
            >
            <v-tab
              @click="getStream(nextStreamIndex)"
              :disabled="nextStreamIndex == null"
              :key="5"
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
          <v-card>
            <v-card-title>Query format</v-card-title>
            <v-simple-table>
              <tbody>
                <tr>
                  <th>Operators</th>
                  <td><code>filter&nbsp;[AND]|OR|THEN&nbsp;filter</code></td>
                  <td width="100%">
                    <code>AND</code>/<code>OR</code> do what you expect.
                    <code>THEN</code> works like <code>AND</code> but makes
                    <code>[cs]data</code> filters match sequentially.
                    <code>AND</code> can be omitted.
                  </td>
                </tr>
                <tr>
                  <th>Brackets</th>
                  <td><code>(filter)</code></td>
                  <td width="100%">Filters can be grouped in brackets.</td>
                </tr>
                <tr>
                  <th>Negation</th>
                  <td><code>-filter</code></td>
                  <td width="100%">Inverts the filter.</td>
                </tr>
                <tr>
                  <th>Filter&nbsp;format</th>
                  <td>
                    <code>key:value</code>&nbsp;or&nbsp;<code>key:"value"</code>
                  </td>
                  <td width="100%">
                    If no special chars(e.g. space, quotes, brackets) are
                    required, format 1 can be used, otherwise use format 2,
                    where <code>"</code> can be escaped by repeating it.
                  </td>
                </tr>
                <tr>
                  <th>Sub-queries</th>
                  <td><code>@name:id:123</code></td>
                  <td width="100%">
                    Sub-queries are supported by prefixing any filter with
                    <code>@subquery-name:</code>.
                  </td>
                </tr>
                <tr>
                  <th>Variables</th>
                  <td><code>@id@</code> or <code>@subquery:ftime@</code></td>
                  <td width="100%">
                    Variables can be used within most filters. If subqueries are
                    used, referencing a variable from a different subquery is
                    done by prefixing the variablename with the subquery name
                    and a <code>:</code>.
                  </td>
                </tr>
                <tr>
                  <th>Tag&nbsp;filter</th>
                  <td><code>tag:tagname,othertag</code></td>
                  <td width="100%">
                    Restricts the results to streams that were identified as
                    matching to the query of one of the named tags separated by
                    <code>,</code>.
                  </td>
                </tr>
                <tr>
                  <th>Protocol&nbsp;filter</th>
                  <td><code>protocol:tcp,udp</code></td>
                  <td width="100%">
                    Restricts the results to streams of the given protocols,
                    supported protocols are <code>tcp</code>,
                    <code>udp</code> and <code>sctp</code>, separate the
                    protocols by <code>,</code>. This filter supports the
                    <code>protocol</code> variable, e.g.
                    <code>protocol:@subquery:protocol@</code>.
                  </td>
                </tr>
                <tr>
                  <th>Id&nbsp;filter</th>
                  <td><code>id:1,2,3,@subquery:id@+123</code></td>
                  <td width="100%">
                    Restricts the results to only streams with one of the given
                    ids. You can give a list of (separate by <code>,</code>) ids
                    or id ranges (using <code>:</code>), id ranges can be
                    open(by leaving out the number) at any side. Any of these
                    variables, optionally from subqueries, can be used:
                    <code>id</code>, <code>[cs]port</code>,
                    <code>[cs]bytes</code>. Simple calculations can be
                    performed, using the operators <code>+</code> and
                    <code>-</code>.
                  </td>
                </tr>
                <tr>
                  <th>Port&nbsp;filter</th>
                  <td><code>[cs]port:80,1024:,</code></td>
                  <td width="100%">
                    <code>cport</code>, <code>sport</code> and
                    <code>port</code> filter on the client, server or any port.
                    The syntax is identical to the <code>id</code> filter
                    syntax.
                  </td>
                </tr>
                <tr>
                  <th>Bytes&nbsp;filter</th>
                  <td><code>[cs]bytes:1337,2048:4096</code></td>
                  <td width="100%">
                    <code>cbytes</code>, <code>sbytes</code> and
                    <code>bytes</code> filter on the number of bytes send by the
                    client, server or any of them. The syntax is identical to
                    the <code>id</code> filter syntax.
                  </td>
                </tr>
                <tr>
                  <th>Host&nbsp;filter</th>
                  <td>
                    <code>[cs]host:1.2.3.4,10.0.0.0/8,::1,10.0.0.1/8/-8</code>
                  </td>
                  <td width="100%">
                    <code>chost</code>, <code>shost</code> and
                    <code>host</code> filter on the client, server or any host,
                    lists are supported, each entry consists of an ip-address or
                    a variable (e.g. <code>@subquery:[cs]host@</code>).
                    Optionally, one or more <code>/bits</code> suffixes are
                    appended. The suffixes can be negative,
                    <code>/16/-8</code> would make a
                    <code>255.255.0.255</code>/<code>ffff::ff</code> netmask.
                  </td>
                </tr>
                <tr>
                  <th>Time&nbsp;filter</th>
                  <td>
                    <code>[fl]time:-1h:,1300:1400,@subquery:ftime@-5m:</code>
                  </td>
                  <td width="100%">
                    Filters to streams with the first(<code>ftime</code>),
                    last(<code>ltime</code>) or any(<code>time</code>) packet
                    being in the given timeranges. Lists are supported, you can
                    use open ranges where each side of the range is either a
                    relative time from now (e.g. <code>-2h3m4s</code>) or an
                    absolute time using the format
                    <code>[YYYY-MM-DD ]HHMM[SS]</code>.
                    <code>[fl]time</code> variables can be used as well as
                    simple calculations using <code>+</code> and <code>-</code>.
                    For finding streams that lasted 5 minutes or longer you
                    could e.g. use <code>ltime:@ftime@+5m</code>.
                  </td>
                </tr>
                <tr>
                  <th>Data&nbsp;filter</th>
                  <td><code>[cs]data:flag[{}].+[}]</code></td>
                  <td width="100%">
                    Select streams that contain the given regex in the data send
                    by the client(<code>cdata</code>),
                    server(<code>sdata</code>) or any of
                    them(<code>data</code>). The regex format is described here:
                    <a
                      href="https://golang.org/pkg/regexp/syntax/#hdr-Syntax"
                      target="_blank"
                      >Golang regexp syntax</a
                    >. Within one set of <code>then</code>-connected data
                    filters, you can use variables referencing named capture
                    groups from previous data filters of the same set. Example:
                    <code
                      >cdata:"(?P&lt;flag&gt;FLAG:[0-9a-f]{16})" then
                      cdata:"@flag@"</code
                    >. One set of <code>then</code>-connected
                    <code>data</code> filters must belong to the same sub-query.
                  </td>
                </tr>
                <tr>
                  <th>Sorting</th>
                  <td><code>sort:saddr,ftime,-id</code></td>
                  <td width="100%">
                    Results can be sorted by using the
                    <code>sort</code> "filter". It may only appear once in the
                    query, the value is a list of <code>,</code> separated terms
                    with an optional <code>-</code> prefix inverting the sort
                    order of that term. Available terms are: <code>id</code>,
                    <code>[fl]time</code>, <code>[cs]bytes</code>,
                    <code>[cs]host</code> and <code>[cs]port</code>. The default
                    is <code>-ftime</code>.
                  </td>
                </tr>
                <tr>
                  <th>Limiting&nbsp;result&nbsp;count</th>
                  <td><code>limit:10</code></td>
                  <td width="100%">
                    <code>limit</code> is used to restrict the number of
                    results, it only accepts a number as value, the default is
                    <code>100</code>, the value <code>0</code> means unlimited.
                  </td>
                </tr>
              </tbody>
            </v-simple-table>
          </v-card>
        </v-tab-item>
        <v-tab-item :key="1">
          <v-simple-table dense>
            <thead>
              <tr>
                <th class="text-left">Name</th>
                <th class="text-left">Query</th>
                <th colspan="2" class="text-left">Status</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="tag in tags" :key="tag.Name">
                <td>{{ tag.Name }}</td>
                <td>{{ tag.Definition }}</td>
                <td>
                  Matching {{ tag.MatchingCount }} Streams<span
                    v-if="tag.IndexesPending != 0"
                    >, {{ tag.IndexesPending }} Indexes pending</span
                  ><span v-if="tag.Referenced"
                    >, Referenced by another tag</span
                  >
                </td>
                <td align="right">
                  <v-btn icon @click="searchStreams('tag:' + tag.Name)"
                    ><v-icon>mdi-magnify</v-icon></v-btn
                  >
                  <v-btn
                    :disabled="tag.Referenced"
                    icon
                    @click="delTag(tag.Name)"
                    :loading="tagDelStatus != null && tagDelStatus.inProgress"
                    ><v-icon>mdi-delete</v-icon></v-btn
                  >
                </td>
              </tr>
            </tbody>
          </v-simple-table>
        </v-tab-item>
        <v-tab-item :key="2">
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
        <v-tab-item :key="3">
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
        </v-tab-item>
      </v-tabs-items>
    </v-container>
  </div>
</template>

<script>
import SearchBar from "./SearchBar.vue";
import { mapMutations, mapGetters, mapActions, mapState } from "vuex";

export default {
  name: "Home",
  components: { SearchBar },
  data() {
    return {
      tabs: 0,
      dataTab: 0,
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
      "prevSearchPage",
      "nextSearchPage",
    ]),
    ...mapState(["searchQuery", "tags", "tagAddStatus", "tagDelStatus"]),
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
      "delTag",
    ]),
  },
  watch: {
    searchRunning() {
      this.tabs = 2;
    },
    streamLoading() {
      this.$vuetify.goTo(0, {});
      this.tabs = 3;
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
            return "&#x"+("00" + i.toString(16)).substr(-2)+";";
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

<style>
.v-tabs__content {
  padding-bottom: 2px;
}
.streams-table tbody tr :hover {
  cursor: pointer;
}
</style>