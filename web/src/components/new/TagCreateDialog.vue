<template>
  <v-dialog v-model="visible" width="500" @keydown.enter="createTag">
    <v-form>
      <v-card>
        <v-card-title>
          <span class="text-h5">Create {{ tagType | capitalize }}</span>
        </v-card-title>
        <v-card-text>
          <v-text-field v-model="tagName" label="Name" autofocus></v-text-field>
          <v-text-field v-model="tagColor" label="Color" hide-details>
            <template v-slot:append>
              <v-menu v-model="tagColorPickerOpen" top nudge-bottom="270" nudge-left="32" :close-on-content-click="false">
                <template v-slot:activator="{ on }">
                  <div :style="swatchStyle" v-on="on" />
                </template>
                <v-card>
                  <v-card-text>
                    <v-color-picker v-model="tagColor" mode="hexa" hide-mode-switch hide-inputs show-swatches flat />
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
      tagColor: "",
      tagColorPickerOpen: false,
    };
  },
  created() {
    EventBus.$on("showCreateTagDialog", this.openDialog);
  },
  computed: {
    // https://codepen.io/JamieCurnow/pen/KKPjraK
    swatchStyle() {
      const { tagColor, tagColorPickerOpen } = this
      return {
        backgroundColor: tagColor,
        cursor: 'pointer',
        height: '30px',
        width: '30px',
        borderRadius: tagColorPickerOpen ? '50%' : '4px',
        transition: 'border-radius 200ms ease-in-out'
      }
    }
  },
  methods: {
    ...mapActions(["addTag","markTagNew"]),
    openDialog({ tagType, tagQuery, tagStreams }) {
      this.tagType = tagType;
      this.tagQuery = tagQuery;
      this.tagStreams = tagStreams;
      this.tagName = "";
      this.tagColor = this.randomColor();
      this.tagColorPickerOpen = false;
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
            color: this.tagColor,
          })
        : this.addTag({
            name: `${this.tagType}/${this.tagName}`,
            query: this.tagQuery,
            color: this.tagColor,
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
    // https://stackoverflow.com/a/17243070
    randomColor() {
      const h = Math.random(), s = 0.6, v = 1.0;
      var r, g, b, i, f, p, q, t;
      i = Math.floor(h * 6);
      f = h * 6 - i;
      p = v * (1 - s);
      q = v * (1 - f * s);
      t = v * (1 - (1 - f) * s);
      switch (i % 6) {
          case 0: r = v, g = t, b = p; break;
          case 1: r = q, g = v, b = p; break;
          case 2: r = p, g = v, b = t; break;
          case 3: r = p, g = q, b = v; break;
          case 4: r = t, g = p, b = v; break;
          case 5: r = v, g = p, b = q; break;
      }
      const toHex = (i) => Math.round(i * 255).toString(16).padStart(2, '0');
      return '#' + toHex(r) + toHex(g) + toHex(b);
    },
  },
};
</script>