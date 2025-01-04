import { createApp } from "vue";
import { registerPlugins } from "@/plugins";
import App from "./App.vue";

// Vue.config.productionTip = process.env.NODE_ENV == "production";

const app = createApp(App);

registerPlugins(app);

app.mount("#app");
