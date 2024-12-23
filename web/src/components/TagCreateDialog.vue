<template>
  <v-dialog v-model="visible" width="500" @keydown.enter="createTag">
    <v-form>
      <v-card>
        <v-card-title>
          <span class="text-h5">Create {{ capitalize(tagType) }}</span>
        </v-card-title>
        <v-card-text>
          <v-text-field v-model="tagName" label="Name" autofocus></v-text-field>
          <v-text-field v-model="tagColor" label="Color" hide-details>
            <template #append>
              <v-menu
                v-model="colorPickerOpen"
                top
                nudge-bottom="182"
                nudge-left="32"
                :close-on-content-click="false"
              >
                <template #activator="{ on }">
                  <div :style="swatchStyle" v-on="on" />
                </template>
                <v-card>
                  <v-card-text>
                    <v-color-picker
                      v-model="colorPickerValue"
                      mode="hexa"
                      hide-mode-switch
                      hide-inputs
                      show-swatches
                      flat
                      @update:color="colorPickerValueUpdate"
                    />
                  </v-card-text>
                </v-card>
              </v-menu>
            </template>
          </v-text-field>
        </v-card-text>
        <v-card-actions>
          <v-spacer></v-spacer>
          <v-btn text @click="visible = false">Cancel</v-btn>
          <v-btn
            text
            :disabled="tagName == '' || loading"
            :loading="loading"
            :color="error ? 'error' : 'primary'"
            type="submit"
            @click="createTag"
            >Create</v-btn
          >
        </v-card-actions>
      </v-card>
    </v-form>
  </v-dialog>
</template>

<script lang="ts" setup>
import { EventBus } from "./EventBus";
import { computed, ref, watch } from "vue";
import { useRootStore } from "@/stores";
import { capitalize } from "@/filters";
import { randomColor } from "@/lib/colors";

const store = useRootStore();
const visible = ref(false);
const loading = ref(false);
const error = ref(false);
const tagQuery = ref("");
const tagStreams = ref<number[]>([]);
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

EventBus.on("showCreateTagDialog", openDialog);

function openDialog(
  tagTypeValue: string,
  tagQueryValue: string,
  tagStreamsValue: number[],
) {
  tagType.value = tagTypeValue;
  tagQuery.value = tagQueryValue;
  tagStreams.value = tagStreamsValue;
  tagName.value = "";
  tagColor.value = randomColor();
  colorPickerOpen.value = false;
  visible.value = true;
  loading.value = false;
  error.value = false;
}

function colorPickerValueUpdate(color: { hex: string }) {
  if (colorPickerOpen.value) tagColor.value = color.hex;
}

function createTag() {
  loading.value = true;
  error.value = false;
  (tagType.value == "mark"
    ? store.markTagNew(
        `${tagType.value}/${tagName.value}`,
        tagStreams.value,
        tagColor.value,
      )
    : store.addTag(
        `${tagType.value}/${tagName.value}`,
        tagQuery.value,
        tagColor.value,
      )
  )
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
