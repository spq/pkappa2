<template>
    <v-dialog v-model="visible" width="500" @keypress.enter="submitTagConverters">
      <v-form>
        <v-card>
          <v-card-title>
            <span class="text-h5"
              >Select converters for {{ tagName }}</span
            >
          </v-card-title>
          <v-card-text>
            Selected converters will be executed on streams matching the tag query.
            Converters can transform the stream data, i.e. make websockets readable.
            The original stream data will not be overridden and stays available.
            To create a converter, please read <a href="https://github.com/spq/pkappa2/converters/pkappa2lib/README.md">converters/pkappa2lib/README.md</a>.
            Then you can search in and view converter results.
          </v-card-text>
          <v-card-text>
            <v-checkbox
              v-for="converter in converters"
              :key="converter"
              :label="converter"
              :value="converter"
              v-model="checkedConverters"
            ></v-checkbox>
          </v-card-text>
          <v-card-actions>
            <v-spacer></v-spacer>
            <v-btn text @click="visible = false">Cancel</v-btn>
            <v-btn
              text
              @click="submitTagConverters"
              :disabled="loading"
              :loading="loading"
              :color="error ? 'error' : 'primary'"
              >Save</v-btn
            >
          </v-card-actions>
        </v-card>
      </v-form>
    </v-dialog>
  </template>
  
  <script>
  import { mapActions, mapState } from "vuex";
  import { EventBus } from "./EventBus";
  
  export default {
    name: "TagSetConverterDialog",
    data() {
      return {
        visible: false,
        loading: false,
        error: false,
        tagType: "",
        tagName: "",
        tagId: null,
        checkedConverters: [],
      };
    },
    computed: {
      ...mapState(['tags', 'converters']),
      tag() {
        return this.tags.find((tag) => tag.Name === this.tagId);
      },
    },
    created() {
      EventBus.$on("showTagSetConvertersDialog", this.openDialog);
    },
    methods: {
      ...mapActions(["setTagConverters"]),
      openDialog({ tagId }) {
        this.tagId = tagId;
        this.tagType = tagId.split("/", 1)[0];
        this.tagName = tagId.substr(this.tagType.length + 1);
        this.visible = true;
        this.loading = false;
        this.error = false;
        this.getConvertersFromTag();
      },
      getConvertersFromTag() {
        this.checkedConverters = this.tag.Converters.concat()
          .filter((converter) => converter !== ""); // TODO: REMOVE FIX FOR BACKEND
        console.log(this.checkedConverters, this.tag);
      },
      submitTagConverters() {
        this.loading = true;
        this.error = false;
        this.setTagConverters({name: this.tagId, converters: this.checkedConverters})
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