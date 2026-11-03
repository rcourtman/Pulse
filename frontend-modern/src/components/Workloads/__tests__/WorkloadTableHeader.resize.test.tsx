import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render } from '@solidjs/testing-library';
import type { ComponentProps } from 'solid-js';

import { WORKLOAD_COLUMN_MIN_WIDTH } from '../workloadColumnWidths';
import { WorkloadTableHeader } from '../WorkloadTableHeader';

type HeaderProps = ComponentProps<typeof WorkloadTableHeader>;

const COLUMN_WIDTH = 120;
const POINTER_ID = 7;

const columns = [
  { id: 'name', label: 'Name', sortKey: 'name' },
  { id: 'cpu', label: 'CPU', sortKey: 'cpu' },
] as unknown as ReturnType<HeaderProps['workloadTableVisibleColumns']>;

// jsdom has no layout and no pointer capture; give each header cell a width and
// remember which element holds capture so release can be observed.
let captured: Set<number>;

beforeEach(() => {
  captured = new Set();
  vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(
    () =>
      ({
        width: COLUMN_WIDTH,
        height: 20,
        top: 0,
        left: 0,
        right: COLUMN_WIDTH,
        bottom: 20,
      }) as DOMRect,
  );
  Object.defineProperty(HTMLElement.prototype, 'setPointerCapture', {
    configurable: true,
    value: (id: number) => captured.add(id),
  });
  Object.defineProperty(HTMLElement.prototype, 'hasPointerCapture', {
    configurable: true,
    value: (id: number) => captured.has(id),
  });
  Object.defineProperty(HTMLElement.prototype, 'releasePointerCapture', {
    configurable: true,
    value: (id: number) => captured.delete(id),
  });
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

const mountHeader = (overrides: Partial<HeaderProps> = {}) => {
  const callbacks = {
    beginWorkloadColumnResize: vi.fn(),
    previewWorkloadColumnWidth: vi.fn(),
    commitWorkloadColumnResize: vi.fn(),
    cancelWorkloadColumnResize: vi.fn(),
    clearWorkloadColumnWidth: vi.fn(),
    handleSort: vi.fn(),
  };
  const props = {
    isMobile: () => false,
    sortDirection: () => 'asc',
    sortKey: () => 'name',
    visibleColumns: () => columns,
    workloadMemoryDisplayBasis: () => 'guest',
    workloadTableLayoutMode: () => 'wide',
    workloadTableVisibleColumnIds: () => columns.map((column) => column.id),
    workloadTableVisibleColumns: () => columns,
    workloadColumnWidths: () => ({}),
    workloadManualColumnSizingSupported: () => true,
    ...callbacks,
    ...overrides,
  } as unknown as HeaderProps;
  const view = render(() => (
    <table>
      <WorkloadTableHeader {...props} />
    </table>
  ));
  const handle = (columnId: string) =>
    view.container.querySelector<HTMLElement>(`[data-workload-col-resizer="${columnId}"]`)!;
  return { ...view, ...callbacks, handle };
};

const pointer = (target: Element, type: string, fields: Record<string, unknown> = {}) => {
  const event = Object.assign(new Event(type, { bubbles: true, cancelable: true }), {
    pointerId: POINTER_ID,
    button: 0,
    clientX: 100,
    ...fields,
  });
  fireEvent(target, event);
  return event;
};

describe('WorkloadTableHeader column resize handle', () => {
  it('offers a drag handle on every column only where manual sizing is supported', () => {
    const supported = mountHeader();
    expect(supported.handle('name')).not.toBeNull();
    expect(supported.handle('cpu')).not.toBeNull();
    cleanup();

    const unsupported = mountHeader({ workloadManualColumnSizingSupported: () => false });
    expect(unsupported.container.querySelector('[data-workload-col-resizer]')).toBeNull();
  });

  it('resizes the dragged column from its measured width and commits on release', () => {
    const { handle, ...view } = mountHeader();
    const cpu = handle('cpu');

    const down = pointer(cpu, 'pointerdown', { clientX: 100 });
    expect(down.defaultPrevented).toBe(true);
    expect(captured.has(POINTER_ID)).toBe(true);
    expect(view.beginWorkloadColumnResize).not.toHaveBeenCalled();

    pointer(cpu, 'pointermove', { clientX: 160 });
    expect(view.beginWorkloadColumnResize).toHaveBeenCalledTimes(1);
    expect(view.beginWorkloadColumnResize).toHaveBeenCalledWith({
      name: COLUMN_WIDTH,
      cpu: COLUMN_WIDTH,
    });
    expect(view.previewWorkloadColumnWidth).toHaveBeenLastCalledWith('cpu', COLUMN_WIDTH + 60);

    pointer(cpu, 'pointermove', { clientX: 90 });
    expect(view.beginWorkloadColumnResize).toHaveBeenCalledTimes(1);
    expect(view.previewWorkloadColumnWidth).toHaveBeenLastCalledWith('cpu', COLUMN_WIDTH - 10);

    pointer(cpu, 'pointerup', { clientX: 90 });
    expect(view.commitWorkloadColumnResize).toHaveBeenCalledTimes(1);
    expect(view.cancelWorkloadColumnResize).not.toHaveBeenCalled();
    expect(captured.has(POINTER_ID)).toBe(false);
  });

  it('never shrinks a column below the minimum width', () => {
    const { handle, ...view } = mountHeader();
    const name = handle('name');

    pointer(name, 'pointerdown', { clientX: 500 });
    pointer(name, 'pointermove', { clientX: 0 });

    expect(view.previewWorkloadColumnWidth).toHaveBeenLastCalledWith(
      'name',
      WORKLOAD_COLUMN_MIN_WIDTH,
    );
  });

  it('treats a plain click on the handle as a click, not a resize', () => {
    const { handle, ...view } = mountHeader();
    const cpu = handle('cpu');

    pointer(cpu, 'pointerdown', { clientX: 100 });
    pointer(cpu, 'pointermove', { clientX: 101 });
    pointer(cpu, 'pointerup', { clientX: 101 });

    expect(view.beginWorkloadColumnResize).not.toHaveBeenCalled();
    expect(view.previewWorkloadColumnWidth).not.toHaveBeenCalled();
    expect(view.commitWorkloadColumnResize).not.toHaveBeenCalled();
    expect(captured.has(POINTER_ID)).toBe(false);
  });

  it('abandons the drag when the browser cancels the pointer', () => {
    const { handle, ...view } = mountHeader();
    const cpu = handle('cpu');

    pointer(cpu, 'pointerdown', { clientX: 100 });
    pointer(cpu, 'pointermove', { clientX: 180 });
    expect(view.previewWorkloadColumnWidth).toHaveBeenCalledTimes(1);

    pointer(cpu, 'pointercancel');
    expect(view.cancelWorkloadColumnResize).toHaveBeenCalledTimes(1);
    expect(view.commitWorkloadColumnResize).not.toHaveBeenCalled();
    expect(captured.has(POINTER_ID)).toBe(false);

    // The drag is over: later moves and a stray release change nothing.
    pointer(cpu, 'pointermove', { clientX: 240 });
    pointer(cpu, 'pointerup', { clientX: 240 });
    expect(view.previewWorkloadColumnWidth).toHaveBeenCalledTimes(1);
    expect(view.commitWorkloadColumnResize).not.toHaveBeenCalled();
  });

  it('ignores a second pointer while one is dragging and non-primary buttons', () => {
    const { handle, ...view } = mountHeader();
    const cpu = handle('cpu');

    pointer(cpu, 'pointerdown', { button: 2 });
    pointer(cpu, 'pointermove', { clientX: 200 });
    expect(view.beginWorkloadColumnResize).not.toHaveBeenCalled();

    pointer(cpu, 'pointerdown', { clientX: 100 });
    pointer(cpu, 'pointermove', { clientX: 200, pointerId: 99 });
    expect(view.beginWorkloadColumnResize).not.toHaveBeenCalled();
  });

  it('resets one column on double-click and never sorts from the handle', () => {
    const { handle, ...view } = mountHeader();

    fireEvent.dblClick(handle('cpu'));
    expect(view.clearWorkloadColumnWidth).toHaveBeenCalledWith('cpu');

    fireEvent.click(handle('cpu'));
    expect(view.handleSort).not.toHaveBeenCalled();

    fireEvent.click(view.container.querySelector('[data-workload-col="cpu"]')!);
    expect(view.handleSort).toHaveBeenCalledWith('cpu');
  });
});
