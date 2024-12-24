<template>
  <v-dialog v-model="visible" width="500" @keydown.enter="updateName">
    <v-form>
      <v-card>
        <v-card-title>
          <span class="text-h5"
            >Change Name of {{ capitalize(tagType) }}
            <v-chip
              :color="tagColor"
              :text-color="getContrastTextColor(tagColor)"
              >{{ tagName }}</v-chip
            ></span
          >
        </v-card-title>
        <v-card-text>
          <v-text-field v-model="tagNewName" label="New Name" hide-details>
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
            @click="updateName"
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
import { getContrastTextColor } from "@/lib/colors";

const store = useRootStore();
const visible = ref(false);
const loading = ref(false);
const error = ref(false);
const tagId = ref("");
const tagType = ref("");
const tagName = ref("");
const tagColor = ref("");
const tagNewName = ref("");

EventBus.on("showTagNameChangeDialog", openDialog);

function openDialog(tagIdValue: string) {
  tagId.value = tagIdValue;
  tagType.value = tagIdValue.split("/")[0];
  tagName.value = tagIdValue.substr(tagType.value.length + 1);
  const tag = store.tags?.find((e) => e.Name == tagIdValue);
  tagColor.value = tag?.Color ?? "#000000";
  tagNewName.value = tagName.value;
  visible.value = true;
  loading.value = false;
  error.value = false;
}
function updateName() {
  loading.value = true;
  error.value = false;
  store
    .changeTagName(tagId.value, `${tagType.value}/${tagNewName.value}`)
    .then(() => {
      visible.value = false;
    })
    .catch((err: Error) => {
      error.value = true;
      loading.value = false;
      EventBus.emit("showError", err.message);
    });
}
</script>
