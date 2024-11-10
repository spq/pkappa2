<template>
  <v-dialog v-model="visible" width="500" @keydown.enter="updateDefinition">
    <v-form>
      <v-card>
        <v-card-title>
          <span class="text-h5"
            >Change Definition of {{ capitalize(tagType) }}
            <v-chip :color="tagColor" :text-color="isDarkColor(tagColor) ? 'white' : 'black'">{{ tagName }}</v-chip></span
          >
        </v-card-title>
        <v-card-text>
          <v-text-field v-model="tagDefinition" label="Definition" hide-details>
          </v-text-field>
        </v-card-text>
        <v-card-actions>
          <v-spacer></v-spacer>
          <v-btn text @click="visible = false">Cancel</v-btn>
          <v-btn
            text
            :disabled="loading"
            :loading="loading"
            :color="error ? 'error' : 'primary'"
            type="submit"
            @click="updateDefinition"
            >Save</v-btn
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
import { isDarkColor } from "@/lib/colors"

const store = useRootStore();
const visible = ref(false);
const loading = ref(false);
const error = ref(false);
const tagId = ref("");
const tagType = ref("");
const tagName = ref("");
const tagColor = ref("");
const tagDefinition = ref("");

EventBus.on("showTagDefinitionChangeDialog", openDialog);

function openDialog(tagIdValue: string) {
  tagId.value = tagIdValue;
  tagType.value = tagIdValue.split("/")[0];
  tagName.value = tagIdValue.substr(tagType.value.length + 1);
  const tag = store.tags?.find((e) => e.Name == tagIdValue);
  tagColor.value = tag?.Color ?? "#000000";
  tagDefinition.value = tag?.Definition ?? "";
  visible.value = true;
  loading.value = false;
  error.value = false;
}
function updateDefinition() {
  loading.value = true;
  error.value = false;
  store
    .changeTagDefinition(tagId.value, tagDefinition.value)
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
