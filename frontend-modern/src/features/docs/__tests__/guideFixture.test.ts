import { afterEach, describe, expect, it, vi } from 'vitest';
import * as renderer from '../docMarkdown';
import { createGuideFixture } from './guideFixture';

afterEach(() => vi.restoreAllMocks());

describe('complete guide fixtures', () => {
  it('renders once for identical guide bytes and returns isolated DOM clones', () => {
    const render = vi.spyOn(renderer, 'renderDocMarkdown');
    const source =
      '# Full guide\n\n## Destination\n\n[Link](OTHER.md#heading)\n\n<script>bad()</script>';
    const read = vi.fn(() => source);
    const guide = createGuideFixture(read);
    const first = guide('nested/GUIDE');
    expect(first.querySelector('#destination')).not.toBeNull();
    expect(first.querySelector('a')?.getAttribute('href')).toBe('/docs/nested/OTHER#heading');
    expect(first.querySelector('script')).toBeNull();
    first.querySelector('a')!.remove();
    first.querySelector('#destination')!.id = 'mutated';
    const second = guide('nested/GUIDE');
    expect(second).not.toBe(first);
    expect(second.querySelector('#destination')).not.toBeNull();
    expect(second.querySelector('a')).not.toBeNull();
    expect(render).toHaveBeenCalledExactlyOnceWith(source, 'nested/GUIDE');
    expect(read).toHaveBeenCalledTimes(2);
  });

  it('does not reuse a different source or document-path link context', () => {
    const render = vi.spyOn(renderer, 'renderDocMarkdown');
    let source = '# Original\n\n[Link](OTHER.md)';
    const guide = createGuideFixture(() => source);
    expect(guide('nested/GUIDE').querySelector('a')?.getAttribute('href')).toBe(
      '/docs/nested/OTHER',
    );
    expect(guide('GUIDE').querySelector('a')?.getAttribute('href')).toBe('/docs/OTHER');
    expect(render).toHaveBeenCalledTimes(2);
    source = '# Changed\n\nNo link.';
    const changed = guide('GUIDE');
    expect(changed.querySelector('#changed')).not.toBeNull();
    expect(changed.querySelector('#original, a')).toBeNull();
    expect(render).toHaveBeenCalledTimes(3);
  });
});
