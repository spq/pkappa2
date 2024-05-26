<template>
  <div>
    <ToolBar>
      <v-tooltip bottom>
        <template #activator="{ on, attrs }">
          <v-btn v-bind="attrs" icon v-on="on" @click="fetchGraphLocal">
            <v-icon>mdi-refresh</v-icon>
          </v-btn>
        </template>
        <span>Refresh</span>
      </v-tooltip>
      <v-toolbar-items class="pt-1">
        <v-select
          v-model="chartType"
          :items="Object.keys(chartTypes)"
          flat
          solo
          dense
          label="Type"
        ></v-select>
        <v-select
          v-model="chartTags"
          :items="chartTagOptions"
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
            <v-list-item v-else v-slot="{ active }" v-bind="attrs" v-on="on">
              <v-list-item-action>
                <v-checkbox :ripple="false" :input-value="active"></v-checkbox>
              </v-list-item-action>
              <v-list-item-content>{{ item.text }}</v-list-item-content>
            </v-list-item>
          </template>
        </v-select>
      </v-toolbar-items>
    </ToolBar>
    <v-skeleton-loader
      v-if="graphStore.running"
      type="image"
    ></v-skeleton-loader>
    <v-alert v-else-if="graphStore.error" type="error" dense>{{
      graphStore.error
    }}</v-alert>
    <div v-else-if="chartData != null && chartOptions != null">
      <VueApexChartsComponent
        type="area"
        :options="chartOptions"
        :series="chartData"
        height="400px"
      ></VueApexChartsComponent>
      <v-text-field v-model="chartTimeFilter" disabled></v-text-field>
    </div>
  </div>
</template>

<script lang="ts" setup>
import ToolBar from "./ToolBar.vue";
import { computed, nextTick, ref, onMounted, watch } from "vue";
import { EventBus } from "./EventBus";
import { useRootStore } from "@/stores";
import { GraphType, useGraphStore } from "@/stores/graph";
import { useRoute, useRouter } from "vue-router/composables";
import VueApexChartsComponent from "vue-apexcharts";
import * as ApexCharts from "apexcharts";

const store = useRootStore();
const graphStore = useGraphStore();
const route = useRoute();
const router = useRouter();
const chartOptions = ref<ApexCharts.ApexOptions | null>(null);
const chartData = ref<ChartData[] | null>(null);
const chartTimeFilter = ref("");

type Group = {
  tags: string[];
  data: [number, number][];
  add(t: number, data: number[] | null): void;
};

type ActiveConnectionsGroup = Group & {
  cur: number;
};

type TrafficGroup = Group & {
  cbytes: [number, number][];
  sbytes: [number, number][];
};

type MakeType<GroupType> = {
  groups: GroupType[];
  addGroup(tags: string[]): GroupType;
  build(data: ChartData[], options: ApexCharts.ApexOptions): void;
};

type ChartData = {
  name: string;
  data: [number, number][];
};

type SomeGroup =
  | MakeType<TrafficGroup>
  | MakeType<ActiveConnectionsGroup>
  | MakeType<Group>;

type ChartType = {
  aspects: string[];
  make: (pos: number[]) => SomeGroup;
};

const chartTypes: { [key: string]: ChartType } = {
  "Active Connections": {
    aspects: ["connections@first", "connections@last"],
    make(pos) {
      const result: MakeType<ActiveConnectionsGroup> = {
        groups: [],
        addGroup(tags): ActiveConnectionsGroup {
          const g: ActiveConnectionsGroup = {
            tags: tags,
            cur: 0,
            data: [],
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
              name: hideTags
                ? "Connections"
                : `Connections (${g.tags.join(",")})`,
              data: g.data,
            });
          }
        },
      };
      return result;
    },
  },
  "Started Connections": {
    aspects: ["connections@first"],
    make(pos) {
      const result: MakeType<Group> = {
        groups: [],
        addGroup(tags: string[]): Group {
          const g: Group = {
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
              name: hideTags
                ? "Connections"
                : `Connections (${g.tags.join(",")})`,
              data: g.data,
            });
          }
        },
      };
      return result;
    },
  },
  "Finished Connections": {
    aspects: ["connections@last"],
    make(pos) {
      const result: MakeType<Group> = {
        groups: [],
        addGroup(tags) {
          const g: Group = {
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
              name: hideTags
                ? "Connections"
                : `Connections (${g.tags.join(",")})`,
              data: g.data,
            });
          }
        },
      };
      return result;
    },
  },
  "Total Traffic": {
    aspects: ["cbytes@first", "sbytes@first"],
    make(pos) {
      const result: MakeType<TrafficGroup> = {
        groups: [],
        addGroup(tags) {
          const g: TrafficGroup = {
            tags: tags,
            data: [],
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
                : `Server Bytes (${g.tags.join(",")})`,
              data: g.sbytes,
            });
            data.push({
              name: hideTags
                ? "Client Bytes"
                : `Client Bytes (${g.tags.join(",")})`,
              data: g.cbytes,
            });
          }
          options.chart!.stacked = false;
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
      return result;
    },
  },
  "Average Traffic": {
    aspects: ["cbytes@first", "sbytes@first", "connections@first"],
    make(pos) {
      const p0 = pos[0];
      const p1 = pos[1];
      const result: MakeType<TrafficGroup> = {
        groups: [],
        addGroup(tags) {
          const g: TrafficGroup = {
            tags: tags,
            data: [],
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
                : `Server Bytes (${g.tags.join(",")})`,
              data: g.sbytes,
            });
            data.push({
              name: hideTags
                ? "Client Bytes"
                : `Client Bytes (${g.tags.join(",")})`,
              data: g.cbytes,
            });
          }
          options.chart!.stacked = false;
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
      return result;
    },
  },
  "Total Duration": {
    aspects: ["duration@first"],
    make(pos) {
      const result: MakeType<Group> = {
        groups: [],
        addGroup(tags) {
          const g: Group = {
            tags: tags,
            data: [],
            add(t, data) {
              this.data.push([t, data === null ? 0 : data[pos[0]] / 1_000_000]);
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
              name: hideTags ? "Duration" : `Duration (${g.tags.join(",")})`,
              data: g.data,
            });
          }
          options.yaxis = {
            labels: {
              formatter: (v) => {
                if (v < 1_000) return v.toString() + "ms";
                v /= 1_000;
                if (v < 60) return v.toFixed(3) + "s";
                v /= 60;
                if (v < 60)
                  return v.toFixed(0) + "m" + ((v % 1) * 60).toFixed(0) + "s";
                v /= 60;
                return v.toFixed(0) + "h" + ((v % 1) * 60).toFixed(0) + "m";
              },
            },
          };
        },
      };
      return result;
    },
  },
  "Average Duration": {
    aspects: ["duration@first", "connections@first"],
    make(pos) {
      const result: MakeType<Group> = {
        groups: [],
        addGroup(tags): Group {
          const g: Group = {
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
              name: hideTags ? "Duration" : `Duration (${g.tags.join(",")})`,
              data: g.data,
            });
          }
          options.yaxis = {
            labels: {
              formatter: (v) => {
                if (v < 1_000) return v.toString() + "ms";
                v /= 1_000;
                if (v < 60) return v.toFixed(3) + "s";
                v /= 60;
                if (v < 60)
                  return v.toFixed(0) + "m" + ((v % 1) * 60).toFixed(0) + "s";
                v /= 60;
                return v.toFixed(0) + "h" + ((v % 1) * 60).toFixed(0) + "m";
              },
            },
          };
        },
      };
      return result;
    },
  },
};

const chartType = computed({
  get: () => {
    return route.query.t as string;
  },
  set: (v: string) => {
    void router.push({
      name: "graph",
      query: {
        t: v,
        g: route.query.g ?? "",
        q: route.query.q ?? "",
      },
    });
  },
});
const chartTags = computed({
  get: () => {
    let g = route.query.g as string;
    if (!g) g = "[]";
    return JSON.parse(g) as string[];
  },
  set: (v: string[]) => {
    void router.push({
      name: "graph",
      query: {
        t: route.query.t ?? "",
        g: JSON.stringify(v),
        q: route.query.q ?? "",
      },
    });
  },
});
const chartTagOptions = computed(() => {
  if (store.tags === null) return [];
  const options = [];
  const types = ["tag", "service", "mark", "generated"];
  for (const typ of types) {
    let first = true;
    for (const t of store.tags) {
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
});

watch(route, () => {
  fetchGraphLocal();
});
graphStore.$subscribe((_mutation, state) => {
  if (state === null || state.type === null) {
    chartData.value = null;
    chartOptions.value = null;
    return;
  }
  const type = chartTypes[state.type];
  const graphVal = state.graph;
  if (graphVal == null) {
    chartData.value = null;
    chartOptions.value = null;
    return;
  }
  const valueIndex = [];
  for (const a of type.aspects) {
    const i = graphVal.Aspects.indexOf(a);
    if (i < 0) return;
    valueIndex.push(i + 1);
  }

  const delta = graphVal.Delta / 1_000_000;
  const tMin = Date.parse(graphVal.Min);
  //const tMax = Date.parse(val.Max);

  const obj = type.make(valueIndex);
  const groups = [];
  const pos = [];
  for (const tg of graphVal.Data) {
    groups.push(obj.addGroup(tg.Tags));
    pos.push(0);
  }
  for (;;) {
    let min = null;
    for (let i = 0; i < pos.length; i++) {
      const p = pos[i];
      const g = graphVal.Data[i].Data;
      if (g == null || p >= g.length) continue;
      const v = g[p][0];
      if (min === null || min > v) min = v;
    }
    if (min === null) break;
    const t = tMin + min * delta;
    for (let i = 0; i < pos.length; i++) {
      const p = pos[i];
      const g = graphVal.Data[i].Data;
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
  const updateChartFilter = function (
    min: number | undefined,
    max: number | undefined
  ) {
    if (min !== undefined && max !== undefined) {
      const fmt = (t: number) => {
        const d = new Date(t);
        return `${d.getFullYear()}-${(1 + d.getMonth())
          .toString()
          .padStart(2, "0")}-${d.getDate().toString().padStart(2, "0")} ${d
          .getHours()
          .toString()
          .padStart(2, "0")}${d.getMinutes().toString().padStart(2, "0")}`;
      };
      chartTimeFilter.value = `time:"${fmt(min)}:${fmt(max + 60_000)}"`;
    } else {
      chartTimeFilter.value = "";
    }
  };
  chartOptions.value = {
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
          updateChartFilter(undefined, undefined);
        },
        zoomed(_, { xaxis }: ApexCharts.ApexOptions) {
          updateChartFilter(xaxis?.min, xaxis?.max);
        },
        scrolled(_, { xaxis }: ApexCharts.ApexOptions) {
          updateChartFilter(xaxis?.min, xaxis?.max);
        },
      },
    },
  };
  chartData.value = [];
  obj.build(chartData.value, chartOptions.value);
});

onMounted(() => {
  fetchGraphLocal();
});

function setChartTagOptions(typ: string, active: boolean) {
  nextTick(() => {
    const sel = chartTags.value;
    for (var i = 0; i < sel.length; i++) {
      if (sel[i].startsWith(`${typ}/`)) {
        sel.splice(i--, 1);
      }
    }
    if (active) {
      for (const t of chartTagOptions.value) {
        if (t.value.startsWith(`${typ}/`)) {
          sel.push(t.value);
        }
      }
    }
    chartTags.value = sel;
  });
}

function fetchGraphLocal() {
  const type = chartType.value;
  if (!type) return;
  let tags = chartTags.value;
  if (!tags) tags = [];
  let query: typeof route.query.q | null = route.query.q;
  if (!query) query = null;

  chartData.value = null;
  graphStore
    .fetchGraph(
      "1m",
      chartTypes[type].aspects,
      tags,
      query as string,
      type as GraphType
    )
    .catch((err: string) => {
      EventBus.emit("showError", `Failed to update graph: ${err}`);
    });
}
</script>

<style>
.apexcharts-toolbar {
  z-index: 0 !important;
}
</style>
