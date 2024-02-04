// vuex.d.ts
import { Store } from "vuex";
import { State, Getters } from "./store";

interface MyStore extends Store<State> {
  getters: Getters;
}

declare module "@vue/runtime-core" {
  // provide typings for `this.$store`
  interface ComponentCustomProperties {
    $store: MyStore;
  }
}
