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

<script>
import { EventBus } from "./EventBus";

export default {
  name: "ErrorBanner",
  data() {
    return {
      visible: false,
      message: "",
    };
  },
  created() {
    EventBus.$on("showError", this.showError);
  },
  methods: {
    showError({ message }) {
      this.message = message;
      this.visible = true;
    },
  },
};
</script>