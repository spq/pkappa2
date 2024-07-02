<template>
  <div>
    <ToolBar>
      <v-tooltip bottom>
        <template #activator="{ on, attrs }">
          <v-btn
            v-bind="attrs"
            icon
            :to="{
              name: 'search',
              query: { q: $route.query.q, p: $route.query.p },
            }"
            v-on="on"
          >
            <v-icon>mdi-arrow-left</v-icon>
          </v-btn>
        </template>
        <span>Back to Search Results</span>
      </v-tooltip>
      <v-tooltip bottom>
        <template #activator="{ on, attrs }">
          <v-btn v-bind="attrs" icon v-on="on" @click="fetchStreamForId()">
            <v-icon>mdi-refresh</v-icon>
          </v-btn>
        </template>
        <span>Refresh</span>
      </v-tooltip>
      <v-tooltip bottom>
        <template #activator="{ on, attrs }">
          <v-btn
            :disabled="selectionQuery == ''"
            link
            exact
            v-bind="attrs"
            icon
            :to="{
              name: 'search',
              query: {
                q: selectionQuery,
              },
            }"
            v-on="on"
            ><v-icon>mdi-selection-search</v-icon></v-btn
          >
        </template>
        <span>Search Selection</span>
      </v-tooltip>
      <v-tooltip bottom>
        <template #activator="{ on, attrs }">
          <v-btn
            link
            exact
            v-bind="attrs"
            icon
            @click="openInCyberChef()"
            v-on="on"
            ><v-icon>mdi-chef-hat</v-icon></v-btn
          >
        </template>
        <span>Open in CyberChef</span>
      </v-tooltip>
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
        <v-list v-if="stream.stream !== null" dense>
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
                  tagify(tag.Name, "name")
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
            icon
            :href="`/api/download/${streamId}.pcap`"
            v-on="on"
            ><v-icon>mdi-download</v-icon></v-btn
          >
        </template>
        <span>Download PCAP</span>
      </v-tooltip>
      <v-btn-toggle v-model="presentation" mandatory dense borderless>
        <v-tooltip bottom>
          <template #activator="{ on, attrs }">
            <v-btn v-bind="attrs" value="ascii" v-on="on">
              <v-icon>mdi-text-long</v-icon>
            </v-btn>
          </template>
          <span>ASCII</span>
        </v-tooltip>
        <v-tooltip bottom>
          <template #activator="{ on, attrs }">
            <v-btn v-bind="attrs" value="hexdump" v-on="on">
              <v-icon>mdi-format-columns</v-icon>
            </v-btn>
          </template>
          <span>HEXDUMP</span>
        </v-tooltip>
        <v-tooltip bottom>
          <template #activator="{ on, attrs }">
            <v-btn v-bind="attrs" value="raw" v-on="on">
              <v-icon>mdi-hexadecimal</v-icon>
            </v-btn>
          </template>
          <span>RAW</span>
        </v-tooltip>
      </v-btn-toggle>
      <v-row>
        <v-col cols="6">
          <v-tooltip
            v-if="stream.stream !== null && selectableConverters.length > 1"
            bottom
          >
            <template #activator="{ on, attrs }">
              <v-select
                v-bind="attrs"
                :items="selectableConverters"
                :value="activeConverter"
                v-on="on"
                @change="changeConverter"
              />
            </template>
            <span>Select converter view</span>
          </v-tooltip>
        </v-col>
      </v-row>
      <v-spacer />
      <div v-if="streamIndex !== null && streams.result !== null">
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
                ref="prevStream"
                icon
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
                ref="nextStream"
                icon
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
      v-if="
        stream.running ||
        (stream.stream === null && stream.error === null) ||
        null === tags
      "
      type="table-thead, table-tbody"
    ></v-skeleton-loader>
    <v-alert v-else-if="stream.error !== null" type="error" dense>{{
      stream.error
    }}</v-alert>
    <div v-else-if="stream.stream !== null">
      <v-container fluid>
        <v-row>
          <v-col cols="1" class="text-subtitle-2">Client:</v-col>
          <v-col
            cols="2"
            class="text-body-2"
            :title="`${stream.stream.Stream.Client.Host}:${stream.stream.Stream.Client.Port} (${stream.stream.Stream.Client.Bytes} Bytes)`"
            >{{ stream.stream.Stream.Client.Host }}:{{
              stream.stream.Stream.Client.Port
            }}
            ({{
              $options.filters?.prettyBytes(
                stream.stream.Stream.Client.Bytes,
                1,
                true
              )
            }})</v-col
          >
          <v-col cols="1" class="text-subtitle-2">First Packet:</v-col>
          <v-col
            cols="3"
            class="text-body-2"
            :title="formatDateLong(stream.stream.Stream.FirstPacket)"
            >{{ formatDate(stream.stream.Stream.FirstPacket) }}</v-col
          >
          <v-col cols="1" class="text-subtitle-2"
            >{{
              streamTags.service.length == 0 ? "Protocol" : "Service"
            }}:</v-col
          >
          <v-col cols="1" class="text-subtitle-2">Tags:</v-col>
          <v-col cols="3" class="text-body-2"
            ><v-chip
              v-for="tag in streamTags.tag"
              :key="`tag/${tag.name}`"
              small
              :color="tag.color"
              >{{ tag.name }}</v-chip
            ></v-col
          >
        </v-row>
        <v-row>
          <v-col cols="1" class="text-subtitle-2">Server:</v-col>
          <v-col
            cols="2"
            class="text-body-2"
            :title="`${stream.stream.Stream.Server.Host}:${stream.stream.Stream.Server.Port} (${stream.stream.Stream.Server.Bytes} Bytes)`"
            >{{ stream.stream.Stream.Server.Host }}:{{
              stream.stream.Stream.Server.Port
            }}
            ({{
              $options.filters?.prettyBytes(
                stream.stream.Stream.Server.Bytes,
                1,
                true
              )
            }})</v-col
          >
          <v-col cols="1" class="text-subtitle-2">Last Packet:</v-col>
          <v-col
            cols="3"
            class="text-body-2"
            :title="formatDateLong(stream.stream.Stream.LastPacket)"
            >{{ formatDate(stream.stream.Stream.LastPacket) }}</v-col
          >
          <v-col cols="1" class="text-body-2"
            ><span v-if="streamTags.service.length == 0">{{
              stream.stream.Stream.Protocol
            }}</span
            ><span v-else
              ><v-chip
                v-for="service in streamTags.service"
                :key="`service/${service.name}`"
                small
                :color="service.color"
                >{{ service.name }}</v-chip
              >
              ({{ stream.stream.Stream.Protocol }})</span
            ></v-col
          >
          <v-col cols="1" class="text-subtitle-2">Marks:</v-col>
          <v-col cols="3" class="text-body-2"
            ><v-chip
              v-for="mark in streamTags.mark"
              :key="`mark/${mark.name}`"
              small
              :color="mark.color"
              >{{ mark.name }}</v-chip
            ></v-col
          >
        </v-row>
        <v-row v-if="streamTags.generated.length > 0">
          <v-col cols="1" class="text-subtitle-2">Generated:</v-col>
          <v-col cols="11" class="text-body-2"
            ><v-chip
              v-for="generated in streamTags.generated"
              :key="`generated/${generated.name}`"
              small
              :color="generated.color"
              >{{ generated.name }}</v-chip
            ></v-col
          >
        </v-row>
      </v-container>
      <StreamData
        ref="streamData"
        :data="stream.stream.Data"
        :presentation="presentation"
      ></StreamData>
    </div>
  </div>
</template>

<script lang="ts" setup>
import { EventBus } from "./EventBus";
import { useRootStore } from "@/stores";
import { useStreamStore } from "@/stores/stream";
import { useStreamsStore } from "@/stores/streams";
import {
  computed,
  getCurrentInstance,
  ref,
  onBeforeUnmount,
  onMounted,
  watch,
} from "vue";
import { useRoute, useRouter } from "vue-router/composables";
import StreamData from "./StreamData.vue";
import ToolBar from "./ToolBar.vue";
import {
  registerSelectionListener,
  destroySelectionListener,
} from "./streamSelector";
import { formatDate, formatDateLong, tagify } from "@/filters";

const CYBERCHEF_URL = "https://gchq.github.io/CyberChef/";

const store = useRootStore();
const route = useRoute();
const router = useRouter();
const presentation = ref("ascii");
const selectionData = ref("");
const selectionQuery = ref("");
const streamData = ref<HTMLElement | null>(null);

if (localStorage.streamPresentation) {
  presentation.value = localStorage.getItem("streamPresentation") ?? "ascii";
}

const stream = useStreamStore();
const streams = useStreamsStore();
const tags = computed(() => store.tags);
const converters = computed(() => store.converters);
const groupedTags = computed(() => store.groupedTags);
const streamTags = computed(() => {
  if (stream.stream == null) return {};
  let res: { [key: string]: { name: string; color: string }[] } = {
    service: [],
    tag: [],
    mark: [],
    generated: [],
  };
  for (const tag of stream.stream.Tags) {
    const type = tag.split("/", 1)[0];
    const name = tag.substr(type.length + 1);
    const color = tags.value?.filter((e) => e.Name == tag)[0]?.Color ?? "";
    res[type].push({ name, color });
  }
  return res;
});

const streamId = computed(() => {
  return parseInt(route.params.streamId, 10);
});

const converter = computed(() => {
  return (route.query.converter as string) ?? "auto";
});

const activeConverter = computed(() => {
  if (stream.stream === null || stream.stream.ActiveConverter === "") {
    return "none";
  }
  return "converter:" + stream.stream.ActiveConverter;
});

const selectableConverters = computed(() => {
  if (stream.stream === null) return [];
  const availableConverters =
    converters.value?.map((converter) => ({
      text: converter.Name,
      value: "converter:" + converter.Name,
    })) ?? [];
  return [
    {
      text: "* none",
      value: "none",
    },
    ...stream.stream.Converters.map((converter) => ({
      text: `* ${converter}`,
      value: "converter:" + converter,
    })),
    ...availableConverters,
  ];
});

const streamIndex = computed(() => {
  if (streams.result == null) return null;
  const id = streamId.value;
  let i = 0;
  for (let r of streams.result.Results) {
    if (r.Stream.ID == id) return i;
    i++;
  }
  return null;
});

const nextStreamId = computed(() => {
  if (streams.result === null) return null;
  const index = streamIndex.value;
  if (index === null) return null;
  if (index + 1 >= streams.result.Results.length) return null;
  return streams.result.Results[index + 1].Stream.ID;
});

const prevStreamId = computed(() => {
  if (streams.result === null) return null;
  const index = streamIndex.value;
  if (index === null || index === 0) return null;
  return streams.result.Results[index - 1].Stream.ID;
});

watch(route, fetchStreamForId);
watch(presentation, (v) => {
  localStorage.streamPresentation = v;
  document.getSelection()?.empty();
});

onMounted(() => {
  fetchStreamForId();
  const proxy = {
    proxy: getCurrentInstance()!.proxy,
    selectionData,
    selectionQuery,
  };
  registerSelectionListener(proxy);

  const handle = (e: KeyboardEvent, streamId: number | null) => {
    if (streamId == null) return;
    e.preventDefault();
    void router.push({
      name: "stream",
      query: { q: route.query.q, p: route.query.p },
      params: { streamId: streamId.toString() },
    });
  };
  const handlers: { [key: string]: (e: KeyboardEvent) => void } = {
    j: (e: KeyboardEvent) => {
      handle(e, prevStreamId.value);
    },
    k: (e: KeyboardEvent) => {
      handle(e, nextStreamId.value);
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
    destroySelectionListener();
  });
});

function changeConverter(converter: string) {
  void router.push({
    query: { converter, q: route.query.q, p: route.query.p },
  });
}

function fetchStreamForId() {
  if (streamId.value !== null) {
    stream.fetchStream(streamId.value, converter.value).catch((err: string) => {
      EventBus.emit("showError", `Failed to fetch stream: ${err}`);
    });
    document.getSelection()?.empty();
  }
}

function openInCyberChef() {
  let data = selectionData.value;
  if (data === "" && stream.stream !== null) {
    for (const chunk of stream.stream.Data) {
      data += atob(chunk.Content);
    }
  }
  const encoded_data = btoa(data);
  window.open(`${CYBERCHEF_URL}#input=${encodeURIComponent(encoded_data)}`);
}

function createMark() {
  EventBus.emit("showCreateTagDialog", "mark", "", [streamId.value]);
}

function markStream(tagId: string, value: boolean) {
  if (value) {
    store.markTagAdd(tagId, [streamId.value]).catch((err: string) => {
      EventBus.emit("showError", `Failed to add stream to mark: ${err}`);
    });
  } else {
    store.markTagDel(tagId, [streamId.value]).catch((err: string) => {
      EventBus.emit("showError", `Failed to remove stream from mark: ${err}`);
    });
  }
}
</script>
