<template>
  <v-snackbar v-model="visible" color="error" timeout="5000">
    {{ message }}
    <template #actions>
      <v-btn icon @click="visible = false">
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
  console.error(msg);
}
</script>
