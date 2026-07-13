// Model logic for the post-update "What's New" banner.
//
// The banner only ever shows the release's "Highlights" section — a curated,
// user-facing summary — never the full changelog. Releases without a
// Highlights section stay silent, so patch releases full of internal fixes
// don't nag anyone.

/**
 * Extract the contents of the `## Highlights` section from a GitHub release
 * body. Returns null when the section is missing or empty, which callers
 * treat as "nothing worth announcing".
 */
export const extractHighlights = (markdown: string): string | null => {
  const lines = markdown.replace(/\r\n/g, '\n').split('\n');
  const headingMatch = (line: string) => line.trim().match(/^(#{1,6})\s+(.*)$/);

  const startIdx = lines.findIndex((line) => {
    const match = headingMatch(line);
    return !!match && /^highlights\b/i.test(match[2].trim());
  });
  if (startIdx === -1) {
    return null;
  }

  const startLevel = headingMatch(lines[startIdx])![1].length;
  const section: string[] = [];
  for (let i = startIdx + 1; i < lines.length; i++) {
    const match = headingMatch(lines[i]);
    if (match && match[1].length <= startLevel) {
      break;
    }
    section.push(lines[i]);
  }

  const content = section.join('\n').trim();
  return content || null;
};

// Dev builds carry -dirty or a -g<hash> suffix; they never correspond to a
// published release, so the banner should stay quiet for them.
export const isReleaseVersion = (version: string): boolean => {
  const trimmed = version.trim();
  return /^v?\d+\.\d+\.\d+(?:-(?:rc|alpha|beta)\.\d+)?$/.test(trimmed);
};
