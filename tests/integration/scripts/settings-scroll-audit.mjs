// Settings lives inside the app's independently scrolling shell. Scrolling
// window (including the mobile WebKit wheel fallback) leaves its content at top.
/** @param {import('@playwright/test').Page} page */
export const scrollSettingsToBottom = async (page) => {
  const shell = page.locator('.app-scroll-shell');
  await shell.waitFor({ state: 'visible' });
  const step = await shell.evaluate((element) =>
    Math.max(240, Math.floor(element.clientHeight * 0.75)),
  );
  // Fleet size determines the distance to the bottom. A fixed number of
  // steps can stop halfway through a populated infrastructure page.
  for (;;) {
    const before = await settingsScrollPosition(page);
    await shell.evaluate((element, deltaY) => element.scrollBy(0, deltaY), step);
    await page.waitForTimeout(60);
    const after = await settingsScrollPosition(page);
    if (after.scrollTop >= after.maxScrollTop - 3 &&
        after.maxScrollTop === before.maxScrollTop) return;
    if (after.scrollTop <= before.scrollTop && after.maxScrollTop === before.maxScrollTop) {
      throw new Error(`Settings content did not scroll: ${JSON.stringify(after)}`);
    }
  }
};

/** @param {import('@playwright/test').Page} page */
export const settingsScrollPosition = async (page) =>
  page.locator('.app-scroll-shell').evaluate((element) => ({
    scrollTop: element.scrollTop,
    maxScrollTop: Math.max(0, element.scrollHeight - element.clientHeight),
  }));

/** @param {import('@playwright/test').Page} page */
export const auditHorizontalOverflow = async (page) =>
  page.evaluate(() => {
    const viewportWidth = Math.max(
      document.documentElement.clientWidth,
      window.innerWidth || 0,
    );
    const pageWidth = Math.max(
      document.querySelector(".app-scroll-shell")?.scrollWidth || 0,
      document.body.scrollWidth,
      document.documentElement.scrollWidth,
      document.body.offsetWidth,
      document.documentElement.offsetWidth,
    );

    const offenders = Array.from(document.querySelectorAll("body *"))
      .map((el) => {
        const rect = el.getBoundingClientRect();
        if (rect.width <= 0 || rect.height <= 0) return null;
        const style = window.getComputedStyle(el);
        if (style.position === "fixed" || style.position === "absolute")
          return null;
        const overflow = rect.right - viewportWidth;
        if (overflow <= 1) return null;
        return {
          tag: el.tagName.toLowerCase(),
          className: (el.getAttribute("class") || "").trim().slice(0, 120),
          overflow: Number(overflow.toFixed(1)),
        };
      })
      .filter(Boolean)
      .slice(0, 8);

    return {
      viewportWidth,
      pageWidth,
      overflowPx: Number((pageWidth - viewportWidth).toFixed(1)),
      offenders,
    };
  });
