<template>
  <div>
    <ToolBar>
      <v-tooltip bottom>
        <template #activator="{ on, attrs }">
          <v-btn
            v-bind="attrs"
            icon
            :disabled="
              streams.result == null || streams.result.Results.length == 0
            "
            v-on="on"
            @click="checkboxAction"
          >
            <v-icon
              >mdi-{{
                noneSelected
                  ? "checkbox-blank-outline"
                  : allSelected
                  ? "checkbox-marked"
                  : "minus-box"
              }}</v-icon
            >
          </v-btn>
        </template>
        <span>Select</span>
      </v-tooltip>
      <div v-if="noneSelected">
        <v-tooltip bottom>
          <template #activator="{ on, attrs }">
            <v-btn v-bind="attrs" icon v-on="on" @click="fetchStreams">
              <v-icon>mdi-refresh</v-icon>
            </v-btn>
          </template>
          <span>Refresh</span>
        </v-tooltip>
      </div>
      <div v-else>
        <v-menu offset-y right bottom
          ><template #activator="{ on: onMenu, attrs }">
            <v-tooltip bottom>
              <template #activator="{ on: onTooltip }">
                <v-btn v-bind="attrs" icon v-on="{ ...onMenu, ...onTooltip }">
                  <v-icon>mdi-checkbox-multiple-outline</v-icon>
                </v-btn>
              </template>
              <span>Marks</span>
            </v-tooltip>
          </template>
          <v-list dense>
            <v-list-item
              v-for="tag of groupedTags.mark"
              :key="tag.Name"
              link
              @click="
                markSelectedStreams(
                  tag.Name,
                  tagStatusForSelection[tag.Name] !== true
                )
              "
            >
              <v-list-item-action>
                <v-icon
                  >mdi-{{
                    tagStatusForSelection[tag.Name] === true
                      ? "checkbox-outline"
                      : tagStatusForSelection[tag.Name] === false
                      ? "minus-box"
                      : "checkbox-blank-outline"
                  }}</v-icon
                >
              </v-list-item-action>
              <v-list-item-content>
                <v-list-item-title>{{
                  tagify(tag.Name, "name")
                }}</v-list-item-title>
              </v-list-item-content>
            </v-list-item>
            <v-divider />
            <v-list-item link @click="createMarkFromSelection">
              <v-list-item-action />
              <v-list-item-title>Create new</v-list-item-title>
            </v-list-item>
          </v-list>
        </v-menu>
      </div>
      <v-alert
        v-if="streams.outdated"
        class="toolbar-alert"
        type="info"
        outlined
        dense
        >Results might be outdated.</v-alert
      >
      <v-spacer />
      <div
        v-if="
          !streams.running &&
          !streams.error &&
          streams.result &&
          streams.result.Results.length != 0
        "
      >
        <span class="text-caption"
          >{{ streams.result.Offset + 1 }}–{{
            streams.result.Offset + streams.result.Results.length
          }}
          of
          {{
            streams.result.MoreResults
              ? "many"
              : streams.result.Results.length + streams.result.Offset
          }}</span
        >
        <v-tooltip bottom>
          <template #activator="{ on, attrs }">
            <v-btn
              v-bind="attrs"
              icon
              :disabled="streams.page == 0"
              v-on="on"
              @click="
                $router.push({
                  name: 'search',
                  query: {
                    q: $route.query.q,
                    p: (Number($route.query.p ?? 0) - 1).toString(),
                  },
                })
              "
            >
              <v-icon>mdi-chevron-left</v-icon>
            </v-btn>
          </template>
          <span>Previous Page</span>
        </v-tooltip>
        <v-tooltip bottom>
          <template #activator="{ on, attrs }">
            <v-btn
              v-bind="attrs"
              icon
              :disabled="!streams.result.MoreResults"
              v-on="on"
              @click="
                $router.push({
                  name: 'search',
                  query: {
                    q: $route.query.q,
                    p: (Number($route.query.p ?? 0) + 1).toString(),
                  },
                })
              "
            >
              <v-icon>mdi-chevron-right</v-icon>
            </v-btn>
          </template>
          <span>Next Page</span>
        </v-tooltip>
      </div>
    </ToolBar>
    <v-skeleton-loader
      v-if="streams.running || (!streams.result && !streams.error)"
      type="table-thead, table-tbody"
    ></v-skeleton-loader>
    <div v-else-if="streams.error">
      <v-alert type="error" border="left">{{ streams.error }}</v-alert>
      <v-alert type="info" border="left"
        ><v-row>
          <v-col class="grow"
            >did you mean to search for the text directly?</v-col
          >
          <v-col class="shrink">
            <v-btn
              @click="
                $router.push({
                  name: 'search',
                  query: {
                    q: `data:\x22${regexEscape($route.query.q)}\x22`,
                  },
                })
              "
              >Search for the input</v-btn
            >
          </v-col></v-row
        ></v-alert
      >
    </div>
    <center
      v-else-if="streams.result === null || streams.result.Results.length === 0"
    >
      <v-icon>mdi-magnify</v-icon
      ><span class="text-subtitle-1">No streams matched your search.</span>
    </center>
    <v-simple-table v-else dense>
      <template #default>
        <thead>
          <tr>
            <th style="width: 0" class="pr-0"></th>
            <th class="text-left pl-0">Tags</th>
            <th class="text-left">Client</th>
            <th class="text-left">Bytes</th>
            <th class="text-left">Server</th>
            <th class="text-left">Bytes</th>
            <th class="text-right pr-0">Time</th>
            <th style="width: 0" class="px-0"></th>
          </tr>
        </thead>
        <tbody>
          <router-link
            v-for="(stream, index) in streams.result.Results"
            :key="index"
            v-slot="{ navigate }"
            :to="{
              name: 'stream',
              query: { q: $route.query.q, p: $route.query.p },
              params: { streamId: stream.Stream.ID.toString() },
            }"
            custom
            style="cursor: pointer"
            :class="{ blue: selected[index], 'lighten-5': selected[index] }"
          >
            <tr
              role="link"
              @click="isTextSelected() || navigate($event)"
              @keypress.enter="navigate"
            >
              <td style="width: 0" class="pr-0">
                <v-simple-checkbox
                  v-model="selected[index]"
                ></v-simple-checkbox>
              </td>
              <td class="pl-0">
                <v-hover
                  v-for="tag in stream.Tags"
                  v-slot="{ hover }"
                  :key="tag"
                  ><v-chip small :color="tagColors[tag]"
                    ><template v-if="hover"
                      >{{ capitalize(tagify(tag, "type")) }}
                      {{ tagify(tag, "name") }}</template
                    ><template v-else>{{
                      tagify(tag, "name")
                    }}</template></v-chip
                  ></v-hover
                >
              </td>
              <td>
                {{ stream.Stream.Client.Host }}:{{ stream.Stream.Client.Port }}
              </td>
              <td>
                <span :title="`${stream.Stream.Client.Bytes} Bytes`">{{
                  $options.filters?.prettyBytes(
                    stream.Stream.Client.Bytes,
                    1,
                    true
                  )
                }}</span>
              </td>
              <td>
                {{ stream.Stream.Server.Host }}:{{ stream.Stream.Server.Port }}
              </td>
              <td>
                <span :title="`${stream.Stream.Server.Bytes} Bytes`">{{
                  $options.filters?.prettyBytes(
                    stream.Stream.Server.Bytes,
                    1,
                    true
                  )
                }}</span>
              </td>
              <td
                class="text-right pr-0"
                :title="formatDateLong(stream.Stream.FirstPacket)"
              >
                {{ formatDate(stream.Stream.FirstPacket) }}
              </td>
              <td style="width: 0" class="px-0">
                <v-btn
                  :href="`/api/download/${stream.Stream.ID}.pcap`"
                  icon
                  @click.native.stop
                >
                  <v-icon>mdi-download</v-icon>
                </v-btn>
              </td>
            </tr>
          </router-link>
        </tbody>
      </template>
    </v-simple-table>
  </div>
</template>

<script lang="ts" setup>
import { EventBus } from "./EventBus";
import ToolBar from "./ToolBar.vue";
import { useRootStore } from "@/stores";
import { useStreamsStore } from "@/stores/streams";
import { computed, onMounted, onBeforeUnmount, ref, watch } from "vue";
import { RouterLink } from "vue-router";
import { useRoute, useRouter } from "vue-router/composables";
import { Result } from "@/apiClient";
import { capitalize, formatDate, formatDateLong, tagify } from "@/filters";

const store = useRootStore();
const route = useRoute();
const router = useRouter();
const streams = useStreamsStore();
const selected = ref<boolean[]>([]);
const tags = computed(() => store.tags);
const groupedTags = computed(() => store.groupedTags);
const selectedCount = computed(
  () => selected.value.filter((i) => i === true).length
);
const noneSelected = computed(() => selectedCount.value === 0);
const allSelected = computed(() => {
  if (selectedCount.value === 0) return false;
  return selectedCount.value === streams.result?.Results.length;
});
const selectedStreams = computed(() => {
  if (streams.result == null) return [];
  const res: Result[] = [];
  for (const [index, value] of Object.entries(selected.value)) {
    if (value) res.push(streams.result.Results[+index]);
  }
  return res;
});
const tagStatusForSelection = computed(() => {
  const counts: { [key: string]: number } = {};
  for (const s of selectedStreams.value) {
    for (const t of s.Tags) {
      if (!(t in counts)) counts[t] = 0;
      counts[t]++;
    }
  }
  const res: { [key: string]: boolean } = {};
  for (const [k, v] of Object.entries(counts)) {
    res[k] = v === selectedStreams.value.length;
  }
  return res;
});
const tagColors = computed(() => {
  const colors: { [key: string]: string } = {};
  tags.value?.forEach((t) => (colors[t.Name] = t.Color));
  return colors;
});

watch(route, () => {
  fetchStreams();
});

onMounted(() => {
  fetchStreams();

  const handle = (e: KeyboardEvent, pageOffset: number) => {
    if (pageOffset >= 1 && !streams.result?.MoreResults) return;
    let p = +route.query.p;
    p += pageOffset;
    if (p < 0) return;
    e.preventDefault();
    void router.push({
      name: "search",
      query: { q: route.query.q, p: p.toString() },
    });
  };
  const handlers: { [key: string]: (e: KeyboardEvent) => void } = {
    j: (e: KeyboardEvent) => {
      handle(e, -1);
    },
    k: (e: KeyboardEvent) => {
      handle(e, 1);
    },
  };
  const keyListener = (e: KeyboardEvent) => {
    if (e.target === null || !(e.target instanceof Element)) return;
    if (["input", "textarea"].includes(e.target.tagName.toLowerCase())) return;

    if (!Object.keys(handlers).includes(e.key)) return;
    handlers[e.key](e);
  };
  window.addEventListener("keydown", keyListener);
  onBeforeUnmount(() => {
    window.removeEventListener("keydown", keyListener);
  });
});

function checkboxAction() {
  let tmp: boolean[] = [];
  const v = noneSelected.value;
  for (let i = 0; i < (streams.result?.Results.length || 0); i++) {
    tmp[i] = v;
  }
  selected.value = tmp;
}

function fetchStreams() {
  streams
    .searchStreams(route.query.q as string, +route.query.p)
    .catch((err: string) => {
      EventBus.emit("showError", `Failed to fetch streams: ${err}`);
    });
  selected.value = [];
}

function createMarkFromSelection() {
  let ids: number[] = [];
  for (const s of selectedStreams.value) {
    ids.push(s.Stream.ID);
  }
  EventBus.emit("showCreateTagDialog", "mark", "", ids);
}

function markSelectedStreams(tagId: string, value: boolean) {
  let ids: number[] = [];
  for (const s of selectedStreams.value) {
    ids.push(s.Stream.ID);
  }
  if (value)
    store.markTagAdd(tagId, ids).catch((err: string) => {
      EventBus.emit("showError", `Failed to add streams to tag: ${err}`);
    });
  else
    store.markTagDel(tagId, ids).catch((err: string) => {
      EventBus.emit("showError", `Failed to remove streams from tag: ${err}`);
    });
}

function isTextSelected() {
  return window?.getSelection()?.type === "Range";
}

function regexEscape(text: string) {
  return text
    .split("")
    .map((char) =>
      char.replace(
        /[^ !#$%&',-/0123456789:;<=>ABCDEFGHIJKLMNOPQRSTUVWXYZ^_`abcdefghijklmnopqrstuvwxyz~]/,
        (match) =>
          `\\x{${match
            .charCodeAt(0)
            .toString(16)
            .toUpperCase()
            .padStart(2, "0")}}`
      )
    )
    .join("");
}
</script>
<style scoped>
.toolbar-alert {
  margin: 0px;
}
</style>
