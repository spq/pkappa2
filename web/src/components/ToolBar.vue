<template>
  <div>
    <v-toolbar
      id="toolbarReal"
      dense
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

<script>
export default {
  data() {
    return {
      toolbarWidth: 0,
      toolbarHeight: 0,
      toolbarResizeObserver: null,
    };
  },
  mounted() {
    this.toolbarResizeObserver = new ResizeObserver(this.onToolbarResize);
    this.toolbarResizeObserver.observe(document.getElementById("toolbarReal"));
    this.toolbarResizeObserver.observe(document.getElementById("toolbarDummy"));
    this.onToolbarResize();
  },
  methods: {
    onToolbarResize() {
      const tbd = document.getElementById("toolbarDummy");
      if (tbd) this.toolbarWidth = tbd.offsetWidth;
      const tbr = document.getElementById("toolbarReal");
      if (tbr) this.toolbarHeight = tbr.offsetHeight;
    },
  },
};
</script>
