const STORAGE_KEY = "colorScheme";

/**
 * @type {Array<Function>}
 */
const colorSchemeChangeListeners = [];

/** @type {import("vuetify").Framework|null} */
let registeredVuetify = null;

export function registerVuetifyTheme(vuetify) {
  registeredVuetify = vuetify;
  vuetify.theme.dark = getColorScheme() === "dark";
  onSystemColorSchemeChange((colorScheme) => {
    vuetify.theme.dark = colorScheme === "dark";
  });
}

export function onSystemColorSchemeChange(callback) {
  window.matchMedia('(prefers-color-scheme: dark)')
    .addEventListener('change', () => {
      callback(getColorScheme());
    });
}

export function onColorSchemeChange(callback) {
  colorSchemeChangeListeners.push(callback);
  window.matchMedia('(prefers-color-scheme: dark)')
    .addEventListener('change', () => {
      callback(getColorScheme());
    });
}

/**
 * @returns {"dark"|"light"}
 */
export function getSystemColorScheme() {
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? "dark" : "light";
}

/**
 * @returns {"dark"|"light"|"system"}
 */
export function getColorSchemeFromStorage() {
  const scheme = localStorage.getItem(STORAGE_KEY);
  if (!['dark', 'light', 'system'].includes(scheme)) {
    return 'system';
  }
  return scheme;
}

/**
 * @returns {"dark"|"light"}
 */
export function getColorScheme() {
  const scheme = getColorSchemeFromStorage();
  if (scheme === 'system') {
    return getSystemColorScheme();
  }
  return scheme;
}

/**
 * @param {"dark"|"light"|"system"} scheme
 */
export function setColorScheme(scheme) {
  if (!['dark', 'light', 'system'].includes(scheme)) {
    throw new Error(`Invalid color scheme: ${scheme}`);
  }
  localStorage.setItem(STORAGE_KEY, scheme);
  colorSchemeChangeListeners.forEach((callback) => callback(scheme));
  if (null !== registeredVuetify) {
    registeredVuetify.theme.dark = getColorScheme() === "dark";
  }
}
