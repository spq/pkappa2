<template>
  <v-dialog v-model="visible" width="500" @keydown.enter="createTag">
    <v-form>
      <v-card>
        <v-card-title>
          <span class="text-h5">Create {{ tagType | capitalize }}</span>
        </v-card-title>
        <v-card-text>
          <v-text-field v-model="tagName" label="Name" autofocus></v-text-field>
        </v-card-text>
        <v-card-actions>
          <v-spacer></v-spacer>
          <v-btn text @click="visible = false">Cancel</v-btn>
          <v-btn
            text
            @click="createTag"
            :disabled="tagName == '' || loading"
            :loading="loading"
            :color="error ? 'error' : 'primary'"
            type="submit"
            >Create</v-btn
          >
        </v-card-actions>
      </v-card>
    </v-form>
  </v-dialog>
</template>

<script>
import { EventBus } from "./EventBus";
import { mapActions } from "vuex";

export default {
  name: "TagCreateDialog",
  data() {
    return {
      visible: false,
      loading: false,
      error: false,
      tagQuery: "",
      tagStreams: [],
      tagType: "",
      tagName: "",
    };
  },
  created() {
    EventBus.$on("showCreateTagDialog", this.openDialog);
  },
  methods: {
    ...mapActions(["addTag","markTagNew"]),
    openDialog({ tagType, tagQuery, tagStreams }) {
      this.tagType = tagType;
      this.tagQuery = tagQuery;
      this.tagStreams = tagStreams;
      this.tagName = "";
      this.visible = true;
      this.loading = false;
      this.error = false;
    },
    createTag() {
      this.loading = true;
      this.error = false;
      (this.tagType == "mark"
        ? this.markTagNew({
            name: `${this.tagType}/${this.tagName}`,
            streams: this.tagStreams,
          })
        : this.addTag({
            name: `${this.tagType}/${this.tagName}`,
            query: this.tagQuery,
          })
      )
        .then(() => {
          this.visible = false;
        })
        .catch((err) => {
          this.error = true;
          this.loading = false;
          EventBus.$emit("showError", { message: err });
        });
    },
  },
};
</script>