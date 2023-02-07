<template>
  <v-dialog v-model="visible" width="500" @keydown.enter="updateColor">
    <v-form>
      <v-card>
        <v-card-title>
          <span class="text-h5"
            >Change Color of {{ tagType | capitalize }}
            <v-chip :color="tagColor">{{ tagName }}</v-chip></span
          >
        </v-card-title>
        <v-card-text>
          <v-text-field v-model="tagColor" label="Color" hide-details>
            <template #append>
              <v-menu
                v-model="colorPickerOpen"
                top
                nudge-bottom="182"
                nudge-left="32"
                :close-on-content-click="false"
              >
                <template #activator="{ on }">
                  <div :style="swatchStyle" v-on="on" />
                </template>
                <v-card>
                  <v-card-text>
                    <v-color-picker
                      v-model="colorPickerValue"
                      mode="hexa"
                      hide-mode-switch
                      hide-inputs
                      show-swatches
                      flat
                      @update:color="colorPickerValueUpdate"
                    />
                  </v-card-text>
                </v-card>
              </v-menu>
            </template>
          </v-text-field>
        </v-card-text>
        <v-card-actions>
          <v-spacer></v-spacer>
          <v-btn text @click="visible = false">Cancel</v-btn>
          <v-btn
            text
            :disabled="loading"
            :loading="loading"
            :color="error ? 'error' : 'primary'"
            type="submit"
            @click="updateColor"
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
  name: "TagColorChangeDialog",
  data() {
    return {
      visible: false,
      loading: false,
      error: false,
      tagType: "",
      tagName: "",
      tagColor: "",
      colorPickerOpen: false,
      colorPickerValue: "",
    };
  },
  created() {
    EventBus.$on("showTagColorChangeDialog", this.openDialog);
  },
  computed: {
    ...mapState(["tags"]),
    // https://codepen.io/JamieCurnow/pen/KKPjraK
    swatchStyle() {
      const { tagColor, tagColorPickerOpen } = this;
      return {
        backgroundColor: tagColor,
        cursor: "pointer",
        height: "30px",
        width: "30px",
        borderRadius: tagColorPickerOpen ? "50%" : "4px",
        transition: "border-radius 200ms ease-in-out",
      };
    },
  },
  methods: {
    ...mapActions(["changeTagColor"]),
    openDialog({ tagId }) {
      this.tagId = tagId;
      this.tagType = tagId.split("/", 1)[0];
      this.tagName = tagId.substr(this.tagType.length + 1);
      this.tagColor = this.tags.filter((e) => e.Name == tagId)[0].Color;
      this.colorPickerOpen = false;
      this.visible = true;
      this.loading = false;
      this.error = false;
    },
    colorPickerValueUpdate(color) {
      if (this.colorPickerOpen) this.tagColor = color.hex;
    },
    updateColor() {
      this.loading = true;
      this.error = false;
      this.changeTagColor({ name: this.tagId, color: this.tagColor })
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
  watch: {
    colorPickerOpen(val, old) {
      if (val && !old) this.colorPickerValue = this.tagColor;
    },
  },
};
</script>
