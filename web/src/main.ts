import { createApp, h } from "vue";
import { registerPlugins } from '@/plugins'
import App from "./App.vue";

// Vue.config.productionTip = process.env.NODE_ENV == "production";

const app = createApp({
  MODE: 3,
  render: () => h(App),
});

registerPlugins(app);

app.mount("#app");
