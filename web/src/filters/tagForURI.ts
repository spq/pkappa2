import { tagNameForURI } from "@/filters/tagNameForURI";

export function tagForURI (tagId: string) {
    const type = tagId.split("/", 1)[0];
    const name = tagNameForURI(tagId.substring(type.length + 1));

    return `${type}:${name}`;
}
