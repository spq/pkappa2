<template>
  <v-toolbar dark>
    <v-text-field
      solo-inverted
      flat
      hide-details
      label="Search for streams + Press Enter"
      prepend-inner-icon="mdi-magnify"
      v-model="query"
      @keyup.enter="onSearchSubmition"
    ></v-text-field>
  </v-toolbar>
</template>

<script>
export default {
  data() {
    return {
      query: this.defaultQuery,
    };
  },
  props: ["defaultQuery"],
  methods: {
    onSearchSubmition() {
      this.$emit("search-submitted", this.query, 0);
    },
  },
};
</script>
