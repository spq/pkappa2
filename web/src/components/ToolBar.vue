<template>
  <div>
    <v-toolbar
      id="toolbarReal"
      density="compact"
      flat
      :style="{
        position: 'fixed',
        width: `${toolbarWidth}px`,
        zIndex: 5,
        borderBottom: '1px solid #eee !important',
      }"
    >
      <slot />
    </v-toolbar>
    <div
      id="toolbarDummy"
      :style="{
        visibility: 'hidden',
        height: `${toolbarHeight}px`,
      }"
    />
  </div>
</template>

<script lang="ts" setup>
import { ref, onMounted } from "vue";

const toolbarWidth = ref(0);
const toolbarHeight = ref(0);
const toolbarResizeObserver = ref<ResizeObserver | null>(null);

onMounted(() => {
  toolbarResizeObserver.value = new ResizeObserver(onToolbarResize);
  const tbr = document.getElementById("toolbarReal");
  if (tbr) toolbarResizeObserver.value.observe(tbr);
  const tbd = document.getElementById("toolbarDummy");
  if (tbd) toolbarResizeObserver.value.observe(tbd);
  onToolbarResize();
});

function onToolbarResize() {
  const tbd = document.getElementById("toolbarDummy");
  if (tbd) toolbarWidth.value = tbd.offsetWidth;
  const tbr = document.getElementById("toolbarReal");
  if (tbr) toolbarHeight.value = tbr.offsetHeight;
  console.log("toolbarWidth", toolbarWidth.value);
  console.log("toolbarHeight", toolbarHeight.value);
}
</script>
