import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { createSignal } from 'solid-js';
import type { Resource } from '@/types/resource';
import { TableCell, TableRow } from '@/components/shared/Table';
import {
  PlatformResourceDetailToggleButton,
  PlatformResourceDetailTableRow,
  createPlatformResourceDetailState,
  getPlatformResourceDetailRowClass,
  getPlatformResourceDetailRowInteractionProps,
} from '../PlatformResourceDetailTableRow';

vi.mock('@/components/Infrastructure/ResourceDetailDrawer', () => ({
  ResourceDetailDrawer: (props: { resource: Resource }) => (
    <div data-testid="canonical-resource-inspector">{props.resource.id}</div>
  ),
}));

afterEach(cleanup);

describe('platform resource row context', () => {
  it('mounts row-specific context only inside an open detail without changing the canonical resource', () => {
    const [open, setOpen] = createSignal(false);
    render(() => (
      <table>
        <tbody>
          <PlatformResourceDetailTableRow
            resource={{ id: 'canonical-host' } as Resource}
            open={open()}
            detailRowId="selected-row"
            colSpan={4}
            context={
              <section aria-label="Selected row">Selected datastore, not a host identity</section>
            }
          />
        </tbody>
      </table>
    ));
    expect(screen.queryByRole('region')).toBeNull();
    expect(screen.queryByTestId('canonical-resource-inspector')).toBeNull();
    setOpen(true);
    expect(screen.getByRole('region', { name: 'Selected row' })).toHaveTextContent(
      'Selected datastore',
    );
    expect(document.getElementById('selected-row')).toContainElement(screen.getByRole('region'));
    expect(screen.getByTestId('canonical-resource-inspector')).toHaveTextContent('canonical-host');
    setOpen(false);
    expect(screen.queryByRole('region')).toBeNull();
  });

  it('leaves context-free inspectors unchanged', () => {
    render(() => (
      <table>
        <tbody>
          <PlatformResourceDetailTableRow
            resource={{ id: 'ordinary-host' } as Resource}
            open
            detailRowId="ordinary-row"
            colSpan={4}
          />
        </tbody>
      </table>
    ));
    expect(screen.queryByRole('region')).toBeNull();
    expect(screen.getByTestId('canonical-resource-inspector')).toHaveTextContent('ordinary-host');
  });
});

describe('getPlatformResourceDetailRowInteractionProps', () => {
  const renderRow = (onToggle = vi.fn()) => {
    render(() => (
      <table>
        <tbody>
          <TableRow
            {...getPlatformResourceDetailRowInteractionProps({
              expanded: false,
              onToggle,
            })}
          >
            <TableCell>
              <span>Resource</span>
              <button type="button">Open link action</button>
            </TableCell>
          </TableRow>
        </tbody>
      </table>
    ));
    return { onToggle, row: screen.getByRole('row'), childAction: screen.getByRole('button') };
  };

  it('keeps whole-row activation as a pointer convenience only', () => {
    const { onToggle, row } = renderRow();

    expect(row).not.toHaveAttribute('tabindex');
    expect(row).not.toHaveAttribute('aria-expanded');
    expect(row).not.toHaveAttribute('aria-controls');

    fireEvent.click(row);
    fireEvent.keyDown(row, { key: 'Enter' });
    fireEvent.keyDown(row, { key: ' ' });
    expect(onToggle).toHaveBeenCalledTimes(1);
  });

  it('does not hijack embedded interactive controls', () => {
    const { onToggle, childAction } = renderRow();

    fireEvent.click(childAction);
    fireEvent.keyDown(childAction, { key: 'Enter' });
    expect(onToggle).not.toHaveBeenCalled();
  });
});

describe('platform resource drawer row toggle', () => {
  afterEach(() => {
    document.getSelection()?.removeAllRanges();
  });

  it('keeps the drawer as it was when a drag selects the resource name', () => {
    const resource = { id: 'pod-web' };
    render(() => {
      const drawer = createPlatformResourceDetailState({ idPrefix: 'test-pod-drawer' });
      return (
        <table>
          <tbody>
            <TableRow
              class={getPlatformResourceDetailRowClass(drawer.isExpanded(resource))}
              onClick={() => drawer.toggle(resource)}
            >
              <TableCell>
                <PlatformResourceDetailToggleButton
                  expanded={drawer.isExpanded(resource)}
                  resourceLabel="web-7d9f8b6c5-x2kqp"
                  controlsId={drawer.detailRowId(resource)}
                  onToggle={() => drawer.toggle(resource)}
                />
                <span>web-7d9f8b6c5-x2kqp</span>
              </TableCell>
            </TableRow>
          </tbody>
        </table>
      );
    });
    const name = screen.getByText('web-7d9f8b6c5-x2kqp');
    const toggle = screen.getByRole('button');
    const range = document.createRange();
    range.selectNodeContents(name);
    const selection = document.getSelection()!;
    selection.removeAllRanges();
    selection.addRange(range);

    const drag = () => {
      fireEvent.mouseDown(name, { clientX: 4, clientY: 8 });
      fireEvent.click(name, { clientX: 96, clientY: 8 });
    };

    drag();
    expect(toggle).toHaveAttribute('aria-expanded', 'false');

    // A plain click on the still-selected name opens the drawer first time.
    fireEvent.mouseDown(name, { clientX: 40, clientY: 8 });
    fireEvent.click(name, { clientX: 40, clientY: 8 });
    expect(toggle).toHaveAttribute('aria-expanded', 'true');

    drag();
    expect(toggle).toHaveAttribute('aria-expanded', 'true');

    // The chevron and its keyboard activation keep working with text selected.
    fireEvent.click(toggle);
    expect(toggle).toHaveAttribute('aria-expanded', 'false');
    fireEvent.keyDown(toggle, { key: 'Enter' });
    expect(toggle).toHaveAttribute('aria-expanded', 'true');
  });
});
