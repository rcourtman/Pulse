import { fireEvent, within } from '@solidjs/testing-library';

// The open Coverage row shows one line per restore point and keeps the rest a
// tap away. Evidence tests read that detail, so take those taps for them.
export function revealCoverageDetail(panel: HTMLElement): HTMLElement {
  const more = within(panel).queryByRole('button', { name: 'Show more details' });
  if (more) fireEvent.click(more);
  for (const point of panel.querySelectorAll<HTMLElement>('[data-coverage-evidence-point]')) {
    const toggle = within(point).queryByRole('button', { name: /^Expand details/ });
    if (toggle) fireEvent.click(toggle);
  }
  return panel;
}
