<template>
  <v-dialog v-model="visible" width="500" @keydown.enter="deleteTag">
    <v-form>
      <v-card>
        <v-card-title>
          <span class="text-h5"
            >Confirm {{ capitalize(tagType) }} deletion</span
          >
        </v-card-title>
        <v-card-text>
          Do you want to delete the {{ capitalize(tagType) }}
          <code>{{ tagName }}</code
          >?
        </v-card-text>
        <v-card-actions>
          <v-spacer></v-spacer>
          <v-btn text @click="visible = false">No</v-btn>
          <v-btn
            text
            :disabled="loading"
            :loading="loading"
            :color="error ? 'error' : 'primary'"
            @click="deleteTag"
            >Yes</v-btn
          >
        </v-card-actions>
      </v-card>
    </v-form>
  </v-dialog>
</template>

<script lang="ts" setup>
import { EventBus } from "./EventBus";
import { ref } from "vue";
import { useRootStore } from "@/stores";
import { capitalize } from "@/filters";

const store = useRootStore();
const visible = ref(false);
const loading = ref(false);
const error = ref(false);
const tagId = ref("");
const tagType = ref("");
const tagName = ref("");

EventBus.on("showTagDeleteDialog", openDialog);

function openDialog(tagIdValue: string) {
  tagId.value = tagIdValue;
  tagType.value = tagIdValue.split("/")[0];
  tagName.value = tagIdValue.substr(tagType.value.length + 1);
  visible.value = true;
  loading.value = false;
  error.value = false;
}

function deleteTag() {
  loading.value = true;
  error.value = false;
  store
    .delTag(tagId.value)
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
