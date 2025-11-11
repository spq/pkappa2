import moment from "moment";

// Cache moment objects for reoccuring dates.
// The moment() constructor showed up pretty high up in the
// performance profiler while rendering a large stream with
// lots of chunks.
const momentCache: Record<string, moment.Moment> = {};
function getMoment(time: string | Date): moment.Moment {
  if (typeof time === "string" && time in momentCache) {
    return momentCache[time];
  }
  const m = moment(time);
  if (typeof time === "string") {
    momentCache[time] = m;
  }
  return m;
}

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

export function formatDateDifference(
  first: string | Date,
  second: string | Date | undefined,
) {
  if (second === undefined) return "0 ms";
  if (first === second) return "0 ms";
  const ms = getMoment(first).diff(getMoment(second));
  if (ms < 1000) return `${ms} ms`;
  const seconds = ms / 1000;
  if (seconds < 60) {
    return `${seconds} s`;
  }
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) {
    return `${minutes} m ${(seconds % 60).toFixed(3)} s`;
  }
  const hours = Math.floor(minutes / 60);
  return `${hours} h ${minutes % 60} m ${(seconds % 60).toFixed(3)} s`;
}

export function formatDate(time: string | Date | null) {
  if (time === null) return undefined;
  const date = getMoment(time).local();
  let format = "HH:mm:ss.SSS";
  if (!date.isSame(moment(), "day")) format = `YYYY-MM-DD ${format}`;
  return date.format(format);
}

export function formatDateLong(time: string | Date | null) {
  if (time === null) return undefined;
  const date = getMoment(time).local();
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
