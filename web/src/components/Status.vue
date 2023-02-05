<template>
  <v-card>
    <v-card-title>Status</v-card-title>
    <v-simple-table>
      <tbody>
        <tr v-for="(value, name) in status" :key="name">
          <th>{{ name }}</th>
          <td width="100%">{{ value }}</td>
        </tr>
      </tbody>
    </v-simple-table>
  </v-card>
</template>

<script>
import { mapActions, mapState } from "vuex";

export default {
  name: "Status",
  mounted() {
    this.updateStatus();
  },
  computed: {
    ...mapState(["status"]),
  },
  methods: {
    ...mapActions(["updateStatus"]),
  },
};
</script>
