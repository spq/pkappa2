import Vue from "vue";
import VueRouter from "vue-router";

import Base from "./components/Base";
import Converters from "./components/Converters";
import Home from "./components/Home";
import Status from "./components/Status";
import Pcaps from "./components/Pcaps";
import Tags from "./components/Tags";
import Graph from "./components/Graph";
import Results from "./components/Results";
import Stream from "./components/Stream";

Vue.use(VueRouter);

export default new VueRouter({
  mode: "hash",
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
