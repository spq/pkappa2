<!-- eslint-disable vue/no-v-html -->
<template>
  <div v-if="viewmode === 'cards'">
    <v-expansion-panels v-model="openPanels" multiple variant="accordion">
      <v-expansion-panel
        v-for="(chunk, index) in data"
        :key="index"
        :value="index"
        static
        class="smol"
      >
        <v-expansion-panel-title
          class="border-bottom elevation-2"
          :class="[title_classes()]"
        >
          <v-row class="text-caption align-center" no-gutters>
            <v-col class="v-col-1">
              <v-icon v-if="chunk.Direction === 0" color="red"
                >mdi-arrow-right-thin-circle-outline</v-icon
              >
              <v-icon v-else color="green"
                >mdi-arrow-left-thin-circle-outline</v-icon
              >
              <span>
                {{ chunk.Direction === 0 ? "Client" : "Server" }}
              </span>
            </v-col>
            <v-col class="v-col-1" v-if="chunk.Time !== undefined">
              <v-tooltip location="bottom">
                <template #activator="{ props: tprops }">
                  <v-chip v-bind="tprops" size="small" variant="text"
                    >+{{
                      formatDateDifference(chunk.Time, data[index - 1]?.Time)
                    }}</v-chip
                  >
                </template>
                <span>{{ formatDate(chunk.Time) }}</span>
              </v-tooltip>
            </v-col>
            <v-col class="v-col-1">
              {{ formatChunkSize(chunk) }}
            </v-col>
            <v-col>
              <v-btn-toggle
                v-model="chunk.Presentation"
                mandatory
                density="compact"
                variant="text"
                color="primary"
                class="smol-group"
                @click.stop
              >
                <v-tooltip location="bottom">
                  <template #activator="{ props: pprops }">
                    <v-btn value="ascii" v-bind="pprops" size="x-small">
                      <v-icon>mdi-text-long</v-icon>
                    </v-btn>
                  </template>
                  <span>ASCII</span>
                </v-tooltip>
                <v-tooltip location="bottom">
                  <template #activator="{ props: pprops }">
                    <v-btn value="utf-8" v-bind="pprops" size="x-small">
                      <v-icon>mdi-format-font</v-icon>
                    </v-btn>
                  </template>
                  <span>UTF-8</span>
                </v-tooltip>
                <v-tooltip location="bottom">
                  <template #activator="{ props: pprops }">
                    <v-btn value="hexdump" v-bind="pprops" size="x-small">
                      <v-icon>mdi-format-columns</v-icon>
                    </v-btn>
                  </template>
                  <span>HEXDUMP</span>
                </v-tooltip>
                <v-tooltip location="bottom">
                  <template #activator="{ props: pprops }">
                    <v-btn value="raw" v-bind="pprops" size="x-small">
                      <v-icon>mdi-hexadecimal</v-icon>
                    </v-btn>
                  </template>
                  <span>RAW</span>
                </v-tooltip>
              </v-btn-toggle>
            </v-col>
            <v-col class="v-col-2">
              <v-tooltip location="bottom">
                <template #activator="{ props: pprops }">
                  <v-btn
                    v-bind="pprops"
                    size="x-small"
                    variant="text"
                    icon="mdi-chef-hat"
                    @click="openInCyberChef(chunk)"
                    @click.stop
                  >
                  </v-btn>
                </template>
                <span>Open in CyberChef</span>
              </v-tooltip>
              <v-tooltip location="bottom">
                <template #activator="{ props: pprops }">
                  <v-btn
                    v-bind="pprops"
                    size="x-small"
                    variant="text"
                    @click="downloadChunk(index, chunk)"
                    @click.stop
                  >
                    <v-icon>mdi-download</v-icon>
                  </v-btn>
                </template>
                <span>Download</span>
              </v-tooltip>
              <v-tooltip location="bottom">
                <template #activator="{ props: pprops }">
                  <v-btn
                    v-bind="pprops"
                    size="x-small"
                    variant="text"
                    icon="mdi-content-copy"
                    @click="copyToClipboard(chunk)"
                    @click.stop
                  >
                  </v-btn>
                </template>
                <span>Copy Content</span>
              </v-tooltip>
            </v-col>
          </v-row>
        </v-expansion-panel-title>
        <v-expansion-panel-text>
          <template v-if="chunk.Presentation === 'ascii'">
            <span
              class="chunk"
              :data-chunk-idx="index"
              :class="[classes(chunk)]"
              v-html="inlineAscii(chunk)"
            >
            </span>
          </template>
          <template v-else-if="chunk.Presentation === 'utf-8'">
            <span
              class="chunk"
              :data-chunk-idx="index"
              :class="[classes(chunk)]"
              v-html="inlineUnicode(chunk)"
            >
            </span>
          </template>
          <template v-else-if="chunk.Presentation === 'hexdump'">
            <pre :class="[classes(chunk), 'hexdump']">{{
              hexdump(chunk.Content)
            }}</pre>
          </template>
          <template v-else>
            <span :class="[classes(chunk)]"
              >{{ inlineHex(chunk.Content) }}<br
            /></span>
          </template>
        </v-expansion-panel-text>
      </v-expansion-panel>
    </v-expansion-panels>
  </div>
  <div v-else>
    <v-card>
      <v-card-text>
        <template v-if="presentation === 'ascii'">
          <span
            v-for="(chunk, index) in data"
            :key="index"
            class="chunk"
            :data-chunk-idx="index"
            :class="[classes(chunk)]"
            v-html="inlineAscii(chunk)"
          >
          </span>
        </template>
        <template v-else-if="presentation === 'utf-8'">
          <span
            v-for="(chunk, index) in data"
            :key="index"
            class="chunk"
            :data-chunk-idx="index"
            :class="[classes(chunk)]"
            v-html="inlineUnicode(chunk)"
          >
          </span>
        </template>
        <template v-else-if="presentation === 'hexdump'">
          <pre
            v-for="(chunk, index) in data"
            :key="index"
            :class="[classes(chunk), 'hexdump']"
            >{{ hexdump(chunk.Content) }}</pre
          >
        </template>
        <template v-else>
          <span
            v-for="(chunk, index) in data"
            :key="index"
            :class="[classes(chunk)]"
            >{{ inlineHex(chunk.Content) }}<br
          /></span>
        </template>
      </v-card-text>
    </v-card>
  </div>
</template>

<script lang="ts" setup>
import { Data, DataRegexes } from "@/apiClient";
import { PropType, computed, ref, watch } from "vue";
import { formatDate } from "@/filters";
import {
  escapeRegex,
  handleUnicodeDecode,
  tryURLDecodeIfEnabled,
} from "@/lib/utils";
import moment from "moment";
import prettyBytes from "pretty-bytes";
import { getColorScheme } from "@/lib/darkmode";
import { useStreamStore } from "@/stores/stream";
import { EventBus } from "./EventBus";
import { CYBERCHEF_URL } from "@/lib/constants";

const stream = useStreamStore();
const props = defineProps({
  viewmode: {
    type: String,
    required: true,
  },
  presentation: {
    type: String,
    required: true,
  },
  data: {
    type: Array as PropType<Data[]>,
    required: true,
  },
  highlightMatches: {
    type: Object as PropType<DataRegexes>,
    required: false,
    default: () => ({ Client: null, Server: null }),
  },
  urlDecode: {
    type: Boolean,
    required: false,
    default: false,
  },
});

const formatDateDifference = (first: string, second: string | undefined) => {
  if (second === undefined) return "0 ms";
  if (first === second) return "0 ms";
  const ms = moment(first).diff(moment(second));
  if (ms < 1000) return `${ms} ms`;
  const seconds = ms / 1000;
  if (seconds < 60) {
    return `${seconds} s`;
  }
  const minutes = Math.floor(seconds / 60);
  return `${minutes}:${seconds} s`;
};

const formatChunkSize = (chunk: Data) => {
  const size = atob(chunk.Content).length;
  return prettyBytes(size, {
    maximumFractionDigits: 1,
    binary: true,
  });
};

type VisualChunk = Data & {
  Presentation: string;
};

const data = ref(
  props.data.map((chunk) => {
    const visualChunk: VisualChunk = {
      ...chunk,
      Presentation: props.presentation,
    };
    return visualChunk;
  }),
);
const openPanels = ref<number[]>(data.value.map((_, index) => index));

const title_classes = () => {
  const colorScheme = getColorScheme();
  return {
    "bg-grey-lighten-3": colorScheme === "light",
    "bg-grey-darken-3": colorScheme === "dark",
  };
};

watch(
  () => props.presentation,
  (newPresentation) => {
    for (const chunk of data.value) {
      chunk.Presentation = newPresentation;
    }
  },
  { immediate: true },
);

const highlightRegex = (highlight: string[] | null) =>
  highlight?.map((regex) => {
    try {
      if (regex === "") return undefined;
      // replace \x{XX} with the actual character
      regex = regex.replace(/\\x{([0-9a-fA-F]{2})}/g, (_, hex) => {
        const decoded = String.fromCharCode(parseInt(hex as string, 16));
        if (escapeRegex.test(decoded)) return `\\${decoded}`;
        return decoded;
      });
      return new RegExp(regex, "g");
    } catch {
      console.error(`Invalid regex: ${regex}`);
    }
  });

const highlightMatchesClient = computed(() =>
  highlightRegex(props.highlightMatches.Client),
);
const highlightMatchesServer = computed(() =>
  highlightRegex(props.highlightMatches.Server),
);

const asciiMap = Array.from({ length: 0x100 }, (_, i) => {
  if (i != 0x0d && i != 0x0a && (i < 0x20 || i > 0x7e)) return ".";
  return `&#x${i.toString(16).padStart(2, "0")};`;
});

const handleHighlightMatches = (
  direction: number,
  chunkData: string,
  asciiEscaped: string[],
) => {
  const highlightMatchesRegex =
    direction === 0
      ? highlightMatchesClient.value
      : highlightMatchesServer.value;
  if (highlightMatchesRegex !== undefined) {
    const highlights: number[][] = [];
    for (const regex of highlightMatchesRegex) {
      if (regex === undefined) continue;
      for (const match of chunkData.matchAll(regex)) {
        highlights.push([match.index, match[0].length]);
      }
    }
    highlights.sort((a, b) => a[0] - b[0] || a[1] - b[1]);
    let highlightIndex = 0;
    for (const [index, length] of highlights) {
      asciiEscaped[index] =
        `<span class="mark" data-offset="${index}">${asciiEscaped[index]}`;
      if (highlightIndex > 0) {
        asciiEscaped[index] = `</span>${asciiEscaped[index]}`;
      }
      asciiEscaped[index + length - 1] =
        `${asciiEscaped[index + length - 1]}</span><span data-offset="${index + length}">`;
      highlightIndex++;
    }
    if (highlightIndex > 0) {
      asciiEscaped[asciiEscaped.length - 1] =
        `${asciiEscaped[asciiEscaped.length - 1]}</span>`;
    }
  }

  return asciiEscaped.join("");
};

const inlineUnicode = (chunk: Data) => {
  const chunkData = handleUnicodeDecode(chunk, props.urlDecode);
  const asciiEscaped = chunkData.split("").map((c) => {
    const charCode = c.charCodeAt(0);
    return asciiMap[charCode] !== undefined ? asciiMap[charCode] : c;
  });
  return handleHighlightMatches(chunk.Direction, chunkData, asciiEscaped);
};

const inlineAscii = (chunk: Data) => {
  const chunkData = tryURLDecodeIfEnabled(atob(chunk.Content), props.urlDecode);
  const asciiEscaped = chunkData
    .split("")
    .map((c) => asciiMap[c.charCodeAt(0)]);
  return handleHighlightMatches(chunk.Direction, chunkData, asciiEscaped);
};

const classes = (chunk: Data) => ({
  chunk: true,
  client: chunk.Direction === 0,
  server: chunk.Direction === 1,
});

const inlineHex = (b64: string) => {
  const ui8 = Uint8Array.from(
    atob(b64)
      .split("")
      .map((char) => char.charCodeAt(0)),
  );
  const str = ([] as number[]).slice
    .call(ui8)
    .map((i) => i.toString(16).padStart(2, "0"))
    .join("");
  return str;
};

const hexdump = (b64: string) => {
  const ui8 = Uint8Array.from(
    atob(b64)
      .split("")
      .map((char) => char.charCodeAt(0)),
  );
  const str = ([] as number[]).slice
    .call(ui8)
    .map((i) => i.toString(16).padStart(2, "0"))
    .join("")
    .match(/.{1,2}/g)
    ?.join(" ")
    .match(/.{1,48}/g)
    ?.map(function (str) {
      while (str.length < 48) {
        str += " ";
      }
      let ascii =
        str
          .replace(/ /g, "")
          .match(/.{1,2}/g)
          ?.map(function (ch) {
            let c = String.fromCharCode(parseInt(ch, 16));
            if (!/[ -~]/.test(c)) {
              c = ".";
            }
            return c;
          })
          .join("") ?? "";
      while (ascii.length < 16) {
        ascii += " ";
      }
      return str + " |" + ascii + "|";
    })
    .join("\n");
  return str;
};

const downloadChunk = (index: number, chunk: Data) => {
  const blob = new Blob([atob(chunk.Content)], {
    type: "application/octet-stream",
  });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = `chunk-${stream.id}-${index}.bin`;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
};

const copyToClipboard = (chunk: VisualChunk) => {
  let content = "";
  switch (chunk.Presentation) {
    case "ascii":
      content = tryURLDecodeIfEnabled(atob(chunk.Content), props.urlDecode);
      break;
    case "utf-8":
      content = handleUnicodeDecode(chunk, props.urlDecode);
      break;
    case "hexdump":
      content = hexdump(chunk.Content) ?? "";
      break;
    case "raw":
      content = inlineHex(chunk.Content);
      break;
    default:
      EventBus.emit(
        "showError",
        `Unknown presentation format: ${chunk.Presentation}`,
      );
      return;
  }
  navigator.clipboard
    .writeText(content)
    .then(() => {
      EventBus.emit("showMessage", "Copied content to clipboard.");
    })
    .catch((err) => {
      EventBus.emit("showError", `Failed to copy to clipboard: ${err}`);
    });
};

function openInCyberChef(chunk: Data) {
  window.open(
    `${CYBERCHEF_URL}#input=${encodeURIComponent(chunk.Content)}`,
    "_blank",
    "noopener,noreferrer",
  );
}
</script>
<style scoped>
.chunk {
  white-space: break-spaces;
  font-family: monospace, monospace;
}
.server {
  color: #000080;
  background-color: #eeedfc;

  &.hexdump {
    margin-left: 2em;
  }
}
.server :deep(.mark) {
  background-color: #9090ff;
}
.client {
  color: #800000;
  background-color: #faeeed;
}
.client :deep(.mark) {
  background-color: #ff8e5e;
}

.v-theme--dark {
  .server {
    color: #ffffff;
    background-color: #261858;
  }

  .client {
    color: #ffffff;
    background-color: #561919;
  }
}

.smol-group {
  height: 24px !important;
}

.smol > button {
  padding-top: 0 !important;
  padding-bottom: 0 !important;
}
</style>
