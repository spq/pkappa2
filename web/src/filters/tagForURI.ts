import { tagNameForURI } from "./tagNameForURI";

export function tagForURI(tagId: string) {
  const type = tagId.split("/", 1)[0];
  const name = tagNameForURI(tagId.substr(type.length + 1));
  return `${type}:${name}`;
}
