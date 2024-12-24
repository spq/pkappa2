import { getColorScheme } from "@/lib/darkmode";
import { createApp, h } from "vue";
import { createVuetify } from "vuetify";
import { createPinia } from "pinia";
import App from "./App.vue";
import router from "./routes";
import VueApexCharts from "vue3-apexcharts";

// Vue.config.productionTip = process.env.NODE_ENV == "production";

const pinia = createPinia();
const app = createApp({
  MODE: 3,
  render: () => h(App),
});
const vuetify = createVuetify({ theme: { defaultTheme: getColorScheme() } });

app.use(vuetify);
app.use(router);
app.use(pinia);
app.use(VueApexCharts);

app.mount("#app");

export default app;
