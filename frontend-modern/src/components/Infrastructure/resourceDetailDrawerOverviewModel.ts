import {
  compactDetailSections,
  type DetailRow,
  type DetailSection,
} from '@/components/shared/detailSectionModel';
import { partitionDrawerExceptionRows } from '@/components/Workloads/drawerOverviewModel';

// A platform section that is the object's own content (a pod's containers, a
// pool's health, a disk's SMART readings) stays visible on the Overview, the
// way filesystems and disks do in the Proxmox drawers. Every other section
// waits under the closed Details disclosure.
const CONTENT_CARD_TEST_ID_PREFIX = 'resource-drawer-card-';

export const drawerContentCardTestId = (name: string): string =>
  `${CONTENT_CARD_TEST_ID_PREFIX}${name}`;

export const isDrawerContentCard = (section: DetailSection): boolean =>
  section.testId?.startsWith(CONTENT_CARD_TEST_ID_PREFIX) === true;

const rowKey = (row: DetailRow): string => `${row.label}\u0000${row.value}`;

const sentenceKey = (value: string): string =>
  value
    .trim()
    .toLowerCase()
    .replace(/[.\s]+$/, '');

// The alerts block above already states these sentences (an alert message, a
// pool's own risk summary). A lifted row repeating one of them, whole or as one
// sentence of a longer message, says nothing new.
const isStatedBy = (row: DetailRow, statedMessages: readonly string[]): boolean => {
  const key = sentenceKey(row.value);
  if (!key) return false;
  const sentence = `. ${key}. `;
  return statedMessages.some((message) => `. ${sentenceKey(message)}. `.includes(sentence));
};

export interface ResourceDrawerOverviewInventory {
  /** Warning, danger and actionable rows for "Worth checking". */
  lifted: DetailRow[];
  /** Content cards shown in full on the Overview. */
  cards: DetailSection[];
  /** Everything else, for the closed Details disclosure. */
  details: DetailSection[];
}

/**
 * Splits a resource drawer's inventory (the shared summary sections and the
 * platform's own sections) the way the Proxmox drawers do: anything wrong is
 * lifted in front once, content cards stay visible in full, and the rest goes
 * under Details. A row an open alert already states (by label or by message)
 * is not lifted again.
 */
export const buildResourceDrawerOverviewInventory = (options: {
  summarySections: readonly DetailSection[];
  platformSections: readonly DetailSection[];
  statedRowLabels?: ReadonlySet<string>;
  statedMessages?: readonly string[];
}): ResourceDrawerOverviewInventory => {
  const sections = [...options.summarySections, ...options.platformSections];
  const statedMessages = options.statedMessages ?? [];
  const lifted = partitionDrawerExceptionRows(sections, options.statedRowLabels).lifted.filter(
    (row) => !isStatedBy(row, statedMessages),
  );
  // Lifted rows move rather than repeat. A row left in front only because an
  // alert states it stays in Details, where the record is complete.
  const liftedKeys = new Set(lifted.map(rowKey));
  return {
    lifted,
    // A content card keeps every row: it is the object's whole reading, as a
    // full filesystem stays in the guest drawer's Filesystems card.
    cards: compactDetailSections(sections.filter(isDrawerContentCard)),
    details: compactDetailSections(
      sections
        .filter((section) => !isDrawerContentCard(section))
        .map((section) => ({
          ...section,
          rows: section.rows.filter((row) => !liftedKeys.has(rowKey(row))),
        })),
    ),
  };
};
