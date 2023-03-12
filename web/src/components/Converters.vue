<template>
  <div>
    <v-data-table
      :headers="headers"
      :items="items"
      item-key="name"
      single-expand
      show-expand
      dense
      @click:row="rowClick"
    >
      <template #expanded-item="{ item }">
        <td colspan="4">
          <v-chip
            v-for="(process) in item.converter.Processes"
            label
            link
            :key="process.Pid"
            class="ma-2"
            :color="process.Running ? 'green' : 'yellow'"
            @click="showErrorLog(process, item.converter)"
          >
            <v-tooltip bottom>
              <template #activator="{ on, attrs }">
                <v-icon
                  v-if="process.Errors > 0"
                  v-bind="attrs"
                  v-on="on"
                >
                  mdi-alert-outline
                </v-icon>
              </template>
              <span>Num errors in Process: {{ process.Errors }}, click to view stderr!</span>
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
      @click:outside="shownProcess = null"
      width="600px"
    >
      <v-card>
        <v-card-title>
          <span class="text-h5">Stderr of Process {{shownProcess?.Pid}}</span>
        </v-card-title>
        <v-card-text>
          <div v-if="fetchStderrError !== null">
            {{fetchStderrError}}
          </div>
          <div v-else>
            <pre><!--
              -->{{ shownProcessErrors?.Stderr?.join('\n') }}<!--
            --></pre>
          </div>
        </v-card-text>
        <v-card-actions>
          <v-spacer></v-spacer>
          <v-btn
            text
            @click="shownProcess = null"
          >
            Close
          </v-btn>
        </v-card-actions>
      </v-card>
    </v-dialog>
  </div>
</template>

<script>
import {mapActions, mapState} from "vuex";
import {EventBus} from "./EventBus";
import APIClient from "../apiClient";

export default {
  name: "Converters",
  data: () => ({
    headers: [
      {text: 'Name', value: 'name', cellClass: 'cursor-pointer'},
      {text: 'Cached Stream Count', value: 'cachedStreamCount', cellClass: 'cursor-pointer'},
      {text: 'Running Processes', value: 'runningProcesses', cellClass: 'cursor-pointer'},
      {text: 'Failed Processes', value: 'failedProcesses', cellClass: 'cursor-pointer'},
    ],
    shownProcess: null,
    loadingStderr: false,
    fetchStderrError: null,
    shownProcessErrors: null,
  }),
  computed: {
    ...mapState(["tags", "converters", 'convertersStderr']),
    items() {
      return this.converters?.map((converter) => ({
        name: converter.Name,
        cachedStreamCount: converter.CachedStreamCount,
        runningProcesses: converter.Processes.filter((process) => process.Running).length,
        failedProcesses: converter.Processes.filter((process) => !process.Running).length,
        converter,
      })) ?? [];
    },
  },
  mounted() {
    this.updateTags();
    this.updateConverters();
  },
  methods: {
    ...mapActions(["updateTags", "updateConverters", 'fetchConverterStderrs']),
    rowClick(item, handler) {
      handler.expand(!handler.isExpanded);
    },
    confirmConverterReset(converter) {
      EventBus.$emit("showConverterResetDialog", {converter});
    },
    async showErrorLog(process, converter) {
      this.loadingStderr = true;
      this.shownProcess = process;
      try {
        this.shownProcessErrors = (await APIClient.getConverterStderrs(converter.Name)).filter(processError => processError.Pid === process.Pid)?.[0];
        if (this.shownProcessErrors === null) {
          this.fetchStderrError = 'Stderr is empty';
        }
      } catch (err) {
        this.fetchStderrError = err.toString();
      } finally {
        this.loadingStderr = false;
      }
      console.log(this.shownProcessErrors, this.shownProcessErrors?.Stderr?.join('\n'));
    },
  },
};
</script>
<style lang="css">
  .cursor-pointer {
    cursor: pointer;
  }
</style>
