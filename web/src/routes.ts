import { createRouter, createWebHashHistory } from "vue-router";

import Base from "./components/Base.vue";
import Converters from "./components/Converters.vue";
import PcapOverIP from "./components/PcapOverIP.vue";
import Webhooks from "./components/Webhooks.vue";
import Home from "./components/Home.vue";
import Status from "./components/Status.vue";
import Pcaps from "./components/Pcaps.vue";
import Settings from "./components/Settings.vue";
import Tags from "./components/Tags.vue";
import Graph from "./components/Graph.vue";
import ResultsLayout from "./components/ResultsLayout.vue";
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
          path: "settings",
          name: "settings",
          component: Settings,
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
          path: "webhooks",
          name: "webhooks",
          component: Webhooks,
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
          component: ResultsLayout,
          props: (route) => ({
            searchTerm: route.query.q,
            searchPage: route.query.p,
          }),
          children: [
            {
              path: "/stream/:streamId(\\d+)",
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
    },
  ],
});
