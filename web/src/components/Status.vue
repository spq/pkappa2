<template>
  <div>
    <ToolBar>
      <v-tooltip bottom>
        <template #activator="{ on, attrs }">
          <v-btn v-bind="attrs" icon v-on="on" @click="updateStatus">
            <v-icon>mdi-refresh</v-icon>
          </v-btn>
        </template>
        <span>Refresh</span>
      </v-tooltip>
    </ToolBar>
    <v-card>
      <v-card-title>Status</v-card-title>
      <v-simple-table>
        <tbody>
          <tr v-for="(value, name) in store.status || []" :key="name">
            <th>{{ name }}</th>
            <td width="100%">{{ value }}</td>
          </tr>
        </tbody>
      </v-simple-table>
    </v-card>
  </div>
</template>

<script lang="ts" setup>
import ToolBar from "./ToolBar.vue";
import { onMounted } from "vue";
import { useRootStore } from "@/stores";
import { EventBus } from "./EventBus";

const store = useRootStore();

onMounted(() => {
  updateStatus();
});

function updateStatus() {
  store.updateStatus().catch((err: string) => {
    EventBus.emit("showError", `Failed to update status: ${err}`);
  });
}
</script>
