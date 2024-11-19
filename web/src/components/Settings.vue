<template>
  <div>
    <v-card>
      <v-card-title>Global Settings</v-card-title>
      <v-simple-table>
        <tbody>
          <tr v-for="(value, name) in config || []" :key="name">
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
import { computed, onMounted, watch } from "vue";
import { useRootStore } from "@/stores";
import { EventBus } from "./EventBus";
import { ref } from "vue";

const store = useRootStore();
const config = computed(() => store.clientConfig);
const autoInsertLimitToQuery = ref(
  store.clientConfig?.AutoInsertLimitToQuery ?? false,
);

watch(config, (newValue) => {
  autoInsertLimitToQuery.value = newValue?.AutoInsertLimitToQuery ?? false;
});

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
