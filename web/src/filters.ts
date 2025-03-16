import moment from "moment";

export function capitalize(value: string | null) {
  if (!value) return "";
  value = value.toString();
  return value.charAt(0).toUpperCase() + value.slice(1);
}

export function tagify(id: string, what: "id" | "type" | "name") {
  const type = id.split("/", 1)[0];
  const name = id.substring(type.length + 1);
  return { id, type, name }[what];
}

export function formatDuration(seconds: number) {
  return moment.duration(seconds, "seconds").humanize();
}

export function formatDate(time: string | Date | null) {
  if (time === null) return undefined;
  const date = moment(time).local();
  let format = "HH:mm:ss.SSS";
  if (!date.isSame(moment(), "day")) format = `YYYY-MM-DD ${format}`;
  return date.format(format);
}

export function formatDateLong(time: string | Date | null) {
  if (time === null) return undefined;
  const date = moment(time).local();
  return date.format("YYYY-MM-DD HH:mm:ss.SSS ZZ");
}

export function tagForURI(tagId: string) {
  const type = tagId.split("/", 1)[0];
  const name = tagNameForURI(tagId.substring(type.length + 1));
  return `${type}:${name}`;
}

export function tagNameForURI(tagName: string) {
  if (tagName.includes('"')) {
    tagName = tagName.replaceAll('"', '""');
  }
  if (/[ "]/.test(tagName)) {
    tagName = `"${tagName}"`;
  }

  return tagName;
}
