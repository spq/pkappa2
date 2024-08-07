<template>
  <div>
    <ToolBar>
      <v-tooltip bottom>
        <template #activator="{ on, attrs }">
          <v-btn v-bind="attrs" icon v-on="on" @click="refreshConverters">
            <v-icon>mdi-refresh</v-icon>
          </v-btn>
        </template>
        <span>Refresh</span>
      </v-tooltip>
    </ToolBar>
    <v-data-table
      :headers="headers"
      :items="items"
      item-key="name"
      single-expand
      show-expand
      dense
      disable-pagination
      @click:row="rowClick"
    >
      <template #expanded-item="{ item }">
        <td colspan="4">
          <v-chip
            v-for="process in item.converter.Processes"
            :key="process.Pid"
            label
            link
            class="ma-2"
            :color="process.Running ? 'green' : 'yellow'"
            @click="showErrorLog(process, item.converter)"
          >
            <v-tooltip bottom>
              <template #activator="{ on, attrs }">
                <v-icon v-if="process.Errors > 0" v-bind="attrs" v-on="on">
                  mdi-alert-outline
                </v-icon>
              </template>
              <span>
                Num errors in Process: {{ process.Errors }}, click to view
                stderr!
              </span>
            </v-tooltip>
            PID: {{ process.Pid }}
          </v-chip>
          <v-tooltip bottom>
            <template #activator="{ on, attrs }">
              <v-btn
                v-bind="attrs"
                icon
                v-on="on"
                @click="confirmConverterReset(item.converter)"
              >
                <v-icon>mdi-restart-alert</v-icon>
              </v-btn>
            </template>
            <span>Reset Converter</span>
          </v-tooltip>
        </td>
      </template>
    </v-data-table>
    <v-dialog
      :value="shownProcess !== null"
      width="600px"
      @click:outside="shownProcess = null"
    >
      <v-card>
        <v-card-title>
          <span class="text-h5">Stderr of Process {{ shownProcess?.Pid }}</span>
        </v-card-title>
        <v-card-text>
          <div v-if="fetchStderrError !== null">
            {{ fetchStderrError }}
          </div>
          <div v-else>
            <pre><!--
              -->{{ shownProcessErrors?.Stderr?.join('\n') }}<!--
            --></pre>
          </div>
        </v-card-text>
        <v-card-actions>
          <v-spacer></v-spacer>
          <v-btn text @click="shownProcess = null"> Close </v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </div>
</template>

<script lang="ts" setup>
import { EventBus } from "./EventBus";
import APIClient, {
  ConverterStatistics,
  ProcessStats,
  ProcessStderr,
} from "@/apiClient";
import ToolBar from "./ToolBar.vue";
import { computed, onMounted, ref } from "vue";
import { useRootStore } from "@/stores";
import { DataTableItemProps } from "vuetify";

const store = useRootStore();
const headers = [
  { text: "Name", value: "name", cellClass: "cursor-pointer" },
  {
    text: "Cached Stream Count",
    value: "cachedStreamCount",
    cellClass: "cursor-pointer",
  },
  {
    text: "Running Processes",
    value: "runningProcesses",
    cellClass: "cursor-pointer",
  },
  {
    text: "Failed Processes",
    value: "failedProcesses",
    cellClass: "cursor-pointer",
  },
];
const shownProcess = ref<ProcessStats | null>(null);
const loadingStderr = ref(false);
const fetchStderrError = ref<string | null>(null);
const shownProcessErrors = ref<ProcessStderr | null>(null);

const items = computed(() => {
  return (
    store.converters?.map((converter) => ({
      name: converter.Name,
      cachedStreamCount: converter.CachedStreamCount,
      runningProcesses: converter.Processes.filter((process) => process.Running)
        .length,
      failedProcesses: converter.Processes.filter((process) => !process.Running)
        .length,
      converter,
    })) ?? []
  );
});

onMounted(() => {
  refreshConverters();
});

function refreshConverters() {
  store.updateTags().catch((err: string) => {
    EventBus.emit("showError", `Failed to update tags: ${err}`);
  });
  store.updateConverters().catch((err: string) => {
    EventBus.emit("showError", `Failed to update converters: ${err}`);
  });
}

function rowClick(item: unknown, handler: DataTableItemProps) {
  handler.expand(!handler.isExpanded);
}

function confirmConverterReset(converter: ConverterStatistics) {
  EventBus.emit("showConverterResetDialog", converter);
}

function showErrorLog(process: ProcessStats, converter: ConverterStatistics) {
  if (process.Errors === 0) return;
  loadingStderr.value = true;
  shownProcess.value = process;
  APIClient.getConverterStderrs(converter.Name, process.Pid)
    .then((res) => {
      shownProcessErrors.value = res;
      if (shownProcessErrors.value === null) {
        fetchStderrError.value = "Stderr is empty";
      }
    })
    .catch((err: string | Error) => {
      fetchStderrError.value = err.toString();
    })
    .finally(() => {
      loadingStderr.value = false;
    });
}
</script>
<style lang="css">
.cursor-pointer {
  cursor: pointer;
}
</style>
