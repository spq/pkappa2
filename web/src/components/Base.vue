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
    <ConverterResetDialog />
    <TagDeleteDialog />
    <TagCreateDialog />
    <TagSetConverterDialog />
    <TagDetailsDialog />
    <TagColorChangeDialog />
    <ErrorBanner />
  </v-app>
</template>

<script>
import Navigation from "./Navigation.vue";
import SearchBox from "./SearchBox.vue";
import ConverterResetDialog from "./ConverterResetDialog.vue";
import TagDeleteDialog from "./TagDeleteDialog.vue";
import TagSetConverterDialog from "./TagSetConverterDialog.vue";
import TagCreateDialog from "./TagCreateDialog.vue";
import TagDetailsDialog from "./TagDetailsDialog.vue";
import TagColorChangeDialog from "./TagColorChangeDialog.vue";
import ErrorBanner from "./ErrorBanner.vue";

export default {
  name: "Base",
  components: {
    Navigation,
    SearchBox,
    ConverterResetDialog,
    TagDeleteDialog,
    TagCreateDialog,
    TagSetConverterDialog,
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