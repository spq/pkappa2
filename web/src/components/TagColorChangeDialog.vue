<template>
  <v-dialog v-model="visible" width="500" @keydown.enter="updateColor">
    <v-form>
      <v-card>
        <v-card-title>
          <span class="text-h5"
            >Change Color of {{ capitalize(tagType) }}
            <v-chip
              variant="flat"
              :color="tagColor"
              :style="{ color: getContrastTextColor(tagColor) }"
              >{{ tagName }}</v-chip
            ></span
          >
        </v-card-title>
        <v-card-text>
          <v-text-field v-model="tagColor" label="Color" hide-details>
            <template #append>
              <v-menu
                v-model="colorPickerOpen"
                location="top"
                :offset="[-226, 30]"
                :close-on-content-click="false"
              >
                <template #activator="{ props }">
                  <div :style="swatchStyle" v-bind="props" />
                </template>
                <v-card>
                  <v-card-text>
                    <v-color-picker
                      v-model="colorPickerValue"
                      mode="hexa"
                      hide-inputs
                      show-swatches
                      @update:model-value="colorPickerValueUpdate"
                    />
                  </v-card-text>
                </v-card>
              </v-menu>
            </template>
          </v-text-field>
        </v-card-text>
        <v-card-actions>
          <v-spacer></v-spacer>
          <v-btn variant="text" @click="visible = false">Cancel</v-btn>
          <v-btn
            variant="text"
            :disabled="loading"
            :loading="loading"
            :color="error ? 'error' : 'primary'"
            type="submit"
            @click="updateColor"
            >Save</v-btn
          >
        </v-card-actions>
      </v-card>
    </v-form>
  </v-dialog>
</template>

<script lang="ts" setup>
import { EventBus } from "./EventBus";
import { ref, computed, watch } from "vue";
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
const colorPickerOpen = ref(false);
const colorPickerValue = ref("");

// https://codepen.io/JamieCurnow/pen/KKPjraK
const swatchStyle = computed(() => {
  return {
    backgroundColor: tagColor.value,
    cursor: "pointer",
    height: "30px",
    width: "30px",
    borderRadius: colorPickerOpen.value ? "50%" : "4px",
    transition: "border-radius 200ms ease-in-out",
  };
});

watch(colorPickerOpen, (val, old) => {
  if (val && !old) colorPickerValue.value = tagColor.value;
});

EventBus.on("showTagColorChangeDialog", openDialog);

function openDialog(tagIdValue: string) {
  tagId.value = tagIdValue;
  tagType.value = tagIdValue.split("/")[0];
  tagName.value = tagIdValue.substr(tagType.value.length + 1);
  tagColor.value =
    store.tags?.find((e) => e.Name == tagIdValue)?.Color ?? "#000000";
  colorPickerOpen.value = false;
  visible.value = true;
  loading.value = false;
  error.value = false;
}

function colorPickerValueUpdate(color: string) {
  if (colorPickerOpen.value) tagColor.value = color;
}

function updateColor() {
  loading.value = true;
  error.value = false;
  store
    .changeTagColor(tagId.value, tagColor.value)
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
