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

<script>
import { mapActions } from "vuex";
import { EventBus } from "./EventBus";

export default {
  name: "ConverterResetDialog",
  data() {
    return {
      visible: false,
      loading: false,
      error: false,
      converterName: "",
      converterStreamCount: 0,
    };
  },
  created() {
    EventBus.$on("showConverterResetDialog", this.openDialog);
  },
  methods: {
    ...mapActions(["resetConverter"]),
    openDialog({ converter }) {
      this.converterName = converter.Name;
      this.converterStreamCount = converter.CachedStreamCount;
      this.visible = true;
      this.loading = false;
      this.error = false;
    },
    resetConverterAction() {
      this.loading = true;
      this.error = false;
      this.resetConverter(this.converterName)
        .then(() => {
          this.visible = false;
        })
        .catch((err) => {
          this.error = true;
          this.loading = false;
          EventBus.$emit("showError", { message: err });
        });
    },
  },
};
</script>
