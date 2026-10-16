import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { renderDocMarkdown } from '../docMarkdown';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../../..');
const read = (file: string) => readFileSync(path.join(root, file), 'utf8');
const render = (source: string, slug: string) => {
  const article = document.createElement('article');
  article.innerHTML = renderDocMarkdown(source, slug);
  return article;
};
const install = render(read('docs/INSTALL.md'), 'INSTALL');
const recipe = [...install.querySelectorAll('pre code')].find((code) =>
  code.textContent?.includes('ssh-keygen -Y verify'),
)!;

describe('signed-installer entrypoints', () => {
  for (const locale of ['de', 'es']) {
    const slug = `i18n/${locale}/README`;
    const source = read(`docs/${slug}.md`);
    const article = render(source, slug);

    it(`${locale} renders the complete bounded LXC recipe, including cleanup`, () => {
      const recipes = [...article.querySelectorAll('pre code')].filter((code) =>
        code.textContent?.includes('ssh-keygen -Y verify'),
      );
      expect(recipes).toHaveLength(1);
      expect(recipes[0].textContent).toBe(recipe.textContent);
      expect(recipes[0].textContent?.trim().endsWith(')')).toBe(true);
      expect(read(`frontend-modern/public/docs/${slug}.md`)).toBe(source);
    });

    it(`${locale} routes a standard Linux host to the actual systemd recipe`, () => {
      const link = article.querySelector('a[href="/docs/INSTALL#2-bare-metal--systemd"]');
      expect(link?.hasAttribute('data-doc-link')).toBe(true);
      expect(install.querySelector('[id="2-bare-metal--systemd"]')).not.toBeNull();
    });
  }

  it('the root landing page links both complete platform recipes without a third fork', () => {
    const article = render(read('README.md'), 'README');
    for (const fragment of ['proxmox-ve-lxc-installer', '2-bare-metal--systemd']) {
      expect(article.querySelector(`a[href="/docs/INSTALL#${fragment}"]`)).not.toBeNull();
      expect(install.querySelector(`[id="${fragment}"]`)).not.toBeNull();
    }
    expect(article.textContent).toContain('private temporary files');
    expect(
      [...article.querySelectorAll('pre code')].some((code) =>
        code.textContent?.includes('ssh-keygen -Y verify'),
      ),
    ).toBe(false);
  });
});
