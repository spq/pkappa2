<template>
  <v-snackbar v-model="visible" :color="color" timeout="5000">
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
const color = ref("error");

EventBus.on("showError", showError);
EventBus.on("showMessage", showMessage);

function showError(msg: string) {
  message.value = msg;
  color.value = "error";
  visible.value = true;
  console.error(msg);
}

function showMessage(msg: string) {
  message.value = msg;
  color.value = "success";
  visible.value = true;
  console.log(msg);
}
</script>
