import Vue from "vue";
import Vuetify from "vuetify";
import App from "./App.vue";
import store from "./store";
import router from "./routes";
import VueApexCharts from "vue-apexcharts";
import vueFilterPrettyBytes from "vue-filter-pretty-bytes";

Vue.config.productionTip = process.env.NODE_ENV == "production";

Vue.use(Vuetify);
Vue.use(VueApexCharts);
Vue.use(require("vue-moment"));
Vue.use(vueFilterPrettyBytes);

Vue.component("Apexchart", VueApexCharts);

const vue = new Vue({
  vuetify: new Vuetify(),
  store,
  router,
  render: (h) => h(App),
});

Vue.filter("capitalize", function (value) {
  if (!value) return "";
  value = value.toString();
  return value.charAt(0).toUpperCase() + value.slice(1);
});
Vue.filter("tagify", function (id, what) {
  const type = id.split("/", 1)[0];
  const name = id.substr(type.length + 1);
  return { id, type, name }[what];
});
Vue.filter("formatDate", function (time) {
  if (time === null) return null;
  time = vue.$moment(time).local();
  let format = "HH:mm:ss.SSS";
  if (!time.isSame(vue.$moment(), "day")) format = `YYYY-MM-DD ${format}`;
  return time.format(format);
});
Vue.filter("formatDateLong", function (time) {
  if (time === null) return null;
  time = vue.$moment(time).local();
  return time.format("YYYY-MM-DD HH:mm:ss.SSS ZZ");
});
Vue.filter("tagForURI", function (tagId) {
  const type = tagId.split("/", 1)[0];
  const name = this.tagNameForURI(tagId.substr(type.length + 1));

  return `${type}:${name}`;
});
Vue.filter("tagNameForURI", function (tagName) {
  if (tagName.includes('"')) {
    tagName = tagName.replaceAll('"', '""');
  }
  if (/[ "]/.test(tagName)) {
    tagName = `"${tagName}"`;
  }

  return tagName;
});
Vue.filter("regexEscape", function (text) {
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
            .padStart("2", "0")}}`
      )
    )
    .join("");
});

vue.$mount("#app");
