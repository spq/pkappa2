<template>
  <v-dialog v-model="visible" width="500" @keypress.enter="submitTagConverters">
    <v-form>
      <v-card>
        <v-card-title>
          <span class="text-h5">Select converters for {{ tagName }}</span>
        </v-card-title>
        <v-card-text>
          Selected converters will be executed on streams matching the tag
          query. Converters can transform the stream data, i.e. make websockets
          readable. The original stream data will not be overridden and stays
          available. To create a converter, please read
          <a
            href="https://github.com/spq/pkappa2/converters/pkappa2lib/README.md"
            >converters/pkappa2lib/README.md</a
          >. Then you can search in and view converter results.
        </v-card-text>
        <v-card-text>
          <v-checkbox
            v-for="converter in converters"
            :key="converter.Name"
            v-model="checkedConverters"
            :label="converter.Name"
            :value="converter.Name"
          ></v-checkbox>
        </v-card-text>
        <v-card-actions>
          <v-spacer></v-spacer>
          <v-btn text @click="visible = false">Cancel</v-btn>
          <v-btn
            text
            :disabled="loading"
            :loading="loading"
            :color="error ? 'error' : 'primary'"
            @click="submitTagConverters"
            >Save</v-btn
          >
        </v-card-actions>
      </v-card>
    </v-form>
  </v-dialog>
</template>

<script lang="ts" setup>
import { computed, ref } from "vue";
import { useStore } from "@/store";
import { EventBus } from "./EventBus";

const visible = ref(false);
const loading = ref(false);
const error = ref(false);
const tagType = ref("");
const tagName = ref("");
const tagId = ref<string | null>(null);
const checkedConverters = ref<string[]>([]);

const store = useStore();
const tag = computed(() => {
  if (store.state.tags === null || tagId.value === null) return undefined;
  return store.state.tags.find((tag) => tag.Name === tagId.value);
});
const converters = computed(() => store.state.converters);

EventBus.on("showTagSetConvertersDialog", openDialog);

function openDialog(tagIdValue: string) {
  tagId.value = tagIdValue;
  tagType.value = tagIdValue.split("/")[0];
  tagName.value = tagIdValue.substr(tagType.value.length + 1);
  visible.value = true;
  loading.value = false;
  error.value = false;
  getConvertersFromTag();
}

function getConvertersFromTag() {
  checkedConverters.value = tag.value?.Converters.concat() || [];
}

function submitTagConverters() {
  loading.value = true;
  error.value = false;
  store
    .dispatch("setTagConverters", {
      name: tagId.value,
      converters: checkedConverters.value,
    })
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
