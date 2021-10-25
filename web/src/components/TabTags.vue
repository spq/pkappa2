<template>
  <v-simple-table dense>
    <thead>
      <tr>
        <th class="text-left">Name</th>
        <th class="text-left">Query</th>
        <th colspan="2" class="text-left">Status</th>
      </tr>
    </thead>
    <tbody>
      <tr v-for="tag in tags" :key="tag.Name">
        <td>{{ tag.Name }}</td>
        <td>{{ tag.Definition }}</td>
        <td>
          Matching {{ tag.MatchingCount }} Streams<span
            v-if="tag.IndexesPending != 0"
            >, {{ tag.IndexesPending }} Indexes pending</span
          ><span v-if="tag.Referenced">, Referenced by another tag</span>
        </td>
        <td align="right">
          <v-btn icon @click="searchStreamsForTag(tag)"
            ><v-icon>mdi-magnify</v-icon></v-btn
          >
          <v-btn
            :disabled="tag.Referenced"
            icon
            @click="delTag(tag.Name)"
            :loading="tagDelStatus != null && tagDelStatus.inProgress"
            ><v-icon>mdi-delete</v-icon></v-btn
          >
        </td>
      </tr>
    </tbody>
  </v-simple-table>
</template>

<script>
import { mapActions, mapState } from "vuex";

export default {
  computed: {
    ...mapState(["tags", "tagDelStatus"]),
  },
  methods: {
    ...mapActions(["delTag"]),
    searchStreamsForTag(tag) {
      this.$emit("searchStreams", "tag:" + tag.Name, 0);
    },
  },
};
</script>