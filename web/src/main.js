import Vue from 'vue';
import Vuetify from 'vuetify';
import App from './App.vue'
import store from './store'
import router from './routes'
import VueApexCharts from 'vue-apexcharts'

Vue.config.productionTip = process.env.NODE_ENV == 'production';

Vue.use(Vuetify);
Vue.use(VueApexCharts)

Vue.component('apexchart', VueApexCharts)

new Vue({
  vuetify: new Vuetify(),
  store,
  router,
  render: h => h(App)
}).$mount('#app')