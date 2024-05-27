import { getColorScheme, registerVuetifyTheme } from "@/lib/darkmode";
import Vue from "vue";
import Vuetify from "vuetify";
import { createPinia, PiniaVuePlugin } from "pinia";
import App from "./App.vue";
import router from "./routes";
import VueApexCharts from "vue-apexcharts";
import VueFilterPrettyBytes from "vue-filter-pretty-bytes";
import * as VueMoment from "vue-moment";

Vue.config.productionTip = process.env.NODE_ENV == "production";

Vue.use(PiniaVuePlugin);
Vue.use(Vuetify);
Vue.use(VueApexCharts);
Vue.use(VueMoment);
Vue.use(VueFilterPrettyBytes);

Vue.component("Apexchart", VueApexCharts);

const pinia = createPinia();
const vue = new Vue({
  vuetify: new Vuetify({ theme: { dark: getColorScheme() === "dark" } }),
  router,
  render: (h) => h(App),
  pinia,
});

registerVuetifyTheme(vue.$vuetify);

vue.$mount("#app");

export default vue;
