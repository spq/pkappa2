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
      dense
    >
      <template #[`item.download`]="{ item }"
        ><v-btn
          :href="`/api/download/pcap/${item.Filename}`"
          icon
          @click.native.stop
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
    text: "File Name",
    value: "Filename",
  },
  {
    text: "First Packet Time",
    value: "PacketTimestampMin",
  },
  {
    text: "Last Packet Time",
    value: "PacketTimestampMax",
  },
  {
    text: "Packet Count",
    value: "PacketCount",
  },
  {
    text: "File Size",
    value: "Filesize",
  },
  {
    text: "Parse Time",
    value: "ParseTime",
    align: "end",
    class: "pr-0",
    cellClass: "pr-0",
  },
  {
    text: "",
    value: "download",
    sortable: false,
    class: ["px-0", "w0"],
    cellClass: ["px-0", "w0"],
  },
];

onMounted(() => {
  store.updatePcaps().catch((err: string) => {
    EventBus.emit("showError", `Failed to update pcaps: ${err}`);
  });
});
</script>

<style scoped>
.w0 {
  width: 0;
}
</style>
