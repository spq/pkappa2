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
          <HelpPage></HelpPage>
        </v-tab-item>
        <v-tab-item :key="1">
          <TabTags v-on:searchStreams="searchStreams" />
        </v-tab-item>
        <v-tab-item :key="2">
          <v-card>
            <v-select
              :items="Object.keys(chartTypes)"
              v-model="chartType"
              dense
            ></v-select>
            <v-select
              :items="chartTagOptions"
              v-model="chartTagSelection"
              multiple
              dense
            >
              <template v-slot:item="{ item, attrs, on }">
                <v-list-item v-if="item.value.startsWith('header/')" dense>
                  <v-list-item-content>
                    <v-subheader
                      >{{ item.text }}
                      <v-btn
                        x-small
                        link
                        text
                        @click="
                          setChartTagOptions(item.value.substring(7), true)
                        "
                        >All</v-btn
                      >
                      <v-btn
                        x-small
                        link
                        text
                        @click="
                          setChartTagOptions(item.value.substring(7), false)
                        "
                        >None</v-btn
                      ></v-subheader
                    >
                  </v-list-item-content>
                </v-list-item>
                <v-list-item
                  v-else
                  v-on="on"
                  v-bind="attrs"
                  #default="{ active }"
                >
                  <v-list-item-action>
                    <v-simple-checkbox
                      :ripple="false"
                      :value="active"
                    ></v-simple-checkbox>
                  </v-list-item-action>
                  <v-list-item-content>{{ item.text }}</v-list-item-content>
                </v-list-item>
              </template>
            </v-select>
            <template v-if="chartType === null"></template>
            <template v-else-if="chartData === null">
              <v-progress-linear indeterminate></v-progress-linear>
            </template>
            <template v-else>
              <apexchart
                type="area"
                :options="chartOptions"
                :series="chartData"
                height="400px"
              ></apexchart>
              <v-text-field disabled v-model="chartTimeFilter"></v-text-field>
            </template>
          </v-card>
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
          <TabStream/>
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
import { mapMutations, mapGetters, mapActions, mapState } from "vuex";

export default {
  name: "Home",
  components: { SearchBar, TabTags, HelpPage, TabStream },
  data() {
    return {
      tabs: 0,
      newTagName: "",
      chartOptions: null,
      chartData: null,
      chartType: null,
      chartTimeFilter: "",
      chartTagOptions: null,
      chartTagSelection: [],
      chartTypes: {
        "Active Connections": {
          aspects: ["connections@first", "connections@last"],
          make(pos) {
            return {
              groups: [],
              addGroup(tags) {
                const g = {
                  tags: tags,
                  data: [],
                  cur: 0,
                  add(t, data) {
                    if (data === null) {
                      this.data.push([t, this.cur]);
                      return;
                    }
                    const f = data[pos[0]];
                    const l = data[pos[1]];
                    this.cur += f;
                    this.data.push([t, this.cur]);
                    this.cur -= l;
                  },
                };
                this.groups.push(g);
                return g;
              },
              build(data) {
                for (const g of this.groups) {
                  const hideTags =
                    this.groups.length == 1 &&
                    (g.tags === null || g.tags.length == 0);
                  data.push({
                    name: hideTags ? "Connections" : `Connections (${g.tags})`,
                    data: g.data,
                  });
                }
              },
            };
          },
        },
        "Started Connections": {
          aspects: ["connections@first"],
          make(pos) {
            return {
              groups: [],
              addGroup(tags) {
                const g = {
                  tags: tags,
                  data: [],
                  add(t, data) {
                    this.data.push([t, data === null ? 0 : data[pos[0]]]);
                  },
                };
                this.groups.push(g);
                return g;
              },
              build(data) {
                for (const g of this.groups) {
                  const hideTags =
                    this.groups.length == 1 &&
                    (g.tags === null || g.tags.length == 0);
                  data.push({
                    name: hideTags ? "Connections" : `Connections (${g.tags})`,
                    data: g.data,
                  });
                }
              },
            };
          },
        },
        "Finished Connections": {
          aspects: ["connections@last"],
          make(pos) {
            return {
              groups: [],
              addGroup(tags) {
                const g = {
                  tags: tags,
                  data: [],
                  add(t, data) {
                    this.data.push([t, data === null ? 0 : data[pos[0]]]);
                  },
                };
                this.groups.push(g);
                return g;
              },
              build(data) {
                for (const g of this.groups) {
                  const hideTags =
                    this.groups.length == 1 &&
                    (g.tags === null || g.tags.length == 0);
                  data.push({
                    name: hideTags ? "Connections" : `Connections (${g.tags})`,
                    data: g.data,
                  });
                }
              },
            };
          },
        },
        "Total Traffic": {
          aspects: ["cbytes@first", "sbytes@first"],
          make(pos) {
            return {
              groups: [],
              addGroup(tags) {
                const g = {
                  tags: tags,
                  cbytes: [],
                  sbytes: [],
                  add(t, data) {
                    if (data === null) {
                      this.cbytes.push([t, 0]);
                      this.sbytes.push([t, 0]);
                      return;
                    }
                    this.cbytes.push([t, -data[pos[0]]]);
                    this.sbytes.push([t, data[pos[1]]]);
                  },
                };
                this.groups.push(g);
                return g;
              },
              build(data, options) {
                for (const g of this.groups) {
                  const hideTags =
                    this.groups.length == 1 &&
                    (g.tags === null || g.tags.length == 0);
                  data.push({
                    name: hideTags
                      ? "Server Bytes"
                      : `Server Bytes (${g.tags})`,
                    data: g.sbytes,
                  });
                  data.push({
                    name: hideTags
                      ? "Client Bytes"
                      : `Client Bytes (${g.tags})`,
                    data: g.cbytes,
                  });
                }
                options.chart.stacked = false;
                options.yaxis = {
                  labels: {
                    formatter: (v) => {
                      if (v < 0) v = -v;
                      let f = 0;
                      while (v >= 1024) {
                        v /= 1024;
                        f++;
                      }
                      return (
                        v.toFixed(1) +
                        " " +
                        ["B", "KiB", "MiB", "GiB", "TiB", "PiB"][f]
                      );
                    },
                  },
                };
              },
            };
          },
        },
        "Average Traffic": {
          aspects: ["cbytes@first", "sbytes@first", "connections@first"],
          make(pos) {
            const p0 = pos[0];
            const p1 = pos[1];
            return {
              groups: [],
              addGroup(tags) {
                const g = {
                  tags: tags,
                  cbytes: [],
                  sbytes: [],
                  add(t, data) {
                    if (data === null) {
                      this.cbytes.push([t, 0]);
                      this.sbytes.push([t, 0]);
                      return;
                    }
                    let c = data[pos[2]];
                    if (c < 1) c = 1;
                    this.cbytes.push([t, data === null ? 0 : -data[p0] / c]);
                    this.sbytes.push([t, data === null ? 0 : data[p1] / c]);
                  },
                };
                this.groups.push(g);
                return g;
              },
              build(data, options) {
                for (const g of this.groups) {
                  const hideTags =
                    this.groups.length == 1 &&
                    (g.tags === null || g.tags.length == 0);
                  data.push({
                    name: hideTags
                      ? "Server Bytes"
                      : `Server Bytes (${g.tags})`,
                    data: g.sbytes,
                  });
                  data.push({
                    name: hideTags
                      ? "Client Bytes"
                      : `Client Bytes (${g.tags})`,
                    data: g.cbytes,
                  });
                }
                options.chart.stacked = false;
                options.yaxis = {
                  labels: {
                    formatter: (v) => {
                      if (v < 0) v = -v;
                      let f = 0;
                      while (v >= 1024) {
                        v /= 1024;
                        f++;
                      }
                      return (
                        v.toFixed(1) +
                        " " +
                        ["B", "KiB", "MiB", "GiB", "TiB", "PiB"][f]
                      );
                    },
                  },
                };
              },
            };
          },
        },
        "Total Duration": {
          aspects: ["duration@first"],
          make(pos) {
            return {
              groups: [],
              addGroup(tags) {
                const g = {
                  tags: tags,
                  data: [],
                  add(t, data) {
                    this.data.push([
                      t,
                      data === null ? 0 : data[pos[0]] / 1_000_000,
                    ]);
                  },
                };
                this.groups.push(g);
                return g;
              },
              build(data, options) {
                for (const g of this.groups) {
                  const hideTags =
                    this.groups.length == 1 &&
                    (g.tags === null || g.tags.length == 0);
                  data.push({
                    name: hideTags ? "Duration" : `Duration (${g.tags})`,
                    data: g.data,
                  });
                }
                options.yaxis = {
                  labels: {
                    formatter: (v) => {
                      if (v < 1_000) return v + "ms";
                      v /= 1_000;
                      if (v < 60) return v.toFixed(3) + "s";
                      v /= 60;
                      if (v < 60)
                        return (
                          v.toFixed(0) + "m" + ((v % 1) * 60).toFixed(0) + "s"
                        );
                      v /= 60;
                      return (
                        v.toFixed(0) + "h" + ((v % 1) * 60).toFixed(0) + "m"
                      );
                    },
                  },
                };
              },
            };
          },
        },
        "Average Duration": {
          aspects: ["duration@first", "connections@first"],
          make(pos) {
            return {
              groups: [],
              addGroup(tags) {
                const g = {
                  tags: tags,
                  data: [],
                  add(t, data) {
                    const d = data === null ? 0 : data[pos[0]];
                    const c = data === null ? 0 : data[pos[1]];
                    this.data.push([t, c != 0 ? d / c / 1_000_000 : 0]);
                  },
                };
                this.groups.push(g);
                return g;
              },
              build(data, options) {
                for (const g of this.groups) {
                  const hideTags =
                    this.groups.length == 1 &&
                    (g.tags === null || g.tags.length == 0);
                  data.push({
                    name: hideTags ? "Duration" : `Duration (${g.tags})`,
                    data: g.data,
                  });
                }
                options.yaxis = {
                  labels: {
                    formatter: (v) => {
                      if (v < 1_000) return v + "ms";
                      v /= 1_000;
                      if (v < 60) return v.toFixed(3) + "s";
                      v /= 60;
                      if (v < 60)
                        return (
                          v.toFixed(0) + "m" + ((v % 1) * 60).toFixed(0) + "s"
                        );
                      v /= 60;
                      return (
                        v.toFixed(0) + "h" + ((v % 1) * 60).toFixed(0) + "m"
                      );
                    },
                  },
                };
              },
            };
          },
        },
      },
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
    ...mapState(["searchQuery", "tags", "tagAddStatus", "graphData"]),
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
      "updateGraph",
    ]),
    setChartTagOptions(typ, active) {
      this.$nextTick(() => {
        const sel = this.chartTagSelection;
        for (var i = 0; i < sel.length; i++) {
          if (sel[i].startsWith(`entry/${typ}/`)) {
            sel.splice(i--, 1);
          }
        }
        if (active) {
          for (const t of this.chartTagOptions) {
            if (t.value.startsWith(`entry/${typ}/`)) {
              sel.push(t.value);
            }
          }
        }
        this.chartTagSelection = sel;
      });
    },
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
    tags(val) {
      const options = [];
      for (const typ of ["tag", "service", "mark"]) {
        let first = true;
        for (const t of val) {
          if (t.Name.startsWith(typ.toLowerCase() + "/")) {
            if (first) {
              first = false;
              options.push({
                text: typ.charAt(0).toUpperCase() + typ.substring(1) + "s",
                value: `header/${typ}`,
              });
            }
            options.push({
              text: t.Name,
              value: `entry/${t.Name}`,
            });
          }
        }
      }
      this.chartTagOptions = options;
    },
    chartTagSelection(val) {
      if (this.chartType === null) return;
      const tags = [];
      for (const t of val) {
        tags.push(t.substr(6));
      }
      this.chartData = null;
      this.updateGraph({
        delta: "1m",
        aspects: this.chartTypes[this.chartType].aspects,
        tags: tags,
      });
    },
    chartType(val) {
      const tags = [];
      for (const t of this.chartTagSelection) {
        tags.push(t.substr(6));
      }
      this.chartData = null;
      this.updateGraph({
        delta: "1m",
        aspects: this.chartTypes[val].aspects,
        tags: tags,
      });
    },
    graphData(val) {
      const type = this.chartTypes[this.chartType];
      const valueIndex = [];
      for (const a of type.aspects) {
        const i = val.Aspects.indexOf(a);
        if (i < 0) return;
        valueIndex.push(i + 1);
      }

      const delta = val.Delta / 1_000_000;
      const tMin = Date.parse(val.Min);
      //const tMax = Date.parse(val.Max);

      const obj = type.make(valueIndex);
      const groups = [];
      const pos = [];
      for (const tg of val.Data) {
        groups.push(obj.addGroup(tg.Tags));
        pos.push(0);
      }
      for (;;) {
        let min = null;
        for (let i = 0; i < pos.length; i++) {
          const p = pos[i];
          const g = val.Data[i].Data;
          if (p >= g.length) continue;
          const v = g[p][0];
          if (min === null || min > v) min = v;
        }
        if (min === null) break;
        const t = tMin + min * delta;
        for (let i = 0; i < pos.length; i++) {
          const p = pos[i];
          const g = val.Data[i].Data;
          let v = null;
          if (p < g.length) v = g[p];
          if (v != null && v[0] === min) {
            groups[i].add(t, v);
            pos[i]++;
          } else {
            groups[i].add(t, null);
          }
        }
      }
      const that = this;
      const updateChartFilter = function (min, max) {
        if (min !== undefined && max !== undefined) {
          const fmt = (t) => {
            t = new Date(t);
            return `${t.getFullYear()}-${(1 + t.getMonth())
              .toString()
              .padStart(2, "0")}-${t.getDate().toString().padStart(2, "0")} ${t
              .getHours()
              .toString()
              .padStart(2, "0")}${t.getMinutes().toString().padStart(2, "0")}`;
          };
          that.chartTimeFilter = `time:"${fmt(min)}:${fmt(max + 60_000)}"`;
        } else {
          that.chartTimeFilter = "";
        }
      };
      this.chartOptions = {
        dataLabels: {
          enabled: false,
        },
        xaxis: {
          type: "datetime",
          labels: {
            datetimeUTC: false,
            datetimeFormatter: {
              day: "dd. HH:mm",
              hour: "HH:mm",
            },
          },
        },
        legend: {
          position: "top",
          horizontalAlign: "left",
        },
        tooltip: {
          x: {
            format: "HH:mm",
          },
        },
        chart: {
          stacked: true,
          events: {
            mounted() {
              updateChartFilter();
            },
            zoomed(ctx, { xaxis }) {
              updateChartFilter(xaxis.min, xaxis.max);
            },
            scrolled(ctx, { xaxis }) {
              updateChartFilter(xaxis.min, xaxis.max);
            },
          },
        },
      };
      this.chartData = [];
      obj.build(this.chartData, this.chartOptions);
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