<template>
  <v-dialog v-model="visible" width="500">
    <v-card>
      <v-card-title>
        <span class="text-h5"
          >{{ capitalize(tagType) }} <code>{{ tagName }}</code> details</span
        >
      </v-card-title>
      <v-card-text v-if="tag != null">
        <v-container>
          <v-row no-gutters>
            <v-col class="text-caption">Matching Streams:</v-col>
            <v-col class="text-caption">Uncertain Streams:</v-col>
            <v-col class="text-caption">Is Referenced:</v-col>
          </v-row>
          <v-row no-gutters>
            <v-col>{{ tag.MatchingCount }}</v-col>
            <v-col>{{ tag.UncertainCount }}</v-col>
            <v-col>{{ tag.Referenced ? "Yes" : "No" }}</v-col>
          </v-row>
          <v-row><v-col></v-col></v-row>
          <v-row no-gutters>
            <v-col cols="4" class="text-caption">Color:</v-col>
            <v-col cols="8"
              ><v-chip
                small
                :color="tag.Color"
                :text-color="getContrastTextColor(tag.Color)"
                >{{ tag.Color }}</v-chip
              ></v-col
            >
          </v-row>
          <v-row><v-col></v-col></v-row>
          <v-row no-gutters>
            <v-col class="text-caption">Definition:</v-col>
          </v-row>
          <v-row no-gutters>
            <v-col
              ><code>{{ tag.Definition }}</code></v-col
            >
          </v-row>
        </v-container>
      </v-card-text>
      <v-card-actions>
        <v-spacer></v-spacer>
        <v-btn text @click="visible = false">Close</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script lang="ts" setup>
import { computed, ref } from "vue";
import { useRootStore } from "@/stores";
import { EventBus } from "./EventBus";
import { capitalize } from "@/filters";
import { getContrastTextColor } from "@/lib/colors";

const store = useRootStore();
const visible = ref(false);
const tagId = ref<string>("");
const tagType = ref<string>("");
const tagName = ref<string>("");
const tag = computed(() => {
  if (store.tags == null) return null;
  return store.tags.find((t) => t.Name === tagId.value);
});

EventBus.on("showTagDetailsDialog", openDialog);

function openDialog(tagIdValue: string) {
  tagId.value = tagIdValue;
  tagType.value = tagIdValue.split("/")[0];
  tagName.value = tagIdValue.substr(tagType.value.length + 1);
  visible.value = true;
}
</script>
