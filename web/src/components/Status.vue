<template>
  <div>
    <ToolBar>
      <v-tooltip location="bottom">
        <template #activator="{ props }">
          <v-btn icon v-bind="props" @click="updateStatus">
            <v-icon>mdi-refresh</v-icon>
          </v-btn>
        </template>
        <span>Refresh</span>
      </v-tooltip>
    </ToolBar>
    <v-card>
      <v-card-title>Status</v-card-title>
      <v-table density="compact">
        <tbody>
          <tr v-for="(value, name) in store.status || []" :key="name">
            <th>{{ name }}</th>
            <td width="100%">{{ value }}</td>
          </tr>
        </tbody>
      </v-table>
    </v-card>
    <v-card>
      <v-card-title>Console Stderr</v-card-title>
      <v-table density="compact">
        <tbody>
          <tr v-for="line in store.mainStderr || []" :key="line">
            <td width="100%">{{ line }}</td>
          </tr>
        </tbody>
      </v-table>
    </v-card>
  </div>
</template>

<script lang="ts" setup>
import { onMounted } from "vue";
import { useRootStore } from "@/stores";
import { EventBus } from "./EventBus";

const store = useRootStore();

onMounted(() => {
  updateStatus();
});

function updateStatus() {
  store.updateStatus().catch((err: Error) => {
    EventBus.emit("showError", `Failed to update status: ${err.message}`);
  });
  store.updateMainStderr().catch((err: Error) => {
    EventBus.emit("showError", `Failed to update main stderr: ${err.message}`);
  });
}
</script>
