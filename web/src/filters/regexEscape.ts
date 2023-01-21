export function regexEscape (text: string) {
    return text
        .split("")
        .map(char => char.replace(
                /[^ !#$%&',-/0123456789:;<=>ABCDEFGHIJKLMNOPQRSTUVWXYZ^_`abcdefghijklmnopqrstuvwxyz~]/,
                (match) => `\\x{${match.charCodeAt(0).toString(16).toUpperCase().padStart(2, '0')}}`
            )
        )
        .join("");
}
