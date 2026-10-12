import { renderDocMarkdown } from '../docMarkdown';

/**
 * Render a complete shipped guide once per exact source in this test suite.
 * Clones keep assertions isolated; source/path changes invalidate the fixture.
 * This is not a production renderer cache or an excerpt of the tested guide.
 */
export function createGuideFixture(readSource: (name: string) => string) {
  const guides = new Map<string, { source: string; article: HTMLElement }>();
  return (name: string): HTMLElement => {
    const source = readSource(name);
    let guide = guides.get(name);
    if (!guide || guide.source !== source) {
      const article = document.createElement('article');
      article.innerHTML = renderDocMarkdown(source, name);
      guide = { source, article };
      guides.set(name, guide);
    }
    return guide.article.cloneNode(true) as HTMLElement;
  };
}
