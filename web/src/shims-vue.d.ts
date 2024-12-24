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
