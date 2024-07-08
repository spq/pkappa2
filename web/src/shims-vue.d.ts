/* eslint-disable */
declare module "*.vue" {
  import type { DefineComponent } from "vue";
  const component: DefineComponent<{}, {}, any>;
  export default component;
}

declare module 'vue' {
  import { CompatVue } from '@vue/runtime-dom';
  const Vue: CompatVue;
  export default Vue;
  export * from '@vue/runtime-dom';
  // const { configureCompat } = Vue;
  // export { configureCompat };
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
