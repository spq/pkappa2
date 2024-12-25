<template>
  <div>
    <v-data-table
      :headers="headers"
      :items="store.pcaps || []"
      :loading="store.pcaps === null"
      :footer-props="{
        itemsPerPageOptions: [20, 50, 100, -1],
        showFirstLastPage: true,
      }"
      density="compact"
    >
      <template #[`item.download`]="{ item }"
        ><v-btn
          variant="plain"
          :href="`/api/download/pcap/${item.Filename}`"
          icon
        >
          <v-icon>mdi-download</v-icon>
        </v-btn></template
      >
      <template
        v-for="field of [
          'ParseTime',
          'PacketTimestampMin',
          'PacketTimestampMax',
        ]"
        #[`item.${field}`]="{ index, value }"
        ><span
          :key="`${field}/${index}`"
          :title="formatDateLong(new Date(value))"
          >{{ formatDate(new Date(value)) }}</span
        ></template
      >
      <template #[`item.Filesize`]="{ value }"
        ><span :title="`${value} Bytes`">{{
          prettyBytes(value, { maximumFractionDigits: 1, binary: true })
        }}</span></template
      >
    </v-data-table>
  </div>
</template>

<script lang="ts" setup>
import { onMounted } from "vue";
import { useRootStore } from "@/stores";
import { EventBus } from "./EventBus";
import { formatDate, formatDateLong } from "@/filters";
import prettyBytes from "pretty-bytes";

const store = useRootStore();
const headers = [
  {
    title: "File Name",
    value: "Filename",
  },
  {
    title: "First Packet Time",
    value: "PacketTimestampMin",
  },
  {
    title: "Last Packet Time",
    value: "PacketTimestampMax",
  },
  {
    title: "Packet Count",
    value: "PacketCount",
  },
  {
    title: "File Size",
    value: "Filesize",
  },
  {
    title: "Parse Time",
    value: "ParseTime",
    align: "end",
    class: "pr-0",
    cellClass: "pr-0",
  },
  {
    title: "",
    value: "download",
    sortable: false,
    class: ["px-0", "w0"],
    cellClass: ["px-0", "w0"],
  },
];

onMounted(() => {
  store.updatePcaps().catch((err: Error) => {
    EventBus.emit("showError", `Failed to update pcaps: ${err.message}`);
  });
});
</script>

<style scoped>
.w0 {
  width: 0;
}
</style>
