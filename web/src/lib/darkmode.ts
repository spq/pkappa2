import { ThemeInstance } from "vuetify";
import { isColorSchemeConfiguration } from "./darkmode.guard";

const STORAGE_KEY = "colorScheme";

export type ColorScheme = "dark" | "light";
/** @see {isColorSchemeConfiguration} ts-auto-guard:type-guard */
export type ColorSchemeConfiguration = ColorScheme | "system";
type ColorSchemeChangeCallback = (colorScheme: ColorScheme) => void;
const colorSchemeChangeListeners: ColorSchemeChangeCallback[] = [];

let theme: ThemeInstance | null = null;

function updateVuetifyTheme(colorScheme: ColorScheme) {
  if (null === theme) {
    return;
  }
  theme.global.name.value = colorScheme;
}

export function registerVuetifyTheme(themeInstance: ThemeInstance) {
  theme = themeInstance;
  updateVuetifyTheme(getColorScheme());
  onSystemColorSchemeChange(updateVuetifyTheme);
}

export function onSystemColorSchemeChange(callback: ColorSchemeChangeCallback) {
  window
    .matchMedia("(prefers-color-scheme: dark)")
    .addEventListener("change", () => {
      callback(getColorScheme());
    });
}

export function onColorSchemeChange(callback: ColorSchemeChangeCallback) {
  colorSchemeChangeListeners.push(callback);
  window
    .matchMedia("(prefers-color-scheme: dark)")
    .addEventListener("change", () => {
      callback(getColorScheme());
    });
}

export function getSystemColorScheme(): ColorScheme {
  return window.matchMedia("(prefers-color-scheme: dark)").matches
    ? "dark"
    : "light";
}

export function getColorSchemeFromStorage(): ColorSchemeConfiguration {
  const scheme = localStorage.getItem(STORAGE_KEY);
  if (isColorSchemeConfiguration(scheme)) {
    return scheme;
  }
  return "system";
}

export function getColorScheme(): ColorScheme {
  const scheme = getColorSchemeFromStorage();
  if (scheme === "system") {
    return getSystemColorScheme();
  }
  return scheme;
}

export function setColorScheme(scheme: ColorSchemeConfiguration) {
  if (!["dark", "light", "system"].includes(scheme)) {
    throw new Error(`Invalid color scheme: ${scheme}`);
  }
  localStorage.setItem(STORAGE_KEY, scheme);
  const activeColorScheme = getColorScheme();
  colorSchemeChangeListeners.forEach((callback) => callback(activeColorScheme));
  updateVuetifyTheme(activeColorScheme);
}
