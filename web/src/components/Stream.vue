<template>
  <div>
    <ToolBar>
      <v-tooltip location="bottom">
        <template #activator="{ props }">
          <v-btn
            icon
            :to="{
              name: 'search',
              query: {
                q: $route.query.q,
                p: $route.query.p,
                converter: $route.query.converter,
              },
            }"
            v-bind="props"
          >
            <v-icon>mdi-arrow-left</v-icon>
          </v-btn>
        </template>
        <span>Back to Search Results</span>
      </v-tooltip>
      <v-tooltip location="bottom">
        <template #activator="{ props }">
          <v-btn icon v-bind="props" @click="fetchStreamForId()">
            <v-icon>mdi-refresh</v-icon>
          </v-btn>
        </template>
        <span>Refresh</span>
      </v-tooltip>
      <v-tooltip location="bottom">
        <template #activator="{ props }">
          <v-btn
            :disabled="selectionQuery == ''"
            exact
            icon
            :to="{
              name: 'search',
              query: {
                q: selectionQuery,
              },
            }"
            v-bind="props"
            ><v-icon>mdi-selection-search</v-icon></v-btn
          >
        </template>
        <span>Search Selection</span>
      </v-tooltip>
      <v-tooltip location="bottom">
        <template #activator="{ props }">
          <v-btn exact icon @click="openInCyberChef()" v-bind="props"
            ><v-icon>mdi-chef-hat</v-icon></v-btn
          >
        </template>
        <span>Open in CyberChef</span>
      </v-tooltip>
      <v-menu location="bottom left"
        ><template #activator="{ props: propsMenu }">
          <v-tooltip location="bottom">
            <template #activator="{ props: propsTooltip }">
              <v-btn v-bind="{ ...propsMenu, ...propsTooltip }" icon>
                <v-icon>mdi-checkbox-multiple-outline</v-icon>
              </v-btn>
            </template>
            <span>Marks</span>
          </v-tooltip>
        </template>
        <v-list v-if="stream.stream !== null" density="compact">
          <template v-for="tag of groupedTags.mark" :key="tag.Name">
            <v-list-item
              slim
              link
              @click="
                markStream(tag.Name, !stream.stream.Tags.includes(tag.Name))
              "
            >
              <template #prepend>
                <v-icon
                  >mdi-{{
                    stream.stream.Tags.includes(tag.Name)
                      ? "checkbox-outline"
                      : "checkbox-blank-outline"
                  }}</v-icon
                >
              </template>
              <v-list-item-title>{{
                tagify(tag.Name, "name")
              }}</v-list-item-title>
            </v-list-item>
          </template>
          <v-divider />
          <v-list-item link @click="createMark">
            <v-list-item-action />
            <v-list-item-title>Create new</v-list-item-title>
          </v-list-item>
        </v-list>
      </v-menu>
      <v-tooltip location="bottom">
        <template #activator="{ props }">
          <v-btn icon :href="`/api/download/${streamId}.pcap`" v-bind="props"
            ><v-icon>mdi-download</v-icon></v-btn
          >
        </template>
        <span>Download PCAP</span>
      </v-tooltip>
      <v-btn-toggle
        v-model="presentation"
        mandatory
        density="compact"
        variant="text"
        color="primary"
      >
        <v-tooltip location="bottom">
          <template #activator="{ props }">
            <v-btn value="ascii" v-bind="props">
              <v-icon>mdi-text-long</v-icon>
            </v-btn>
          </template>
          <span>ASCII</span>
        </v-tooltip>
        <v-tooltip location="bottom">
          <template #activator="{ props }">
            <v-btn value="utf-8" v-bind="props">
              <v-icon>mdi-format-font</v-icon>
            </v-btn>
          </template>
          <span>UTF-8</span>
        </v-tooltip>
        <v-tooltip location="bottom">
          <template #activator="{ props }">
            <v-btn value="hexdump" v-bind="props">
              <v-icon>mdi-format-columns</v-icon>
            </v-btn>
          </template>
          <span>HEXDUMP</span>
        </v-tooltip>
        <v-tooltip location="bottom">
          <template #activator="{ props }">
            <v-btn value="raw" v-bind="props">
              <v-icon>mdi-hexadecimal</v-icon>
            </v-btn>
          </template>
          <span>RAW</span>
        </v-tooltip>
      </v-btn-toggle>
      <v-tooltip
        v-if="stream.stream !== null && selectableConverters.length > 1"
        location="bottom"
      >
        <template #activator="{ props }">
          <v-select
            hide-details
            density="compact"
            label="Converter"
            :items="selectableConverters"
            :model-value="activeConverter"
            v-bind="props"
            :style="{ maxWidth: 'fit-content', minWidth: '200px' }"
            @update:model-value="changeConverter"
          />
        </template>
        <span>Select converter view</span>
      </v-tooltip>

      <v-tooltip location="bottom">
        <template #activator="{ props }">
          <v-switch
            v-model="urlDecode"
            label="URL Decode"
            density="compact"
            hide-details
            color="primary"
            v-bind="props"
          />
        </template>
        <span>URL Decode</span>
      </v-tooltip>

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
        <v-tooltip location="bottom">
          <template #activator="{ props }">
            <v-btn
              variant="plain"
              icon="mdi-chevron-left"
              v-bind="props"
              :disabled="prevStreamId == null"
              :to="
                prevStreamId == null
                  ? {}
                  : {
                      name: 'stream',
                      query: {
                        q: $route.query.q,
                        p: $route.query.p,
                        converter: $route.query.converter,
                      },
                      params: { streamId: prevStreamId },
                    }
              "
            />
          </template>
          <span>Previous Stream</span>
        </v-tooltip>
        <v-tooltip location="bottom">
          <template #activator="{ props }">
            <v-btn
              variant="plain"
              icon="mdi-chevron-right"
              v-bind="props"
              :disabled="nextStreamId == null"
              :to="
                nextStreamId == null
                  ? {}
                  : {
                      name: 'stream',
                      query: {
                        q: $route.query.q,
                        p: $route.query.p,
                        converter: $route.query.converter,
                      },
                      params: { streamId: nextStreamId },
                    }
              "
            />
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
    <v-alert v-else-if="stream.error !== null" type="error" density="compact">{{
      stream.error
    }}</v-alert>
    <div v-else-if="stream.stream !== null">
      <v-container fluid>
        <v-row>
          <v-col cols="1" class="text-subtitle-2">Client:</v-col>
          <v-col cols="2" class="text-body-2"
            ><span
              :title="`${stream.stream.Stream.Client.Host}:${stream.stream.Stream.Client.Port} (${stream.stream.Stream.Client.Bytes} Bytes)`"
              >{{ stream.stream.Stream.Client.Host }}:{{
                stream.stream.Stream.Client.Port
              }}
              ({{
                prettyBytes(stream.stream.Stream.Client.Bytes, {
                  maximumFractionDigits: 1,
                  binary: true,
                })
              }})</span
            ></v-col
          >
          <v-col cols="1" class="text-subtitle-2">First Packet:</v-col>
          <v-col cols="3" class="text-body-2"
            ><span :title="formatDateLong(stream.stream.Stream.FirstPacket)">{{
              formatDate(stream.stream.Stream.FirstPacket)
            }}</span></v-col
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
              size="small"
              variant="flat"
              :color="tag.color"
              :style="{ color: getContrastTextColor(tag.color) }"
              >{{ tag.name }}</v-chip
            ></v-col
          >
        </v-row>
        <v-row>
          <v-col cols="1" class="text-subtitle-2">Server:</v-col>
          <v-col cols="2" class="text-body-2"
            ><span
              :title="`${stream.stream.Stream.Server.Host}:${stream.stream.Stream.Server.Port} (${stream.stream.Stream.Server.Bytes} Bytes)`"
              >{{ stream.stream.Stream.Server.Host }}:{{
                stream.stream.Stream.Server.Port
              }}
              ({{
                prettyBytes(stream.stream.Stream.Server.Bytes, {
                  maximumFractionDigits: 1,
                  binary: true,
                })
              }})</span
            ></v-col
          >
          <v-col cols="1" class="text-subtitle-2">Last Packet:</v-col>
          <v-col cols="3" class="text-body-2"
            ><span :title="formatDateLong(stream.stream.Stream.LastPacket)">{{
              formatDate(stream.stream.Stream.LastPacket)
            }}</span></v-col
          >
          <v-col cols="1" class="text-body-2"
            ><span v-if="streamTags.service.length == 0">{{
              stream.stream.Stream.Protocol
            }}</span
            ><span v-else
              ><v-chip
                v-for="service in streamTags.service"
                :key="`service/${service.name}`"
                size="small"
                variant="flat"
                :color="service.color"
                :style="{ color: getContrastTextColor(service.color) }"
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
              size="small"
              variant="flat"
              :color="mark.color"
              :style="{ color: getContrastTextColor(mark.color) }"
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
              size="small"
              variant="flat"
              :color="generated.color"
              :style="{ color: getContrastTextColor(generated.color) }"
              >{{ generated.name }}</v-chip
            ></v-col
          >
        </v-row>
      </v-container>
      <StreamData
        ref="streamData"
        :data="stream.stream.Data"
        :presentation="presentation"
        :highlight-matches="streams.result?.DataRegexes"
        :url-decode="urlDecode"
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
import { useRoute, useRouter } from "vue-router";
import {
  registerSelectionListener,
  destroySelectionListener,
} from "./streamSelector";
import { formatDate, formatDateLong, tagify } from "@/filters";
import { getContrastTextColor } from "@/lib/colors";
import prettyBytes from "pretty-bytes";

const CYBERCHEF_URL = "https://gchq.github.io/CyberChef/";

const store = useRootStore();
const route = useRoute();
const router = useRouter();
const presentation = ref("ascii");
const selectionData = ref("");
const selectionQuery = ref("");
const streamData = ref<HTMLElement | null>(null);
const urlDecode = ref(false);

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
  const res: { [key: string]: { name: string; color: string }[] } = {
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
  return parseInt(route.params.streamId as string, 10);
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
      title: converter.Name,
      value: "converter:" + converter.Name,
    })) ?? [];
  return [
    {
      title: "* none",
      value: "none",
    },
    ...stream.stream.Converters.map((converter) => ({
      title: `* ${converter}`,
      value: "converter:" + converter,
    })),
    ...availableConverters,
  ];
});

const streamIndex = computed(() => {
  if (streams.result == null) return null;
  const id = streamId.value;
  let i = 0;
  for (const r of streams.result.Results) {
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
    streamData: getCurrentInstance()!.proxy,
    presentation,
    urlDecode,
    selectionData,
    selectionQuery,
  };
  registerSelectionListener(proxy);

  const handle = (e: KeyboardEvent, streamId: number | null) => {
    if (streamId == null) return;
    e.preventDefault();
    void router.push({
      name: "stream",
      query: {
        q: route.query.q,
        p: route.query.p,
        converter: route.query.converter,
      },
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
    stream.fetchStream(streamId.value, converter.value).catch((err: Error) => {
      EventBus.emit("showError", `Failed to fetch stream: ${err.message}`);
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
    store.markTagAdd(tagId, [streamId.value]).catch((err: Error) => {
      EventBus.emit(
        "showError",
        `Failed to add stream to mark: ${err.message}`,
      );
    });
  } else {
    store.markTagDel(tagId, [streamId.value]).catch((err: Error) => {
      EventBus.emit(
        "showError",
        `Failed to remove stream from mark: ${err.message}`,
      );
    });
  }
}
</script>
