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
const message = ref('');

function showError(event: {message: string}) {
    message.value = event.message;
    visible.value = true;
}

EventBus.$on("showError", showError);
</script>
