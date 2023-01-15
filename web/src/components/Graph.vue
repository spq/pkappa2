<template>
  <div>
    <ToolBar>
      <v-tooltip bottom>
        <template #activator="{ on, attrs }">
          <v-btn v-bind="attrs" v-on="on" icon @click="fetchGraphLocal">
            <v-icon>mdi-refresh</v-icon>
          </v-btn>
        </template>
        <span>Refresh</span>
      </v-tooltip>
      <v-toolbar-items class="pt-1">
        <v-select
          :items="Object.keys(chartTypes)"
          v-model="chartType"
          flat
          solo
          dense
          label="Type"
        ></v-select>
        <v-select
          :items="chartTagOptions"
          v-model="chartTags"
          multiple
          flat
          solo
          dense
          label="Grouping"
        >
          <template #item="{ item, attrs, on }">
            <v-list-item v-if="item.value.startsWith('header/')" dense>
              <v-list-item-content>
                <v-subheader
                  >{{ item.text }}
                  <v-btn
                    x-small
                    link
                    text
                    @click="setChartTagOptions(item.value.substring(7), true)"
                    >All</v-btn
                  >
                  <v-btn
                    x-small
                    link
                    text
                    @click="setChartTagOptions(item.value.substring(7), false)"
                    >None</v-btn
                  ></v-subheader
                >
              </v-list-item-content>
            </v-list-item>
            <v-list-item v-else v-on="on" v-bind="attrs" #default="{ active }">
              <v-list-item-action>
                <v-checkbox :ripple="false" :input-value="active"></v-checkbox>
              </v-list-item-action>
              <v-list-item-content>{{ item.text }}</v-list-item-content>
            </v-list-item>
          </template>
        </v-select>
      </v-toolbar-items>
    </ToolBar>
    <v-skeleton-loader type="image" v-if="graph.running"></v-skeleton-loader>
    <v-alert type="error" dense v-else-if="graph.error">{{
      graph.error
    }}</v-alert>
    <div v-else-if="chartData != null && chartOptions != null">
      <apexchart
        type="area"
        :options="chartOptions"
        :series="chartData"
        height="400px"
      ></apexchart>
      <v-text-field disabled v-model="chartTimeFilter"></v-text-field>
    </div>
  </div>
</template>

<style>
.apexcharts-toolbar {
    z-index: 0 !important;
}
</style>

<script>
import { mapActions, mapState } from "vuex";
import ToolBar from './ToolBar.vue';

export default {
  components: { ToolBar },
  name: "Graph",
  data() {
    return {
      chartOptions: null,
      chartData: null,
      chartTimeFilter: "",
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
  mounted() {
    this.fetchGraphLocal();
  },
  computed: {
    ...mapState(["tags", "graph"]),
    chartType: {
      get() {
        return this.$route.query.t;
      },
      set(v) {
        this.$router.push({
          name: "graph",
          query: {
            t: v,
            g: this.$route.query.g,
            q: this.$route.query.q,
          },
        });
      },
    },
    chartTags: {
      get() {
        let g = this.$route.query.g;
        if (!g) g = "[]";
        return JSON.parse(g);
      },
      set(v) {
        this.$router.push({
          name: "graph",
          query: {
            t: this.$route.query.t,
            g: JSON.stringify(v),
            q: this.$route.query.q,
          },
        });
      },
    },
    chartTagOptions: function () {
      if (this.tags == null) return [];
      const options = [];
      const types = ["tag", "service", "mark", "generated"];
      for (const typ of types) {
        let first = true;
        for (const t of this.tags) {
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
              value: t.Name,
            });
          }
        }
      }
      return options;
    },
  },
  methods: {
    ...mapActions(["fetchGraph"]),
    setChartTagOptions(typ, active) {
      this.$nextTick(() => {
        const sel = this.chartTags;
        for (var i = 0; i < sel.length; i++) {
          if (sel[i].startsWith(`${typ}/`)) {
            sel.splice(i--, 1);
          }
        }
        if (active) {
          for (const t of this.chartTagOptions) {
            if (t.value.startsWith(`${typ}/`)) {
              sel.push(t.value);
            }
          }
        }
        this.chartTags = sel;
      });
    },
    fetchGraphLocal() {
      const type = this.chartType;
      if (!type) return;
      let tags = this.chartTags;
      if (!tags) tags = [];
      let query = this.$route.query.q;
      if (!query) query = null;

      this.chartData = null;
      this.fetchGraph({
        delta: "1m",
        aspects: this.chartTypes[type].aspects,
        tags: tags,
        query: query,
        type,
      });
    },
  },
  watch: {
    $route: function () {
      this.fetchGraphLocal();
    },
    graph(val) {
      const type = this.chartTypes[val.type];
      val = val.graph;
      if (val == null) {
        this.chartData = null;
        this.chartOptions = null;
        return;
      }
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
          if (g == null || p >= g.length) continue;
          const v = g[p][0];
          if (min === null || min > v) min = v;
        }
        if (min === null) break;
        const t = tMin + min * delta;
        for (let i = 0; i < pos.length; i++) {
          const p = pos[i];
          const g = val.Data[i].Data;
          let v = null;
          if (g != null && p < g.length) v = g[p];
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