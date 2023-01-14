<template>
  <v-simple-table dense>
    <thead>
      <tr>
        <th class="text-left">Name</th>
        <th class="text-left">Cached Stream Count</th>
      </tr>
    </thead>
    <tbody>
      <template v-for="converter in converters">
        <tr :key="converter.Name">
          <th>
            {{ converter.Name }}
          </th>
          <th>
            {{ converter.CachedStreamCount }}
          </th>
        </tr>
      </template>
    </tbody>
  </v-simple-table>
</template>

<script>
import { mapActions, mapState } from "vuex";

export default {
  name: "Converters",
  mounted() {
    this.updateTags();
    this.updateConverters();
  },
  computed: {
    ...mapState(["tags", "converters"]),
  },
  methods: {
    ...mapActions(["updateTags", "updateConverters"]),
  },
};
</script>