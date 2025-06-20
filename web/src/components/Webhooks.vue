<template>
  <div>
    <ToolBar>
      <v-tooltip location="bottom">
        <template #activator="{ props }">
          <v-btn icon v-bind="props" @click="refresh">
            <v-icon>mdi-refresh</v-icon>
          </v-btn>
        </template>
        <span>Refresh</span>
      </v-tooltip>
      <v-dialog v-model="addDialogVisible" width="500" @keydown.enter="add">
        <template #default>
          <v-form>
            <v-card>
              <v-card-title class="text-h5">
                Add Webhook Endpoint
              </v-card-title>
              <v-card-text>
                Add the address of the Webhook endpoint to connect to.

                <v-text-field
                  v-model="newAddress"
                  label="Address"
                  autofocus
                ></v-text-field>
              </v-card-text>
              <v-card-actions>
                <v-spacer></v-spacer>
                <v-btn variant="text" @click="addDialogVisible = false"
                  >Cancel</v-btn
                >
                <v-btn
                  variant="text"
                  :disabled="addDialogLoading"
                  :loading="addDialogLoading"
                  :color="addDialogError ? 'error' : 'primary'"
                  type="submit"
                  @click="add"
                  >Add</v-btn
                >
              </v-card-actions>
            </v-card>
          </v-form>
        </template>
        <template #activator="{ props: propsDialog }">
          <v-tooltip location="bottom">
            <template #activator="{ props: propsTooltip }">
              <v-btn v-bind="{ ...propsDialog, ...propsTooltip }" icon>
                <v-icon>mdi-plus</v-icon>
              </v-btn>
            </template>
            <span>Add Endpoint</span>
          </v-tooltip>
        </template>
      </v-dialog>
    </ToolBar>
    <v-card density="compact" variant="flat">
      <v-card-title>Webhook Endpoints</v-card-title>
      <v-card-text>
        All URIs that are registered as Webhook endpoints will be notified when
        a new PCAP file was processed. The Webhook endpoints will receive a POST
        request with a JSON body containing an array of strings with the
        absolute paths of the processed PCAP files.
      </v-card-text>
    </v-card>
    <v-data-table
      :headers="headers"
      :items="store.webhooks || []"
      density="compact"
      disable-pagination
      disable-filtering
      hide-default-footer
      hover
    >
      <template #[`item.address`]="{ item }">
        {{ item }}
      </template>
      <template #[`item.delete`]="{ item }">
        <v-tooltip location="bottom">
          <template #activator="{ props }">
            <v-btn
              variant="plain"
              density="compact"
              icon
              v-bind="props"
              @click="
                delDialogAddress = item;
                delDialogVisible = true;
              "
            >
              <v-icon>mdi-delete</v-icon></v-btn
            >
          </template>
          <span>Delete</span>
        </v-tooltip>
      </template>
    </v-data-table>
    <v-dialog v-model="delDialogVisible" width="500" @keydown.enter="del">
      <template #default
        ><v-form>
          <v-card>
            <v-card-title class="text-h5">
              Confirm Webhook endpoint deletion
            </v-card-title>
            <v-card-text>
              Do you want to delete the Webhook endpoint
              <code>{{ delDialogAddress }}</code
              >?
            </v-card-text>
            <v-card-actions>
              <v-spacer></v-spacer>
              <v-btn variant="text" @click="delDialogVisible = false">No</v-btn>
              <v-btn
                variant="text"
                :disabled="delDialogLoading"
                :loading="delDialogLoading"
                :color="delDialogError ? 'error' : 'primary'"
                @click="del"
                >Yes</v-btn
              >
            </v-card-actions>
          </v-card>
        </v-form>
      </template>
    </v-dialog>
  </div>
</template>

<script lang="ts" setup>
import { EventBus } from "./EventBus";
import { ref, onMounted } from "vue";
import { useRootStore } from "@/stores";

const delDialogVisible = ref(false);
const delDialogAddress = ref("");
const delDialogLoading = ref(false);
const delDialogError = ref(false);

const addDialogVisible = ref(false);
const addDialogLoading = ref(false);
const addDialogError = ref(false);
const newAddress = ref("");

const store = useRootStore();
const headers = [
  { title: "Address", value: "address", cellClass: "cursor-pointer" },
  { title: "", value: "delete", sortable: false, cellClass: "cursor-pointer" },
];

onMounted(() => {
  refresh();
});

function refresh() {
  store.updateWebhooks().catch((err: Error) => {
    EventBus.emit(
      "showError",
      `Failed to update registered webhooks: ${err.message}`,
    );
  });
}

function add() {
  addDialogLoading.value = true;
  addDialogError.value = false;
  store
    .addWebhook(newAddress.value)
    .then(() => {
      addDialogVisible.value = false;
      addDialogLoading.value = false;
      newAddress.value = "";
      refresh();
    })
    .catch((err: Error) => {
      addDialogLoading.value = false;
      addDialogError.value = true;
      EventBus.emit(
        "showError",
        `Failed to add webhook endpoint: ${err.message}`,
      );
    });
}
function del() {
  delDialogLoading.value = true;
  delDialogError.value = false;
  store
    .delWebhook(delDialogAddress.value)
    .then(() => {
      delDialogVisible.value = false;
      delDialogLoading.value = false;
      refresh();
    })
    .catch((err: Error) => {
      EventBus.emit(
        "showError",
        `Failed to delete webhook endpoint: ${err.message}`,
      );
      delDialogError.value = true;
      delDialogLoading.value = false;
    });
}
</script>
<style lang="css">
.cursor-pointer {
  cursor: pointer;
}
</style>
