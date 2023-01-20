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
app.filter('tagify', function (id, what) {
  const type = id.split("/", 1)[0];
  const name = id.substr(type.length + 1);
  return { id, type, name }[what];
})
app.filter('formatDate', function (time) {
  if (time === null) return null;
  time = moment(time).local();
  let format = "HH:mm:ss.SSS";
  if (!time.isSame(moment(), "day")) format = `YYYY-MM-DD ${format}`;
  return time.format(format);
})
app.filter('formatDateLong', function (time) {
  if (time === null) return null;
  time = moment(time).local();
  return time.format('YYYY-MM-DD HH:mm:ss.SSS ZZ');
})
app.filter('tagForURI', function (tagId) {
  const type = tagId.split("/", 1)[0];
  const name = this.tagNameForURI(tagId.substr(type.length + 1));

  return `${type}:${name}`;
})
app.filter('tagNameForURI', function (tagName) {
  if (tagName.includes('"')) {
    tagName = tagName.replaceAll('"', '""');
  }
  if (/[ "]/.test(tagName)) {
    tagName = `"${tagName}"`;
  }

  return tagName;
})
app.filter('regexEscape', function (text) {
  return text
    .split("")
    .map(char => char.replace(
      /[^ !#$%&',-/0123456789:;<=>ABCDEFGHIJKLMNOPQRSTUVWXYZ^_`abcdefghijklmnopqrstuvwxyz~]/,
      (match) => `\\x{${match.charCodeAt(0).toString(16).toUpperCase().padStart('2', '0')}}`
    )
    )
    .join("");
})


vue.$mount('#app')
