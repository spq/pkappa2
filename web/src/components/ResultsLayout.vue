<template>
  <div class="search-layout">
    <div
      v-if="!isExpanded"
      tabindex="0"
      :class="['top-pane', 'overflow-auto', { 'stream-open': streamOpen }]"
    >
      <Results />
    </div>
    <v-scroll-y-reverse-transition>
      <div
        v-if="streamOpen"
        class="bottom-pane elevation-4 border-t overflow-auto"
      >
        <router-view />
      </div>
    </v-scroll-y-reverse-transition>
  </div>
</template>

<script lang="ts" setup>
import { computed } from "vue";
import { useRoute } from "vue-router";

const route = useRoute();
const streamOpen = computed(() => route.name === "stream");
const isExpanded = computed(
  () => streamOpen.value && route.query.expand === "true",
);
</script>

<style scoped>
.search-layout {
  display: flex;
  flex-direction: column;
  height: calc(100vh - 64px);
}
.top-pane {
  flex: 1 1 auto;
}
.top-pane.stream-open {
  flex: 1 1 30%;
}
.bottom-pane {
  flex: 1 1 70%;
  min-height: 300px;
}
</style>
