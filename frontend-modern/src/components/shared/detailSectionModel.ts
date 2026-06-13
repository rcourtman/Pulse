import { formatBytes } from '@/utils/format';

export type DetailValueTone = 'default' | 'accent' | 'success' | 'warning' | 'danger' | 'muted';

export type DetailRow = {
  label: string;
  value: string;
  title?: string;
  tone?: DetailValueTone;
};

export type DetailSection = {
  label: string;
  rows: DetailRow[];
};

export const makeDetailRow = (
  label: string,
  value?: string | null,
  options: Pick<DetailRow, 'title' | 'tone'> = {},
): DetailRow | null => {
  const trimmed = value?.trim();
  if (!trimmed || trimmed === '-') return null;
  return { label, value: trimmed, ...options };
};

export const compactDetailRows = (rows: Array<DetailRow | null>): DetailRow[] =>
  rows.filter((row): row is DetailRow => Boolean(row));

export const compactDetailSections = (sections: Array<DetailSection | null>): DetailSection[] =>
  sections.filter((section): section is DetailSection =>
    Boolean(section && section.rows.length > 0),
  );

export type DetailBytesFormatOptions = {
  allowZero?: boolean;
  precision?: 'auto' | 'compact';
  trimWhole?: boolean;
};

const isFiniteNumber = (value: number | undefined): value is number =>
  typeof value === 'number' && Number.isFinite(value);

const formatCompactBytes = (bytes: number): string => {
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
  let scaled = bytes;
  let unitIndex = 0;
  while (scaled >= 1024 && unitIndex < units.length - 1) {
    scaled /= 1024;
    unitIndex += 1;
  }
  const precision = unitIndex === 0 || scaled >= 10 ? 0 : 1;
  return `${scaled.toFixed(precision)} ${units[unitIndex]}`;
};

const trimWholeUnit = (value: string): string => value.replace(/\.0+ ([A-Z]+)/, ' $1');

export const formatDetailBytesValue = (
  bytes: number | undefined,
  options: DetailBytesFormatOptions = {},
): string | null => {
  if (!isFiniteNumber(bytes) || bytes < 0) return null;
  if (bytes === 0 && options.allowZero !== true) return null;

  const formatted =
    options.precision === 'compact' ? formatCompactBytes(bytes) : formatBytes(bytes);
  return options.trimWhole ? trimWholeUnit(formatted) : formatted;
};

export const formatDetailIntegerValue = (value: number | undefined): string | null => {
  if (!isFiniteNumber(value)) return null;
  return new Intl.NumberFormat().format(Math.round(value));
};

export function formatDetailCountValue(value: number, singular: string, plural?: string): string;
export function formatDetailCountValue(
  value: number | undefined,
  singular: string,
  plural?: string,
): string | null;
export function formatDetailCountValue(
  value: number | undefined,
  singular: string,
  plural = `${singular}s`,
): string | null {
  if (!isFiniteNumber(value)) return null;
  return `${formatDetailIntegerValue(value)} ${value === 1 ? singular : plural}`;
}
