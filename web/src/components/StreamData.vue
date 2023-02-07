<template>
  <v-card>
    <v-card-text>
      <template v-if="presentation == 'ascii'">
        <span
          v-for="(chunk, index) in data"
          :key="index"
          :data-chunk-idx="index"
          :style="
            chunk.Direction != 0
              ? 'font-family: monospace,monospace; color: #000080; background-color: #eeedfc;'
              : 'font-family: monospace,monospace; color: #800000; background-color: #faeeed;'
          "
        >
          <span
            v-for="({ str, offset }, index) in $options.filters.inlineAscii(
              chunk.Content
            )"
            :key="index"
            :data-offset="offset"
            v-html="str"
          >
          </span>
        </span>
      </template>
      <template v-else-if="presentation == 'hexdump'">
        <pre
          v-for="(chunk, index) in data"
          :key="index"
          :style="
            chunk.Direction != 0
              ? 'margin-left: 2em; color: #000080; background-color: #eeedfc;'
              : 'color: #800000; background-color: #faeeed;'
          "
          >{{ chunk.Content | hexdump }}</pre
        >
      </template>
      <template v-else>
        <span
          v-for="(chunk, index) in data"
          :key="index"
          :style="
            chunk.Direction != 0
              ? 'font-family: monospace,monospace; color: #000080; background-color: #eeedfc;'
              : 'font-family: monospace,monospace; color: #800000; background-color: #faeeed;'
          "
        >
          {{ chunk.Content | inlineHex }}<br
        /></span>
      </template>
    </v-card-text>
  </v-card>
</template>

<script>
const asciiMap = Array.from({ length: 0x100 }, (_, i) => {
  if (i == 0x0a) return "<br/>";
  if (i == 0x20) return "&nbsp;";
  if (i >= 0x21 && i <= 0x7e) return `&#x${i.toString(16).padStart("2", "0")};`;
  return ".";
});
export default {
  name: "StreamData",
  filters: {
    inlineAscii(b64) {
      return atob(b64)
        .split("")
        .flatMap((c, idx, arr) =>
          c == "\r" && idx + 1 < arr.length && arr[idx + 1] == "\n"
            ? []
            : [
                {
                  str: asciiMap[c.charCodeAt(0)],
                  offset: idx,
                },
              ]
        );
    },
    inlineHex(b64) {
      const ui8 = Uint8Array.from(
        atob(b64)
          .split("")
          .map((char) => char.charCodeAt(0))
      );
      var str = [].slice
        .call(ui8)
        .map((i) => i.toString(16).padStart("2", "0"))
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
        .map((i) => i.toString(16).padStart("2", "0"))
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
  props: ["presentation", "data"],
};
</script>
