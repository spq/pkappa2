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
        v-slot:[`item.${field}`]="{ index, value }"
        ><span :title="formatDateLong(value)" :key="`${field}/${index}`">{{
          formatDate(value)
        }}</span></template
      >
    </v-data-table>
  </div>
</template>

<script>
import { mapActions, mapState } from "vuex";
import {formatDateLong} from "@/filters/formatDateLong";
import {formatDate} from "../filters/formatDate";

export default {
  name: "Pcaps",
  data() {
    return {
      headers: [
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
      ],
    };
  },
  mounted() {
    this.updatePcaps();
  },
  computed: {
    ...mapState(["pcaps"]),
    pcapsPretty() {
      if (this.pcaps == null) return [];
      return this.pcaps.map((i) => {
        const res = { ...i };
        for (const k of [
          "ParseTime",
          "PacketTimestampMin",
          "PacketTimestampMax",
        ]) {
          let v = new Date(res[k]);
          if (v < 0) v = null;
          res[k] = v;
        }
        return res;
      });
    },
  },
  methods: {
      formatDate,
      formatDateLong
    ...mapActions(["updatePcaps"]),
  },
};
</script>
