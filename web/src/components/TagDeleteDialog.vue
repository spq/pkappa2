<template>
  <v-dialog v-model="visible" width="500" @keydown="deleteTag">
    <v-form>
      <v-card>
        <v-card-title>
          <span class="text-h5"
            >Confirm {{ tagType | capitalize }} deletion</span
          >
        </v-card-title>
        <v-card-text>
          Do you want to delete the {{ tagType | capitalize }}
          <code>{{ tagName }}</code
          >?
        </v-card-text>
        <v-card-actions>
          <v-spacer></v-spacer>
          <v-btn text @click="visible = false">No</v-btn>
          <v-btn
            text
            @click="deleteTag"
            :disabled="loading"
            :loading="loading"
            :color="error ? 'error' : 'primary'"
            >Yes</v-btn
          >
        </v-card-actions>
      </v-card>
    </v-form>
  </v-dialog>
</template>

<script>
import { mapActions } from "vuex";
import { EventBus } from "./EventBus";

export default {
  name: "TagDeleteDialog",
  data() {
    return {
      visible: false,
      loading: false,
      error: false,
      tagType: "",
      tagName: "",
    };
  },
  created() {
    EventBus.$on("showTagDeleteDialog", this.openDialog);
  },
  methods: {
    ...mapActions(["delTag"]),
    openDialog({ tagId }) {
      this.tagId = tagId;
      this.tagType = tagId.split("/", 1)[0];
      this.tagName = tagId.substr(this.tagType.length + 1);
      this.visible = true;
      this.loading = false;
      this.error = false;
    },
    deleteTag() {
      this.loading = true;
      this.error = false;
      this.delTag(this.tagId)
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
