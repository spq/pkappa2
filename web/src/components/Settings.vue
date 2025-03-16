<template>
  <div>
    <v-card>
      <v-card-title>Global Settings</v-card-title>
      <v-card-text>
        These settings are global and will affect all users immediately.
      </v-card-text>
      <v-table density="compact">
        <tbody>
          <tr>
            <th scope="row">
              <v-switch
                v-model="autoInsertLimitToQuery"
                color="primary"
                @change="save"
              />
            </th>
            <td>
              Limit tag queries to the last hour by default. When checked auto
              appends <code>ltime:-1h:</code> to queries clicked on in the
              navbar.
            </td>
          </tr>
        </tbody>
      </v-table>
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
