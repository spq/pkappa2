<template>
  <div>
    <v-app-bar elevation="0" style="border-bottom: 1px solid #eee !important">
      <template #prepend>
        <v-app-bar-nav-icon
          @click="showNavigation = !showNavigation"
        ></v-app-bar-nav-icon>
      </template>
      <v-responsive>
        <SearchBox></SearchBox>
      </v-responsive>
    </v-app-bar>
    <v-navigation-drawer v-model="showNavigation">
      <Navigation></Navigation>
    </v-navigation-drawer>
    <v-main>
      <router-view></router-view>
    </v-main>
    <CTFWizardDialog />
    <ConverterResetDialog />
    <TagDeleteDialog />
    <TagCreateDialog />
    <TagSetConverterDialog />
    <TagDetailsDialog />
    <TagColorChangeDialog />
    <TagDefinitionChangeDialog />
    <TagNameChangeDialog />
    <ErrorBanner />
  </div>
</template>

<script lang="ts" setup>
import Navigation from "./Navigation.vue";
import SearchBox from "./SearchBox.vue";
import CTFWizardDialog from "./CTFWizardDialog.vue";
import ConverterResetDialog from "./ConverterResetDialog.vue";
import TagDeleteDialog from "./TagDeleteDialog.vue";
import TagSetConverterDialog from "./TagSetConverterDialog.vue";
import TagCreateDialog from "./TagCreateDialog.vue";
import TagDetailsDialog from "./TagDetailsDialog.vue";
import TagColorChangeDialog from "./TagColorChangeDialog.vue";
import TagDefinitionChangeDialog from "./TagDefinitionChangeDialog.vue";
import TagNameChangeDialog from "./TagNameChangeDialog.vue";
import ErrorBanner from "./ErrorBanner.vue";
import { ref } from "vue";
import { useTheme } from "vuetify";
import { registerVuetifyTheme } from "@/lib/darkmode";

/** @TODO: Only show navigation on larger screens.
 * The navigation overlays on smaller screens.
 * An overlay on first site visit is very disrupting. */
const showNavigation = ref(true);
registerVuetifyTheme(useTheme());
</script>
