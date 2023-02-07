<template>
  <v-simple-table dense>
    <thead>
      <tr>
        <th class="text-left">Name</th>
        <th class="text-left">Cached Stream Count</th>
        <th colspan="2" class="text-left">Processes</th>
      </tr>
    </thead>
    <tbody>
      <template v-for="converter in converters">
        <tr :key="converter.Name">
          <td>
            {{ converter.Name }}
          </td>
          <td>
            {{ converter.CachedStreamCount }}
          </td>
          <td>
            {{ converter.Processes }}
          </td>
          <td style="text-align: right">
            <v-tooltip bottom>
              <template #activator="{ on, attrs }">
                <v-btn
                  v-bind="attrs"
                  v-on="on"
                  icon
                  @click="confirmConverterReset(converter)"
                  ><v-icon>mdi-restart-alert</v-icon></v-btn
                >
              </template>
              <span>Reset Converter</span>
            </v-tooltip>
          </td>
        </tr>
      </template>
    </tbody>
  </v-simple-table>
</template>

<script>
import { mapActions, mapState } from "vuex";
import { EventBus } from "./EventBus";

export default {
  name: "Converters",
  mounted() {
    this.updateTags();
    this.updateConverters();
  },
  computed: {
    ...mapState(["tags", "converters"]),
  },
  methods: {
    ...mapActions(["updateTags", "updateConverters"]),

    confirmConverterReset(converter) {
      EventBus.$emit("showConverterResetDialog", { converter });
    },
  },
};
</script>
