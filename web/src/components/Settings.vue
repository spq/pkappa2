<template>
  <div>
    <ToolBar>
      <v-tooltip bottom>
        <template #activator="{ on, attrs }">
          <v-btn v-bind="attrs" icon v-on="on" @click="getSettings">
            <v-icon>mdi-refresh</v-icon>
          </v-btn>
        </template>
        <span>Refresh</span>
      </v-tooltip>
    </ToolBar>
    <v-card>
      <v-card-title>Global Settings</v-card-title>
      <v-simple-table>
        <tbody>
          <tr v-for="(value, name) in store.clientConfig || []" :key="name">
            <th>{{ name }}</th>
            <td width="100%">
              <input
                v-model="autoInsertLimitToQuery"
                type="checkbox"
                @change="save"
              />
            </td>
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
import { ref } from "vue";

const store = useRootStore();
const autoInsertLimitToQuery = ref(false);

onMounted(() => {
  getSettings();
});

function getSettings() {
  store
    .getClientConfig()
    .then(
      () =>
        (autoInsertLimitToQuery.value =
          store.clientConfig?.AutoInsertLimitToQuery ?? false),
    )
    .catch((err: string) => {
      EventBus.emit("showError", `Failed to get settings: ${err}`);
    });
}

function save() {
  store
    .updateClientConfig({
      AutoInsertLimitToQuery: autoInsertLimitToQuery.value,
    })
    .catch((err: string) => {
      EventBus.emit("showError", `Failed to set settings: ${err}`);
    });
}
</script>
