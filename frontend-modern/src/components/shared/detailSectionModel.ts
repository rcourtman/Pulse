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
