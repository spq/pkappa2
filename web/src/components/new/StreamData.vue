<template>
  <v-card>
    <v-card-text>
        <template v-if="presentation == 'ascii'">
          <span
            v-for="(chunk, index) in data"
            :data-chunk-idx="index"
            :key="index"
            :style="
              chunk.Direction != 0
                ? 'font-family: monospace,monospace; color: #000080; background-color: #eeedfc;'
                : 'font-family: monospace,monospace; color: #800000; background-color: #faeeed;'
            "
          >
            <span 
              v-for="({str, offset}, index) in $options.filters.inlineAscii(chunk.Content)"
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
      return [].slice
        .call(ui8)
        .map(function (i, idx, arr) {
          const result = {
            str: '.',
            offset: idx,
          };
          if (i == 0x0d && idx + 1 < arr.length && arr[idx + 1] == 0x0a) {
            result.str = '';
          } else if (i == 0x0a) {
            result.str = "<br/>";
          } else if (/[ -~]/.test(String.fromCharCode(i))) {
            result.str = "&#x" + ("00" + i.toString(16)).substr(-2) + ";";
          }

          return result;
        })
        .filter(obj => obj.str !== '');
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
