import Vue from "vue";
import Vuetify from "vuetify";
import { createPinia, PiniaVuePlugin } from "pinia";
import App from "./App.vue";
import router from "./routes";
import VueApexCharts from "vue-apexcharts";
import VueFilterPrettyBytes from "vue-filter-pretty-bytes";
import * as VueMoment from "vue-moment";
import { tagForURI } from "./filters/tagForURI";
import { tagNameForURI } from "./filters/tagNameForURI";

Vue.config.productionTip = process.env.NODE_ENV == "production";

Vue.use(PiniaVuePlugin);
Vue.use(Vuetify);
Vue.use(VueApexCharts);
Vue.use(VueMoment);
Vue.use(VueFilterPrettyBytes);

Vue.component("Apexchart", VueApexCharts);

const pinia = createPinia();
const vue = new Vue({
  vuetify: new Vuetify(),
  router,
  render: (h) => h(App),
  pinia,
});

declare module "vue/types/vue" {
  interface Vue {
    capitalize: (value: string | null) => string;
    tagify: (id: string, what: "id" | "type" | "name") => string;
    formatDate: (time: string | null) => string;
    formatDateLong: (time: string | null) => string;
    tagForURI: (tagId: string) => string;
    tagNameForURI: (tagName: string) => string;
    regexEscape: (text: string) => string;
  }
}

Vue.filter("capitalize", function (value: string | null) {
  if (!value) return "";
  value = value.toString();
  return value.charAt(0).toUpperCase() + value.slice(1);
});
Vue.filter("tagify", function (id: string, what: "id" | "type" | "name") {
  const type = id.split("/", 1)[0];
  const name = id.substr(type.length + 1);
  return { id, type, name }[what];
});
Vue.filter("formatDate", function (time: string | null) {
  if (time === null) return null;
  const moment = vue.$moment(time).local();
  let format = "HH:mm:ss.SSS";
  if (!moment.isSame(vue.$moment(), "day")) format = `YYYY-MM-DD ${format}`;
  return moment.format(format);
});
Vue.filter("formatDateLong", function (time: string | null) {
  if (time === null) return null;
  const moment = vue.$moment(time).local();
  return moment.format("YYYY-MM-DD HH:mm:ss.SSS ZZ");
});
Vue.filter("tagForURI", tagForURI);
Vue.filter("tagNameForURI", tagNameForURI);
Vue.filter("regexEscape", function (text: string) {
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
});

vue.$mount("#app");
