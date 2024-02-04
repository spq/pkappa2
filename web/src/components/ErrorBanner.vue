<template>
  <v-snackbar v-model="visible" app color="error" timeout="5000">
    {{ message }}
    <template #action="{ attrs }">
      <v-btn icon v-bind="attrs" @click="visible = false">
        <v-icon>mdi-close</v-icon>
      </v-btn>
    </template>
  </v-snackbar>
</template>

<script lang="ts" setup>
import { ref } from "vue";
import { EventBus } from "./EventBus";

const visible = ref(false);
const message = ref("");

EventBus.on("showError", showError);

function showError(msg: string) {
  message.value = msg;
  visible.value = true;
}
</script>
