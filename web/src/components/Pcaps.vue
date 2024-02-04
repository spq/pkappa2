<template>
  <div>
    <v-data-table
      :headers="headers"
      :items="pcapsPretty"
      :loading="pcaps === null"
      disable-pagination
      hide-default-footer
      dense
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
          :title="$options.filters?.formatDateLong(value)"
          >{{ $options.filters?.formatDate(value) }}</span
        ></template
      >
      <template #[`item.Filesize`]="{ value }"
        ><span :title="`${value} Bytes`">{{
          $options.filters?.prettyBytes(value, 1, true)
        }}</span></template
      >
    </v-data-table>
  </div>
</template>

<script lang="ts" setup>
import { computed, onMounted } from "vue";
import { useStore } from "@/store";
import { EventBus } from "./EventBus";
import { PcapInfo } from "@/apiClient";

const store = useStore();
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
  },
];
const pcaps = computed(() => store.state.pcaps);
const pcapsPretty = computed(() => {
  if (pcaps.value == null) return [];
  return pcaps.value.map((i) => {
    const res: { [key: string]: string | number | Date } = { ...i };
    for (const k of ["ParseTime", "PacketTimestampMin", "PacketTimestampMax"]) {
      res[k] = new Date(res[k]);
    }
    return res as PcapInfo;
  });
});

onMounted(() => {
  store.dispatch("updatePcaps").catch((err: string) => {
    EventBus.emit("showError", `Failed to update pcaps: ${err}`);
  });
});
</script>
