export type TrueNASDetailTone = 'default' | 'accent' | 'success' | 'warning' | 'danger' | 'muted';

export type TrueNASDetailRow = {
  label: string;
  value: string;
  title?: string;
  tone?: TrueNASDetailTone;
};

export type TrueNASDetailSection = {
  label: string;
  rows: TrueNASDetailRow[];
};

export const makeTrueNASDetailRow = (
  label: string,
  value?: string | null,
  options: Pick<TrueNASDetailRow, 'title' | 'tone'> = {},
): TrueNASDetailRow | null => {
  const trimmed = value?.trim();
  if (!trimmed || trimmed === '-') return null;
  return { label, value: trimmed, ...options };
};

export const compactTrueNASDetailRows = (
  rows: Array<TrueNASDetailRow | null>,
): TrueNASDetailRow[] => rows.filter((row): row is TrueNASDetailRow => Boolean(row));

export const compactTrueNASDetailSections = (
  sections: Array<TrueNASDetailSection | null>,
): TrueNASDetailSection[] =>
  sections.filter((section): section is TrueNASDetailSection =>
    Boolean(section && section.rows.length > 0),
  );
