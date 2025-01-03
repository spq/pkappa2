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
    <router-view></router-view>
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
import { ref } from "vue";
import { useTheme } from "vuetify";
import { registerVuetifyTheme } from "@/lib/darkmode";

/** @TODO: Only show navigation on larger screens.
 * The navigation overlays on smaller screens.
 * An overlay on first site visit is very disrupting. */
const showNavigation = ref(true);
registerVuetifyTheme(useTheme());
</script>
