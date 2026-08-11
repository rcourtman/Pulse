// Model logic for the release preview and post-update changelog.

type Heading = {
  index: number;
  level: number;
  title: string;
};

const headingMatch = (line: string) => line.trim().match(/^(#{1,6})\s+(.*)$/);

const headingsIn = (lines: string[]): Heading[] =>
  lines.flatMap((line, index) => {
    const match = headingMatch(line);
    return match ? [{ index, level: match[1].length, title: match[2].trim() }] : [];
  });

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
 * Extract the contents of the `## Highlights` section from a GitHub release
 * body. Returns null when the section is missing or empty, which callers
 * treat as "nothing worth announcing".
 */
export const extractHighlights = (markdown: string): string | null => {
  const lines = markdown.replace(/\r\n/g, '\n').split('\n');
  const headings = headingsIn(lines);
  const headingIndex = headings.findIndex((heading) => /^highlights\b/i.test(heading.title));
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
  changed: 'Changed',
  fixed: 'Fixed',
  'bug fixes': 'Fixed',
  security: 'Security',
  'breaking changes': 'Breaking changes',
  deprecated: 'Deprecated',
  removed: 'Removed',
};

/**
 * Build the post-update changelog from the user-facing change categories in a
 * published release body. Highlights are intentionally excluded: they remain
 * a compact pre-update preview, while this view tells users what actually
 * changed under recognizable changelog headings.
 */
export const extractChangelog = (markdown: string): string | null => {
  const lines = markdown.replace(/\r\n/g, '\n').split('\n');
  const headings = headingsIn(lines);
  const sections: string[] = [];

  headings.forEach((heading, headingIndex) => {
    const normalizedTitle = heading.title.replace(/\s+/g, ' ').toLowerCase();
    const label = CHANGELOG_SECTION_LABELS[normalizedTitle];
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
