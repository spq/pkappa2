<template>
  <v-dialog v-model="visible" width="500" @keydown.enter="submitCurrent">
    <v-form>
      <v-card>
        <v-card-title>
          <span class="text-h5">CTF Setup Wizards</span>
        </v-card-title>
        <v-tabs v-model="tab" icons-and-text>
          <v-tab href="#tab_flag_regex">
            Setup Flag tags
            <v-icon>mdi-flag</v-icon>
          </v-tab>
          <v-tab href="#tab_service_by_port">
            Setup Service ports
            <v-icon>mdi-cloud-outline</v-icon>
          </v-tab>
        </v-tabs>
        <v-tabs-items v-model="tab">
          <v-tab-item value="tab_service_by_port">
            <v-card-text>
              This wizard will create a service with the given port(s). Enter
              the ports separated by commas, ranges using a dash.

              <v-text-field
                v-model="serviceName"
                label="Service name"
                autofocus
              ></v-text-field>
              <v-text-field
                v-model="servicePorts"
                label="Service ports"
                example="80,8080-8081"
                autofocus
              ></v-text-field>
            </v-card-text>
            <v-card-actions>
              <v-spacer></v-spacer>
              <v-btn text @click="visible = false">Cancel</v-btn>
              <v-btn
                text
                :disabled="
                  serviceName == '' ||
                  !goodServicePorts ||
                  service_by_port_loading
                "
                :loading="service_by_port_loading"
                :color="service_by_port_error ? 'error' : 'primary'"
                type="submit"
                @click="createService"
                >Create Service</v-btn
              >
            </v-card-actions>
          </v-tab-item>
          <v-tab-item value="tab_flag_regex">
            <v-card-text>
              This wizard will create the two tags {{ flagInName }} and
              {{ flagOutName }} with the specified regex below:

              <v-text-field
                v-model="flagRegex"
                label="Flag Regex"
                example="flag_[a-fA-F0-9]{32}"
                autofocus
              ></v-text-field>
            </v-card-text>
            <v-card-actions>
              <v-spacer></v-spacer>
              <v-btn text @click="visible = false">Cancel</v-btn>
              <v-btn
                text
                :disabled="flagRegex == '' || flag_regex_loading"
                :loading="flag_regex_loading"
                :color="flag_regex_error ? 'error' : 'primary'"
                type="submit"
                @click="createFlagTags"
                >Create Flag tags</v-btn
              >
            </v-card-actions>
          </v-tab-item>
        </v-tabs-items>
      </v-card>
    </v-form>
  </v-dialog>
</template>

<script lang="ts" setup>
import { EventBus } from "./EventBus";
import { ref, computed } from "vue";
import { useRootStore } from "@/stores";
import { randomColor } from "@/lib/colors";

const store = useRootStore();
const visible = ref(false);
const tab = ref("");

const flag_regex_loading = ref(false);
const flag_regex_error = ref(false);
const flagRegex = ref("");

const service_by_port_loading = ref(false);
const service_by_port_error = ref(false);
const serviceName = ref("");
const servicePorts = ref("");

const tagPrefix = "tag/";
const servicePrefix = "service/";
const flagInName = "flag_in";
const flagInColor = "#66ff66";
const flagInPrefix = "cdata:";
const flagOutName = "flag_out";
const flagOutColor = "#ff6666";
const flagOutPrefix = "sdata:";

EventBus.on("showCTFWizard", openDialog);

const goodServicePorts = computed(() => {
  return /^(,[0-9]+|,[0-9]+[-:][0-9]+)+$/.test("," + servicePorts.value);
});

function openDialog() {
  visible.value = true;
  tab.value = "flag_regex";

  flag_regex_loading.value = false;
  flag_regex_error.value = false;

  service_by_port_loading.value = false;
  service_by_port_error.value = false;
}

function submitCurrent() {
  switch (tab.value) {
    case "tab_flag_regex":
      createFlagTags();
      break;
    case "tab_service_by_port":
      createService();
      break;
  }
}

function createService() {
  service_by_port_loading.value = true;
  service_by_port_error.value = false;
  const query = `sport:${servicePorts.value
    .replaceAll("-", ":")
    .replaceAll(" ", "")}`;
  store
    .addTag(servicePrefix + serviceName.value, query, randomColor())
    .then(() => {
      visible.value = false;
      service_by_port_loading.value = false;
    })
    .catch((err: string) => {
      service_by_port_error.value = true;
      service_by_port_loading.value = false;
      EventBus.emit("showError", err);
    });
}

function createFlagTags() {
  flag_regex_loading.value = true;
  flag_regex_error.value = false;
  store
    .addTag(tagPrefix + flagInName, flagInPrefix + flagRegex.value, flagInColor)
    .then(() => {
      return store.addTag(
        tagPrefix + flagOutName,
        flagOutPrefix + flagRegex.value,
        flagOutColor
      );
    })
    .then(() => {
      visible.value = false;
      flag_regex_loading.value = false;
    })
    .catch((err: string) => {
      flag_regex_error.value = true;
      flag_regex_loading.value = false;
      EventBus.emit("showError", err);
    });
}
</script>
