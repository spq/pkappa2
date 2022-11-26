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
      <template v-for="typ in ['tag', 'service', 'mark', 'generated']">
        <tr :key="typ">
          <th colspan="4">
            <v-icon
              >mdi-{{
                { service: "anchor", tag: "tag", mark: "bookmark", generated: "robot" }[typ]
              }}</v-icon
            >
            {{ typ.charAt(0).toUpperCase() + typ.substring(1) }}s
          </th>
        </tr>
        <template v-for="tag in tags">
          <tr v-if="tag.Name.startsWith(typ + '/')" :key="typ + '/' + tag.Name">
            <td><v-icon>mdi-circle-small</v-icon><v-chip :color="tag.Color">{{ tag.Name.substring(1 + typ.length) }}</v-chip></td>
            <td>{{ tag.Definition }}</td>
            <td>
              Matching {{ tag.MatchingCount }} Streams<span
                v-if="tag.UncertainCount != 0"
                >, {{ tag.UncertainCount }} pending</span
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
        </template>
      </template>
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
      this.$emit("searchStreams", this.$options.filters.tagForURI(tag.Name), 0);
    },
  },
};
</script>