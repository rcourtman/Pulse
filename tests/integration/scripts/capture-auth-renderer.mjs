import { createHash } from 'node:crypto';

// Read renderer facts, not page contents or account/session state. CDP reads
// the already loaded stylesheets; this collector makes no HTTP requests and
// never changes CSS, fonts, the viewport or the screenshot assertion.
export async function captureAuthRenderer(page) {
  const browser = page.context().browser();
  const result = {
    schema_version: 1,
    browser: { name: browser?.browserType().name(), version: browser?.version() },
    viewport: null,
    heading_style: null,
    fonts: [],
    stylesheets: [],
    gaps: [],
  };
  try {
    const observed = await page.evaluate(() => {
      const heading = document.querySelector('h1');
      const style = heading && getComputedStyle(heading);
      return {
        viewport: { width: innerWidth, height: innerHeight, scale: devicePixelRatio },
        heading_style: style && {
          family: style.fontFamily,
          size: style.fontSize,
          weight: style.fontWeight,
          line_height: style.lineHeight,
          letter_spacing: style.letterSpacing,
        },
      };
    });
    result.viewport = observed.viewport;
    result.heading_style = observed.heading_style;
    if (!observed.heading_style) result.gaps.push('heading-missing');
  } catch {
    result.gaps.push('computed-style-unavailable');
  }
  if (result.browser.name !== 'chromium') {
    result.gaps.push('platform-fonts-and-css-cdp-unavailable');
    return result;
  }

  let session;
  try {
    session = await page.context().newCDPSession(page);
    const sheets = new Map();
    session.on('CSS.styleSheetAdded', ({ header }) => {
      if (sheets.size < 32) sheets.set(header.styleSheetId, header);
      else if (!sheets.has(header.styleSheetId) && !result.gaps.includes('stylesheet-limit')) {
        result.gaps.push('stylesheet-limit');
      }
    });
    await session.send('DOM.enable');
    await session.send('CSS.enable');
    const { root } = await session.send('DOM.getDocument', { depth: 0 });
    const { nodeId } = await session.send('DOM.querySelector', { nodeId: root.nodeId, selector: 'h1' });
    if (nodeId) {
      try {
        const { fonts } = await session.send('CSS.getPlatformFontsForNode', { nodeId });
        result.fonts = fonts.slice(0, 32).map((font) => ({
          family: font.familyName,
          postscript: font.postScriptName,
          custom: font.isCustomFont,
          glyphs: font.glyphCount,
        }));
        if (!fonts.length) result.gaps.push('platform-fonts-empty');
        if (fonts.length > 32) result.gaps.push('font-limit');
      } catch {
        result.gaps.push('platform-fonts-unavailable');
      }
    }
    for (const styleSheetId of sheets.keys()) {
      try {
        const { text } = await session.send('CSS.getStyleSheetText', { styleSheetId });
        const bytes = Buffer.byteLength(text);
        if (bytes > 1024 * 1024) {
          result.gaps.push('stylesheet-byte-limit');
          continue;
        }
        result.stylesheets.push({ bytes, sha256: createHash('sha256').update(text).digest('hex') });
      } catch {
        result.gaps.push('stylesheet-unavailable');
      }
    }
    if (!sheets.size) result.gaps.push('stylesheets-empty');
  } catch {
    // Diagnostic failures stay explicit without logging protocol error text,
    // which can include page URLs. They do not replace the strict pixel result.
    result.gaps.push('cdp-unavailable');
  } finally {
    if (session) {
      try { await session.detach(); }
      catch { result.gaps.push('cdp-detach-unavailable'); }
    }
  }
  return result;
}
