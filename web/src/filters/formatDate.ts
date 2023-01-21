import moment from "moment/moment";

export function formatDate(time: Date|string|number|null) {
    if (time === null) {
        return null;
    }
    const localTime = moment(time).local();
    let format = "HH:mm:ss.SSS";
    if (!localTime.isSame(moment(), "day")) {
        format = `YYYY-MM-DD ${format}`;
    }

    return localTime.format(format);
}
