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
                Add PCAP-over-IP Endpoint
              </v-card-title>
              <v-card-text>
                Add the address of the PCAP-over-IP endpoint to connect to.

                <v-text-field
                  v-model="newAddress"
                  label="Address"
                  autofocus
                  :rules="[() => goodNewAddress]"
                ></v-text-field>
              </v-card-text>
              <v-card-actions>
                <v-spacer></v-spacer>
                <v-btn variant="text" @click="addDialogVisible = false">Cancel</v-btn>
                <v-btn
                  variant="text"
                  :disabled="!goodNewAddress || addDialogLoading"
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
              <v-btn
                v-bind="{ ...propsDialog, ...propsTooltip }"
                icon
              >
                <v-icon>mdi-plus</v-icon>
              </v-btn>
            </template>
            <span>Add Endpoint</span>
          </v-tooltip>
        </template>
      </v-dialog>
    </ToolBar>
    <v-data-table
      :headers="headers"
      :items="store.pcapOverIPEndpoints || []"
      item-key="address"
      dense
      disable-pagination
      disable-filtering
      hide-default-footer
    >
      <template #[`item.status`]="props">
        <v-tooltip>
          <template #activator="{ props: propsTooltip }">
            <div v-bind="propsTooltip">
              {{
                props.item.LastConnected === 0 &&
                props.item.LastDisconnected === 0
                  ? "Never connected"
                  : props.item.LastConnected > props.item.LastDisconnected
                    ? `Connected since ${moment
                        .duration(
                          currentTime - props.item.LastConnected / 1_000_000,
                        )
                        .humanize()}`
                    : `Disconnected since ${moment
                        .duration(
                          currentTime - props.item.LastDisconnected / 1_000_000,
                        )
                        .humanize()}`
              }}
            </div>
          </template>
          <span
            >last connected:
            {{
              props.item.LastConnected == 0
                ? "never"
                : moment(props.item.LastConnected / 1_000_000)
            }}
            / last disconnected:
            {{
              props.item.LastDisconnected == 0
                ? "never"
                : moment(props.item.LastDisconnected / 1_000_000)
            }}</span
          >
        </v-tooltip>
      </template>
      <template #[`item.delete`]="{ item }">
        <v-tooltip location="bottom">
          <template #activator="{ props }">
            <v-btn
             
              icon
              v-bind="props"
              @click="
                delDialogAddress = item.Address;
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
              Confirm PCAP-over-IP endpoint deletion
            </v-card-title>
            <v-card-text>
              Do you want to delete the PCAP-over-IP endpoint
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
import ToolBar from "./ToolBar.vue";
import { ref, computed, onMounted, onUnmounted } from "vue";
import { useRootStore } from "@/stores";
import moment from "moment";

const delDialogVisible = ref(false);
const delDialogAddress = ref("");
const delDialogLoading = ref(false);
const delDialogError = ref(false);

const addDialogVisible = ref(false);
const addDialogLoading = ref(false);
const addDialogError = ref(false);
const newAddress = ref("");
const goodNewAddress = computed(() => {
  return newAddress.value.match(/^.+:[0-9]+$/) != null;
});

const store = useRootStore();
const headers = [
  { text: "Address", value: "Address", cellClass: "cursor-pointer" },
  { text: "Status", value: "status", cellClass: "cursor-pointer" },
  {
    text: "Packets Received",
    value: "ReceivedPackets",
    cellClass: "cursor-pointer",
  },
  { text: "", value: "delete", sortable: false, cellClass: "cursor-pointer" },
];

const ticker = ref<NodeJS.Timeout | null>(null);
const currentTime = ref(new Date().getTime());

onMounted(() => {
  ticker.value = setInterval(() => {
    currentTime.value = new Date().getTime();
  }, 1_000);
});

onUnmounted(() => {
  if (ticker.value != null) clearInterval(ticker.value);
});

onMounted(() => {
  refresh();
});

function refresh() {
  store.updatePcapOverIPEndpoints().catch((err: Error) => {
    EventBus.emit(
      "showError",
      `Failed to update PCAP-over-IP endpoints: ${err.message}`,
    );
  });
}

function add() {
  addDialogLoading.value = true;
  addDialogError.value = false;
  store
    .addPcapOverIPEndpoint(newAddress.value)
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
        `Failed to add PCAP-over-IP endpoint: ${err.message}`,
      );
    });
}
function del() {
  delDialogLoading.value = true;
  delDialogError.value = false;
  store
    .delPcapOverIPEndpoint(delDialogAddress.value)
    .then(() => {
      delDialogVisible.value = false;
      delDialogLoading.value = false;
      refresh();
    })
    .catch((err: Error) => {
      EventBus.emit(
        "showError",
        `Failed to delete PCAP-over-IP endpoint: ${err.message}`,
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
