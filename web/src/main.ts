// @ts-ignore-file until all dependencies and filters are migrated
import App from './App.vue';
import store from './store';
import router from './routes';
import VueApexCharts from 'vue3-apexcharts';
import {createVuetify} from "vuetify";
import {createApp} from "vue";
import moment from "moment";

const app = createApp(App);

app.use(createVuetify());
app.use(VueApexCharts);
app.use(store);
app.use(router);

app.component('apexchart', VueApexCharts);








app.mount('#app');
