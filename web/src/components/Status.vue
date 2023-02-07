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
  computed: {
    ...mapState(["status"]),
  },
  mounted() {
    this.updateStatus();
  },
  methods: {
    ...mapActions(["updateStatus"]),
  },
};
</script>
