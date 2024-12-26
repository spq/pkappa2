// Plugins
import vuetify from "./vuetify";
import pinia from "../stores";
import router from "../routes";
import VueApexCharts from "vue3-apexcharts";

// Types
import type { App } from "vue";

export function registerPlugins(app: App) {
  app.use(vuetify).use(router).use(pinia).use(VueApexCharts);
}
