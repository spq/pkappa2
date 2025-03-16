<template>
  <div>
    <v-card>
      <v-card-title>Global Settings</v-card-title>
      <v-simple-table>
        <tbody>
          <tr>
            <th scope="row">AutoInsertLimitToQuery</th>
            <td>
              <input
                v-model="autoInsertLimitToQuery"
                type="checkbox"
                @change="save"
              />
            </td>
            <td>
              When checked auto appends limit to queries clicked on in navbar
            </td>
          </tr>
        </tbody>
      </v-simple-table>
    </v-card>
  </div>
</template>

<script lang="ts" setup>
import { watch } from "vue";
import { useRootStore } from "@/stores";
import { EventBus } from "./EventBus";
import { ref } from "vue";

const store = useRootStore();
const autoInsertLimitToQuery = ref(store.clientConfig.AutoInsertLimitToQuery);

//TODO find a way to only listen to clientconfig
watch(store, (newValue) => {
  autoInsertLimitToQuery.value =
    newValue?.clientConfig.AutoInsertLimitToQuery ?? false;
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
