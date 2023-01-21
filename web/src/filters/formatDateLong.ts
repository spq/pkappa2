import moment from "moment/moment";

export function formatDateLong(time: null|Date|number|string) {
    if (time === null) {
        return null;
    }
    const localTime = moment(time).local();

    return localTime.format('YYYY-MM-DD HH:mm:ss.SSS ZZ');
}
