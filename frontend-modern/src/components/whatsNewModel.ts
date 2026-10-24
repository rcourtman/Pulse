// Model logic for the release preview and post-update changelog.
import { marked } from 'marked';
import { normalizeReleaseVersion } from './updateVersion';

type Heading = {
  index: number;
  level: number;
  title: string;
};

const headingMatch = (line: string) => line.trim().match(/^(#{1,6})\s+(.*)$/);

const headingsIn = (lines: string[]): Heading[] => {
  let fence: string | undefined;
  return lines.flatMap((line, index) => {
    const marker = line.trim().match(/^(`{3,}|~{3,})/);
    if (marker) {
      if (!fence) fence = marker[1];
      else if (marker[1][0] === fence[0] && marker[1].length >= fence.length) fence = undefined;
      return [];
    }
    if (fence) return [];
    const match = headingMatch(line);
    return match ? [{ index, level: match[1].length, title: match[2].trim() }] : [];
  });
};

const sectionBody = (lines: string[], headings: Heading[], headingIndex: number): string => {
  const heading = headings[headingIndex];
  const nextHeading = headings
    .slice(headingIndex + 1)
    .find((candidate) => candidate.level <= heading.level);
  return lines
    .slice(heading.index + 1, nextHeading?.index ?? lines.length)
    .join('\n')
    .trim();
};

/**
 * Extract the compact pre-update summary from a GitHub release body. Current
 * customer-facing notes use `Highlights`; older packets may carry only
 * `What's improved`.
 */
export const extractHighlights = (markdown: string): string | null => {
  const lines = markdown.replace(/\r\n/g, '\n').split('\n');
  const headings = headingsIn(lines);
  const highlightsIndex = headings.findIndex((heading) => /^highlights\b/i.test(heading.title));
  const improvementsIndex = headings.findIndex((heading) =>
    /^what(?:'|’)s improved\b/i.test(heading.title),
  );
  const headingIndex = highlightsIndex === -1 ? improvementsIndex : highlightsIndex;
  if (headingIndex === -1) {
    return null;
  }

  const content = sectionBody(lines, headings, headingIndex);
  return content || null;
};

const CHANGELOG_SECTION_LABELS: Readonly<Record<string, string>> = {
  added: 'Added',
  'new features': 'Added',
  improved: 'Improved',
  improvements: 'Improved',
  "what's improved": 'Improved',
  'what’s improved': 'Improved',
  changed: 'Changed',
  fixed: 'Fixed',
  fixes: 'Fixed',
  'bug fixes': 'Fixed',
  security: 'Security',
  deprecated: 'Deprecated',
  removed: 'Removed',
  'alerts and notifications': 'Alerts and notifications',
  'disks and storage': 'Disks and storage',
  'proxmox, pbs and backups': 'Proxmox, PBS and backups',
  'truenas, vsphere and docker': 'TrueNAS, vSphere and Docker',
  'install, updates and agents': 'Install, updates and agents',
  'updates and agents': 'Updates and agents',
  'pulse pro, ai and hosted': 'Pulse Pro, AI and hosted',
  'monitoring and service health': 'Monitoring and service health',
  'other improvements': 'Other improvements',
};

export type ReleaseVisual = {
  title: string;
  description: string;
  before?: string;
  after: string;
};

export type WhatsNewContent = {
  changes: string | null;
  important: string | null;
  upgradeNotes: string | null;
  visuals: ReleaseVisual[];
};

// Only published screenshot sidecars for this exact running release may load.
// PR images are copied into these assets by the release pipeline first.
const releaseImage = (href: string, version: string): RegExpMatchArray | null => {
  const base = `https://github.com/rcourtman/Pulse/releases/download/v${normalizeReleaseVersion(version)}/`;
  if (!isReleaseVersion(version) || !href.startsWith(base)) return null;
  return href
    .slice(base.length)
    .match(/^release-note-([a-z0-9]+(?:-[a-z0-9]+)*)-(before|now)\.png$/);
};

export const extractReleaseVisuals = (markdown: string, version: string): ReleaseVisual[] => {
  const lines = markdown.replace(/\r\n/g, '\n').split('\n');
  const headings = headingsIn(lines);
  const visualIndex = headings.findIndex((heading) => /^see the difference$/i.test(heading.title));
  if (visualIndex === -1) return [];
  const visualLines = sectionBody(lines, headings, visualIndex).split('\n');
  const visualHeadings = headingsIn(visualLines);
  return visualHeadings.flatMap((heading, index) => {
    if (heading.level !== headings[visualIndex].level + 1) return [];
    const body = sectionBody(visualLines, visualHeadings, index);
    const tokens = marked.lexer(body);
    const images: { href: string; id: string; state: string }[] = [];
    marked.walkTokens(tokens, (token) => {
      if (token.type !== 'image') return;
      const match = releaseImage(token.href, version);
      if (match) images.push({ href: token.href, id: match[1], state: match[2] });
    });
    const after = images.find((image) => image.state === 'now');
    if (!after) return [];
    const before = images.find((image) => image.state === 'before' && image.id === after.id);
    const description = tokens.find(
      (token) =>
        token.type === 'paragraph' && !token.tokens?.some((child) => child.type === 'image'),
    );
    return [
      {
        title: heading.title,
        description: description?.type === 'paragraph' ? description.text : '',
        before: before?.href,
        after: after.href,
      },
    ];
  });
};

/**
 * Read the same authored release body used by GitHub and the update preview.
 * The post-update dialog is a changelog: the curated Highlights summary only
 * stands in for the change groups when a release has none.
 */
export const extractWhatsNew = (markdown: string, version: string): WhatsNewContent | null => {
  const lines = markdown.replace(/\r\n/g, '\n').split('\n');
  const headings = headingsIn(lines);
  const sectionMarkdown = (titles: ReadonlySet<string>, includeTitles = true) =>
    headings
      .flatMap((heading, index) => {
        if (!titles.has(heading.title.toLowerCase())) return [];
        const body = sectionBody(lines, headings, index);
        return body ? [includeTitles ? `### ${heading.title}\n\n${body}` : body] : [];
      })
      .join('\n\n') || null;
  // Authored warning sections lead the changelog, without inferring severity
  // from individual words.
  const important = sectionMarkdown(new Set(['known issues', 'breaking changes']));
  const upgradeNotes = sectionMarkdown(new Set(['before you upgrade', 'upgrade notes']), false);
  const changes = extractChangelog(markdown) ?? extractHighlights(markdown);
  const visuals = extractReleaseVisuals(markdown, version);
  return changes || important || upgradeNotes || visuals.length
    ? { changes, important, upgradeNotes, visuals }
    : null;
};

/**
 * Build the post-update changelog from the user-facing change categories in a
 * published release body. Warnings, upgrade notes and visuals render
 * separately around it.
 */
export const extractChangelog = (markdown: string): string | null => {
  const lines = markdown.replace(/\r\n/g, '\n').split('\n');
  const headings = headingsIn(lines);
  const sections: string[] = [];
  let coveredThrough = -1;

  headings.forEach((heading, headingIndex) => {
    if (heading.index <= coveredThrough) return;
    const normalizedTitle = heading.title.replace(/\s+/g, ' ').toLowerCase();
    const label = CHANGELOG_SECTION_LABELS[normalizedTitle];
    const separateSection = [
      'highlights',
      'see the difference',
      'before you upgrade',
      'upgrade notes',
      'known issues',
      'breaking changes',
    ].includes(normalizedTitle);
    if (label || separateSection) {
      const boundary = headings.slice(headingIndex + 1).find((next) => next.level <= heading.level);
      coveredThrough = (boundary?.index ?? lines.length) - 1;
    }
    if (!label) return;

    const content = sectionBody(lines, headings, headingIndex);
    if (content) {
      sections.push(`### ${label}\n\n${content}`);
    }
  });

  return sections.length > 0 ? sections.join('\n\n') : null;
};

// Dev builds carry -dirty or a -g<hash> suffix; they never correspond to a
// published release, so the banner should stay quiet for them.
export const isReleaseVersion = (version: string): boolean => {
  const trimmed = version.trim();
  return /^v?\d+\.\d+\.\d+(?:-(?:rc|alpha|beta)\.\d+)?$/.test(trimmed);
};
