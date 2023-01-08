<template>
  <v-app>
    <v-app-bar
      app
      clipped-left
      color="white"
      flat
      style="border-bottom: 1px solid #eee !important"
    >
      <v-app-bar-nav-icon
        @click="showNavigation = !showNavigation"
      ></v-app-bar-nav-icon>
      <v-responsive>
        <SearchBox></SearchBox>
      </v-responsive>
    </v-app-bar>
    <v-navigation-drawer app clipped v-model="showNavigation">
      <Navigation></Navigation>
    </v-navigation-drawer>
    <v-main>
      <router-view></router-view>
    </v-main>
    <TagDeleteDialog />
    <TagCreateDialog />
    <TagDetailsDialog />
    <TagColorChangeDialog />
    <ErrorBanner />
  </v-app>
</template>

<script>
import Navigation from "./Navigation.vue";
import SearchBox from "./SearchBox.vue";
import TagDeleteDialog from "./TagDeleteDialog.vue";
import TagCreateDialog from "./TagCreateDialog.vue";
import TagDetailsDialog from "./TagDetailsDialog.vue";
import TagColorChangeDialog from "./TagColorChangeDialog.vue";
import ErrorBanner from "./ErrorBanner.vue";

export default {
  name: "Base",
  components: {
    Navigation,
    SearchBox,
    TagDeleteDialog,
    TagCreateDialog,
    TagDetailsDialog,
    TagColorChangeDialog,
    ErrorBanner,
  },
  data() {
    return {
      showNavigation: true,
    };
  },
};
</script>