<template>
  <v-dialog v-model="visible" width="500" @keydown.enter="resetConverterAction">
    <v-form>
      <v-card>
        <v-card-title>
          <span class="text-h5">Confirm reset of {{ converterName }}</span>
        </v-card-title>
        <v-card-text>
          Do you want to reset the converter
          <code>{{ converterName }}</code
          >? This will cause the {{ converterStreamCount }} cached streams to be
          deleted and the converter processes to be restarted.
        </v-card-text>
        <v-card-actions>
          <v-spacer></v-spacer>
          <v-btn text @click="visible = false">No</v-btn>
          <v-btn
            text
            :disabled="loading"
            :loading="loading"
            :color="error ? 'error' : 'primary'"
            @click="resetConverterAction"
            >Yes</v-btn
          >
        </v-card-actions>
      </v-card>
    </v-form>
  </v-dialog>
</template>

<script lang="ts" setup>
import { ref } from "vue";
import { EventBus } from "./EventBus";
import { ConverterStatistics } from "@/apiClient";
import { useRootStore } from "@/stores";

const visible = ref(false);
const loading = ref(false);
const error = ref(false);
const converterName = ref("");
const converterStreamCount = ref(0);

const store = useRootStore();

EventBus.on("showConverterResetDialog", openDialog);

function openDialog(converter: ConverterStatistics) {
  converterName.value = converter.Name;
  converterStreamCount.value = converter.CachedStreamCount;
  visible.value = true;
  loading.value = false;
  error.value = false;
}

function resetConverterAction() {
  loading.value = true;
  error.value = false;
  const converterNameValue = converterName.value;
  store
    .resetConverter(converterNameValue)
    .then(() => {
      visible.value = false;
    })
    .catch((err: string) => {
      error.value = true;
      loading.value = false;
      EventBus.emit("showError", err);
    });
}
</script>
