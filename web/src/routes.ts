import { createRouter, createWebHashHistory } from "vue-router";

import Base from "./components/Base.vue";
import Converters from "./components/Converters.vue";
import PcapOverIP from "./components/PcapOverIP.vue";
import Home from "./components/Home.vue";
import Status from "./components/Status.vue";
import Pcaps from "./components/Pcaps.vue";
import Tags from "./components/Tags.vue";
import Graph from "./components/Graph.vue";
import Results from "./components/Results.vue";
import Stream from "./components/Stream.vue";

export default createRouter({
  history: createWebHashHistory(),
  routes: [
    {
      path: "/",
      component: Base,
      children: [
        {
          path: "",
          name: "home",
          component: Home,
        },
        {
          path: "status",
          name: "status",
          component: Status,
        },
        {
          path: "pcaps",
          name: "pcaps",
          component: Pcaps,
        },
        {
          path: "tags",
          name: "tags",
          component: Tags,
        },
        {
          path: "converters",
          name: "converters",
          component: Converters,
        },
        {
          path: "pcap-over-ip",
          name: "pcap-over-ip",
          component: PcapOverIP,
        },
        {
          path: "graph",
          name: "graph",
          component: Graph,
          props: (route) => ({ searchTerm: route.query.q }),
        },
        {
          path: "search",
          name: "search",
          component: Results,
          props: (route) => ({
            searchTerm: route.query.q,
            searchPage: route.query.p,
          }),
        },
        {
          path: "stream/:streamId(\\d+)",
          name: "stream",
          component: Stream,
          props: (route) => ({
            searchTerm: route.query.q,
            searchPage: route.query.p,
          }),
        },
      ],
    },
  ],
});
