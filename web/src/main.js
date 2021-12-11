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

Vue.filter('capitalize', function (value) {
  if (!value) return ''
  value = value.toString()
  return value.charAt(0).toUpperCase() + value.slice(1)
})
Vue.filter('tagify', function (id, what) {
  const type = id.split("/", 1)[0];
  const name = id.substr(type.length + 1);
  return { id, type, name }[what];
})

new Vue({
  vuetify: new Vuetify(),
  store,
  router,
  render: h => h(App)
}).$mount('#app')