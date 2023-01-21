export function tagNameForURI (tagName: string) {
    if (tagName.includes('"')) {
        tagName = tagName.replaceAll('"', '""');
    }
    if (/[ "]/.test(tagName)) {
        tagName = `"${tagName}"`;
    }

    return tagName;
}
