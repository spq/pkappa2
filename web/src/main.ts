import { getColorScheme } from "@/lib/darkmode";
import Vue, { createApp, h } from "vue";
import { createVuetify } from "vuetify";
import { createPinia } from "pinia";
import App from "./App.vue";
import router from "./routes";
import VueApexCharts from "vue3-apexcharts";
// import VueFilterPrettyBytes from "vue-filter-pretty-bytes";


Vue.use(VueApexCharts);
// Vue.use(VueFilterPrettyBytes);

Vue.component("Apexchart", VueApexCharts);

const vuetify = createVuetify({ theme: { defaultTheme: getColorScheme() } });
const pinia = createPinia();
const vue = createApp({
  render: () => h(App),
});
vue.use(vuetify);
vue.use(pinia);
vue.use(router);

vue.mount("#app");

export default vue;
