/* eslint-disable */
declare module "*.vue" {
  import type { DefineComponent } from "vue";
  const component: DefineComponent<{}, {}, any>;
  export default component;
}

/* eslint-disable @typescript-eslint/no-namespace */
declare module "shims-vue" {
  global {
    namespace NodeJS {
      interface ProcessEnv {
        NODE_ENV: "production" | "development" | undefined;
      }
      interface Process {
        env: ProcessEnv;
      }
    }
  }
}

declare module "vue-filter-pretty-bytes" {
  import { PluginObject } from "vue";
  type prettyBytes = (
    bytes: number,
    decimals: number,
    kib: boolean,
    maxuint: string,
  ) => string;
  interface VueFilterPrettyBytes extends PluginObject<undefined> {}

  module "vue/types/vue" {
    interface Vue {
      $prettyBytes: prettyBytes;
    }
  }

  const VueFilterPrettyBytes: VueFilterPrettyBytes;
  export default VueFilterPrettyBytes;
}
