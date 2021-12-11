<template>
  <v-card>
    <v-card-text>
        <div v-if="presentation == 'ascii'">
          <span
            v-for="(chunk, index) in data"
            :key="index"
            :style="
              chunk.Direction != 0
                ? 'font-family: monospace,monospace; color: #000080; background-color: #eeedfc;'
                : 'font-family: monospace,monospace; color: #800000; background-color: #faeeed;'
            "
            v-html="$options.filters.inlineAscii(chunk.Content)"
          />
        </div>
        <div v-else-if="presentation == 'hexdump'">
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
        </div>
        <div v-else>
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
        </div>
    </v-card-text>
  </v-card>
</template>

<script>
export default {
  name: "StreamData",
  props: ["presentation", "data"],
  filters: {
    inlineAscii(b64) {
      const ui8 = Uint8Array.from(
        atob(b64)
          .split("")
          .map((char) => char.charCodeAt(0))
      );
      var str = [].slice
        .call(ui8)
        .map(function (i, idx, arr) {
          if (i == 0x0d && idx + 1 < arr.length && arr[idx + 1] == 0x0a)
            return "";
          if (i == 0x0a) return "<br/>";
          if (/[ -~]/.test(String.fromCharCode(i))) {
            return "&#x" + ("00" + i.toString(16)).substr(-2) + ";";
          }
          return ".";
        })
        .join("");
      return str;
    },
    inlineHex(b64) {
      const ui8 = Uint8Array.from(
        atob(b64)
          .split("")
          .map((char) => char.charCodeAt(0))
      );
      var str = [].slice
        .call(ui8)
        .map(function (i) {
          var h = i.toString(16);
          if (h.length < 2) {
            h = "0" + h;
          }
          return h;
        })
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
        .map(function (i) {
          var h = i.toString(16);
          if (h.length < 2) {
            h = "0" + h;
          }
          return h;
        })
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
};
</script>