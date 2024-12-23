<!-- eslint-disable vue/no-v-html -->
<template>
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
</template>

<script lang="ts" setup>
import { Data, DataRegexes } from "@/apiClient";
import { PropType, computed } from "vue";
import { escapeRegex } from "./streamSelector";

const props = defineProps({
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
});
const presentation = computed(() => props.presentation);
const data = computed(() => props.data);
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

const inlineAscii = (chunk: Data) => {
  const chunkData = atob(chunk.Content);
  const asciiEscaped = chunkData
    .split("")
    .map((c) => asciiMap[c.charCodeAt(0)]);
  const highlightMatches =
    chunk.Direction === 0
      ? highlightMatchesClient.value
      : highlightMatchesServer.value;
  if (highlightMatches !== undefined) {
    const highlights: number[][] = [];
    for (const regex of highlightMatches) {
      if (regex === undefined) continue;
      chunkData.matchAll(regex)?.forEach((match) => {
        highlights.push([match.index, match[0].length]);
      });
    }
    highlights.sort((a, b) => a[0] - b[0] || a[1] - b[1]);
    let highlightIndex = 0;
    for (const [index, length] of highlights) {
      if (highlightIndex > 0) {
        asciiEscaped[index] = `</span>${asciiEscaped[index]}`;
      }
      asciiEscaped[index] =
        `<span class="mark" data-offset="${index}">${asciiEscaped[index]}`;
      asciiEscaped[index + length - 1] =
        `${asciiEscaped[index + length - 1]}</span>`;
      if (highlightIndex < highlights.length - 1) {
        asciiEscaped[index + length - 1] =
          `${asciiEscaped[index + length - 1]}<span data-offset="${index + length}">`;
      }
      highlightIndex++;
    }
  }

  return asciiEscaped.join("");
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
.server >>> .mark {
  background-color: #9090ff;
}
.client {
  color: #800000;
  background-color: #faeeed;
}
.client >>> .mark {
  background-color: #ff8e5e;
}

.theme--dark {
  .server {
    color: #ffffff;
    background-color: #261858;
  }

  .client {
    color: #ffffff;
    background-color: #561919;
  }
}
</style>
