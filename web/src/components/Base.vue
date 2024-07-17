<template>
  <v-app>
    <v-app-bar
      app
      clipped-left
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
    <v-navigation-drawer v-model="showNavigation" app clipped>
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

<script lang="ts" setup>
import Navigation from "./Navigation.vue";
import SearchBox from "./SearchBox.vue";
import ConverterResetDialog from "./ConverterResetDialog.vue";
import TagDeleteDialog from "./TagDeleteDialog.vue";
import TagSetConverterDialog from "./TagSetConverterDialog.vue";
import TagCreateDialog from "./TagCreateDialog.vue";
import TagDetailsDialog from "./TagDetailsDialog.vue";
import TagColorChangeDialog from "./TagColorChangeDialog.vue";
import ErrorBanner from "./ErrorBanner.vue";
import { ref } from "vue";

/** @TODO: Only show navigation on larger screens.
 * The navigation overlays on smaller screens.
 * An overlay on first site visit is very disrupting. */
const showNavigation = ref(true);
</script>
