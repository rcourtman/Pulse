/** Read a reported backup instant without inventing a date or browser timezone. */
export function parseBackupTimestampMs(value: string | undefined): number | undefined {
  if (!value) return undefined;
  // The API's time.Time values carry an explicit RFC3339 zone. Date.parse
  // alone also accepts browser-local dates and normalises impossible dates
  // (September 31 becomes October 1). Neither is evidence for age or order.
  const parts =
    /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})(?:\.\d{1,9})?(Z|[+-]\d{2}:\d{2})$/.exec(
      value,
    );
  if (!parts) return undefined;
  const [, yearText, monthText, dayText, hourText, minuteText, secondText, zone] = parts;
  const [year, month, day, hour, minute, second] = [
    yearText,
    monthText,
    dayText,
    hourText,
    minuteText,
    secondText,
  ].map(Number);
  const leapYear = year % 4 === 0 && (year % 100 !== 0 || year % 400 === 0);
  const daysInMonth = [31, leapYear ? 29 : 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31];
  if (
    month < 1 ||
    month > 12 ||
    day < 1 ||
    day > daysInMonth[month - 1] ||
    hour > 23 ||
    minute > 59 ||
    second > 59 ||
    (zone !== 'Z' && (Number(zone.slice(1, 3)) > 23 || Number(zone.slice(4)) > 59))
  ) {
    return undefined;
  }
  const ms = Date.parse(value);
  return Number.isFinite(ms) ? ms : undefined;
}
