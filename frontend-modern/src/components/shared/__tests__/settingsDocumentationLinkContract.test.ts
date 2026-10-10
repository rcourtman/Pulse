import { describe, expect, it } from 'vitest';
import { settingsDocumentationLinkViolations } from './settingsDocumentationLinkContract';

const valid = `
  import { ExternalTextLink } from '@/components/shared/ExternalTextLink';
  import { TERMS_DOC_URL } from '@/utils/docsLinks';
  export const Terms = () => <ExternalTextLink href={TERMS_DOC_URL}>Terms of Service</ExternalTextLink>;
`;

describe('surviving settings documentation link contract', () => {
  it('accepts an actual shared link to shipped Terms', () => {
    expect(settingsDocumentationLinkViolations(valid)).toEqual([]);
  });

  it('accepts legitimate removal of the whole link and its imports', () => {
    expect(
      settingsDocumentationLinkViolations('export const Settings = () => <p>Ask first</p>;'),
    ).toEqual([]);
  });

  it('resolves aliased shared imports instead of requiring literal symbol mentions', () => {
    expect(
      settingsDocumentationLinkViolations(
        valid
          .replace('import { ExternalTextLink }', 'import { ExternalTextLink as TextLink }')
          .replaceAll('<ExternalTextLink', '<TextLink')
          .replaceAll('</ExternalTextLink', '</TextLink'),
      ),
    ).toEqual([]);
  });

  it.each([
    [
      'raw Terms anchor',
      valid.replaceAll('<ExternalTextLink', '<a').replaceAll('</ExternalTextLink', '</a'),
    ],
    ['wrong local route', valid.replace('href={TERMS_DOC_URL}', 'href="/docs/PRIVACY"')],
    [
      'unpublished GitHub route',
      valid.replace(
        'href={TERMS_DOC_URL}',
        'href="https://github.com/rcourtman/Pulse/blob/main/TERMS.md"',
      ),
    ],
    [
      'opener opt-out',
      valid.replace('href={TERMS_DOC_URL}', 'href={TERMS_DOC_URL} preserveOpener={true}'),
    ],
    [
      'raw target override',
      valid.replace('href={TERMS_DOC_URL}', 'href={TERMS_DOC_URL} target="_blank"'),
    ],
    [
      'untrusted primitive',
      valid.replace('@/components/shared/ExternalTextLink', './ExternalTextLink'),
    ],
    [
      'comment instead of import',
      valid.replace(
        "import { ExternalTextLink } from '@/components/shared/ExternalTextLink';",
        '// ExternalTextLink',
      ),
    ],
    [
      'dangling Terms import',
      "import { TERMS_DOC_URL } from '@/utils/docsLinks'; export const Settings = () => <p>No link</p>;",
    ],
    [
      'raw new-tab link with expression target',
      'export const Settings = () => <a href="https://example.invalid" target={"_blank"}>Docs</a>;',
    ],
  ])('rejects %s rather than accepting symbol-only coverage', (_name, source) => {
    expect(settingsDocumentationLinkViolations(source)).not.toEqual([]);
  });
});
