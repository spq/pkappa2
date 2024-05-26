/*
 * Generated type guards for "darkmode.ts".
 * WARNING: Do not manually change this file.
 */
import { ColorSchemeConfiguration } from "./darkmode";

export function isColorSchemeConfiguration(obj: unknown): obj is ColorSchemeConfiguration {
    const typedObj = obj as ColorSchemeConfiguration
    return (
        (typedObj === "dark" ||
            typedObj === "light" ||
            typedObj === "system")
    )
}
