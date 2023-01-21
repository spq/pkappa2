export function capitalize(value: unknown) {
    if (typeof value !== "string") {
        return '';
    }

    return value.charAt(0).toUpperCase() + value.slice(1);
}
