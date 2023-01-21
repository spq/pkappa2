export type TagPart = 'id'|'type'|'name';

export function tagify(id: string, tagPart: TagPart) {
    const type = id.split("/", 1)[0];
    const name = id.substring(type.length + 1);
    return {id, type, name}[tagPart];
}
