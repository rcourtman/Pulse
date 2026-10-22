import { makeDetailRow, type DetailRow } from '@/components/shared/detailSectionModel';
import { asTrimmedString } from '@/utils/stringUtils';
import {
  formatPlatformTableDateTimeValue,
  formatPlatformTableRelativeTimeValue,
} from './sharedPlatformPage';

/**
 * A drawer row for a reported timestamp: "Oct 2, 2026, 09:13 PM (1h ago)".
 * The absolute time is what an operator compares with logs or CLI output; the
 * age says whether it still matters. Table rows only have room for the age, so
 * drawers carry both, wrapping because a phone cell is too narrow for one
 * line. `now` comes from the shared relative-time clock so an open drawer's
 * ages keep moving. A stamp that does not parse reads as reported; with
 * `minYear`, an earlier stamp is a provider's unset sentinel (Docker's
 * 0001-01-01) and the row is omitted.
 */
export const makeDetailTimestampRow = (
  label: string,
  timestamp: string | null | undefined,
  now: number | undefined,
  options: { minYear?: number } = {},
): DetailRow | null => {
  const raw = asTrimmedString(timestamp);
  if (!raw) return null;
  const absolute = formatPlatformTableDateTimeValue(raw, {
    emptyText: '',
    minYear: options.minYear,
    dateTimeFormat: { year: 'numeric' },
  });
  if (!absolute) {
    return options.minYear === undefined ? makeDetailRow(label, raw, { wrap: true }) : null;
  }
  const relative = formatPlatformTableRelativeTimeValue(raw, { emptyText: '', now });
  return makeDetailRow(label, relative ? `${absolute} (${relative})` : absolute, {
    title: raw,
    wrap: true,
  });
};
