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
          v-html="inlineAscii(chunk.Content)"
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
        >
          {{ inlineHex(chunk.Content) }}<br
        /></span>
      </template>
    </v-card-text>
  </v-card>
</template>

<script lang="ts" setup>
import { Data } from "@/apiClient";
import { PropType, computed } from "vue";

const props = defineProps({
  presentation: {
    type: String,
    required: true,
  },
  data: {
    type: Array as PropType<Data[]>,
    required: true,
  },
});
const presentation = computed(() => props.presentation);
const data = computed(() => props.data);

const asciiMap = Array.from({ length: 0x100 }, (_, i) => {
  if (i != 0x0d && i != 0x0a && (i < 0x20 || i > 0x7e)) return ".";
  return `&#x${i.toString(16).padStart(2, "0")};`;
});

const inlineAscii = (b64: string) => {
  return atob(b64)
    .split("")
    .map((c) => asciiMap[c.charCodeAt(0)])
    .join("");
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
      .map((char) => char.charCodeAt(0))
  );
  var str = ([] as number[]).slice
    .call(ui8)
    .map((i) => i.toString(16).padStart(2, "0"))
    .join("");
  return str;
};

const hexdump = (b64: string) => {
  const ui8 = Uint8Array.from(
    atob(b64)
      .split("")
      .map((char) => char.charCodeAt(0))
  );
  var str = ([] as number[]).slice
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
      var ascii =
        str
          .replace(/ /g, "")
          .match(/.{1,2}/g)
          ?.map(function (ch) {
            var c = String.fromCharCode(parseInt(ch, 16));
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
  @media (prefers-color-scheme: dark) {
    color: #ffffff;
    background-color: #261858;
  }
  &.hexdump {
    margin-left: 2em;
  }
}
.client {
  color: #800000;
  background-color: #faeeed;
  @media (prefers-color-scheme: dark) {
    color: #ffffff;
    background-color: #561919;
  }
}
</style>
