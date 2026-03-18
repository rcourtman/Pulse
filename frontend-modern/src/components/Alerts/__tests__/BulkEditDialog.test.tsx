import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';

import { BulkEditDialog, normalizeMetricKey } from '../BulkEditDialog';

// Mock ThresholdSlider — renders a range input that calls onChange on input events
vi.mock('../../Dashboard/ThresholdSlider', () => ({
  ThresholdSlider: (props: {
    type: string;
    min: number;
    max: number;
    value: number;
    onChange: (v: number) => void;
  }) => (
    <input
      type="range"
      data-testid={`slider-${props.type}`}
      min={props.min}
      max={props.max}
      value={props.value}
      onInput={(e) => props.onChange(parseInt(e.currentTarget.value))}
    />
  ),
}));

afterEach(() => {
  cleanup();
});

// ---------------------------------------------------------------------------
// normalizeMetricKey (pure function)
// ---------------------------------------------------------------------------
describe('normalizeMetricKey', () => {
  it('maps known column headers to metric keys', () => {
    const cases: [string, string][] = [
      ['CPU %', 'cpu'],
      ['Memory %', 'memory'],
      ['Disk %', 'disk'],
      ['Disk R MB/s', 'diskRead'],
      ['Disk W MB/s', 'diskWrite'],
      ['Net In MB/s', 'networkIn'],
      ['Net Out MB/s', 'networkOut'],
      ['Usage %', 'usage'],
      ['Temp °C', 'temperature'],
      ['Temperature °C', 'temperature'],
      ['Temperature', 'temperature'],
      ['Restart Count', 'restartCount'],
      ['Restart Window', 'restartWindow'],
      ['Restart Window (s)', 'restartWindow'],
      ['Memory Warn %', 'memoryWarnPct'],
      ['Memory Critical %', 'memoryCriticalPct'],
      ['Warning Size (GiB)', 'warningSizeGiB'],
      ['Critical Size (GiB)', 'criticalSizeGiB'],
      ['Disk Temp °C', 'diskTemperature'],
      ['Backup', 'backup'],
      ['Snapshot', 'snapshot'],
    ];

    for (const [input, expected] of cases) {
      expect(normalizeMetricKey(input)).toBe(expected);
    }
  });

  it('is case-insensitive and trims whitespace', () => {
    expect(normalizeMetricKey('  cpu %  ')).toBe('cpu');
    expect(normalizeMetricKey('CPU %')).toBe('cpu');
    expect(normalizeMetricKey('cPu %')).toBe('cpu');
  });

  it('strips non-alphanumeric chars for unknown columns', () => {
    expect(normalizeMetricKey('Custom Metric!')).toBe('custommetric');
    expect(normalizeMetricKey('foo-bar_baz')).toBe('foobarbaz');
  });

  it('returns empty string for empty input', () => {
    expect(normalizeMetricKey('')).toBe('');
    expect(normalizeMetricKey('   ')).toBe('');
  });
});

// ---------------------------------------------------------------------------
// BulkEditDialog component
// ---------------------------------------------------------------------------
describe('BulkEditDialog', () => {
  const defaultProps = () => ({
    isOpen: true,
    onClose: vi.fn(),
    onSave: vi.fn(),
    selectedIds: ['vm-1', 'vm-2', 'vm-3'],
    columns: ['CPU %', 'Memory %', 'Net In MB/s'],
  });

  it('renders the dialog with correct item count', () => {
    const props = defaultProps();
    render(() => <BulkEditDialog {...props} />);

    expect(screen.getByText('Bulk Edit Settings')).toBeInTheDocument();
    expect(screen.getByText(/Applying changes to 3 items/)).toBeInTheDocument();
  });

  it('renders column labels for each column', () => {
    const props = defaultProps();
    render(() => <BulkEditDialog {...props} />);

    expect(screen.getByText('CPU %')).toBeInTheDocument();
    expect(screen.getByText('Memory %')).toBeInTheDocument();
    expect(screen.getByText('Net In MB/s')).toBeInTheDocument();
  });

  it('calls onClose when Cancel is clicked', () => {
    const props = defaultProps();
    render(() => <BulkEditDialog {...props} />);

    fireEvent.click(screen.getByText('Cancel'));
    expect(props.onClose).toHaveBeenCalledTimes(1);
  });

  it('calls onSave with empty thresholds when Apply is clicked without changes', () => {
    const props = defaultProps();
    render(() => <BulkEditDialog {...props} />);

    fireEvent.click(screen.getByText(/Apply to 3 items/));
    expect(props.onSave).toHaveBeenCalledWith({});
  });

  it('renders ThresholdSlider for cpu/memory/disk/temperature columns', () => {
    const props = defaultProps();
    props.columns = ['CPU %', 'Memory %', 'Disk %', 'Temp °C'];
    render(() => <BulkEditDialog {...props} />);

    expect(screen.getByTestId('slider-cpu')).toBeInTheDocument();
    expect(screen.getByTestId('slider-memory')).toBeInTheDocument();
    expect(screen.getByTestId('slider-disk')).toBeInTheDocument();
    expect(screen.getByTestId('slider-temperature')).toBeInTheDocument();
  });

  it('renders number inputs for non-slider metrics', () => {
    const props = defaultProps();
    props.columns = ['Restart Count', 'Net In MB/s'];
    render(() => <BulkEditDialog {...props} />);

    const inputs = screen.getAllByRole('spinbutton');
    expect(inputs.length).toBe(2);
  });

  it('excludes backup and snapshot columns', () => {
    const props = defaultProps();
    props.columns = ['CPU %', 'Backup', 'Snapshot'];
    render(() => <BulkEditDialog {...props} />);

    // CPU should render
    expect(screen.getByTestId('slider-cpu')).toBeInTheDocument();
    // Backup and Snapshot should NOT render as labels (they render null)
    expect(screen.queryByText('Backup')).not.toBeInTheDocument();
    expect(screen.queryByText('Snapshot')).not.toBeInTheDocument();
  });

  it('saves user-entered values via number input', () => {
    const props = defaultProps();
    props.columns = ['Restart Count'];
    render(() => <BulkEditDialog {...props} />);

    const input = screen.getByRole('spinbutton');
    fireEvent.input(input, { target: { value: '5' } });

    fireEvent.click(screen.getByText(/Apply to 3 items/));
    expect(props.onSave).toHaveBeenCalledWith({ restartCount: 5 });
  });

  it('saves user-entered values via slider', () => {
    const props = defaultProps();
    props.columns = ['CPU %'];
    render(() => <BulkEditDialog {...props} />);

    const slider = screen.getByTestId('slider-cpu');
    fireEvent.input(slider, { target: { value: '75' } });

    fireEvent.click(screen.getByText(/Apply to 3 items/));
    expect(props.onSave).toHaveBeenCalledWith({ cpu: 75 });
  });

  it('sets threshold to undefined when input is cleared (NaN)', () => {
    const props = defaultProps();
    props.columns = ['Restart Count'];
    render(() => <BulkEditDialog {...props} />);

    const input = screen.getByRole('spinbutton');
    // Set a value first
    fireEvent.input(input, { target: { value: '10' } });
    // Then clear it
    fireEvent.input(input, { target: { value: '' } });

    fireEvent.click(screen.getByText(/Apply to 3 items/));
    const saved = props.onSave.mock.calls[0][0] as Record<string, number | undefined>;
    expect(saved.restartCount).toBeUndefined();
  });

  it('shows Clear button only when a threshold is set', () => {
    const props = defaultProps();
    props.columns = ['Restart Count'];
    render(() => <BulkEditDialog {...props} />);

    // Initially no Clear button
    expect(screen.queryByText('Clear')).not.toBeInTheDocument();

    // Set a value
    const input = screen.getByRole('spinbutton');
    fireEvent.input(input, { target: { value: '42' } });

    // Clear button should appear
    expect(screen.getByText('Clear')).toBeInTheDocument();
  });

  it('Clear button resets the threshold to undefined', () => {
    const props = defaultProps();
    props.columns = ['Restart Count'];
    render(() => <BulkEditDialog {...props} />);

    // Set a value
    const input = screen.getByRole('spinbutton');
    fireEvent.input(input, { target: { value: '42' } });

    // Click Clear
    fireEvent.click(screen.getByText('Clear'));

    // Save should produce no set value for restartCount
    fireEvent.click(screen.getByText(/Apply to 3 items/));
    const saved = props.onSave.mock.calls[0][0] as Record<string, number | undefined>;
    expect(saved.restartCount).toBeUndefined();
  });

  it('displays Unchanged when threshold is not set, value when it is', () => {
    const props = defaultProps();
    props.columns = ['Restart Count'];
    render(() => <BulkEditDialog {...props} />);

    // Initially shows Unchanged
    expect(screen.getByText('Unchanged')).toBeInTheDocument();

    // Set a value
    const input = screen.getByRole('spinbutton');
    fireEvent.input(input, { target: { value: '99' } });

    // Should show the value
    expect(screen.getByText('99')).toBeInTheDocument();
    expect(screen.queryByText('Unchanged')).not.toBeInTheDocument();
  });

  it('resets thresholds when the dialog is reopened', () => {
    const onSave = vi.fn();
    const [isOpen, setIsOpen] = createSignal(true);

    render(() => (
      <BulkEditDialog
        isOpen={isOpen()}
        onClose={() => setIsOpen(false)}
        onSave={onSave}
        selectedIds={['a']}
        columns={['Restart Count']}
      />
    ));

    // Set a value
    const input = screen.getByRole('spinbutton');
    fireEvent.input(input, { target: { value: '50' } });

    // Close and reopen
    setIsOpen(false);
    setIsOpen(true);

    // After reopening, save should be empty (thresholds reset)
    fireEvent.click(screen.getByText(/Apply to 1 items/));
    expect(onSave).toHaveBeenCalledWith({});
  });

  it('handles multiple columns with mixed input types', () => {
    const props = defaultProps();
    props.columns = ['CPU %', 'Restart Count', 'Memory %'];
    render(() => <BulkEditDialog {...props} />);

    // Slider for CPU
    const cpuSlider = screen.getByTestId('slider-cpu');
    fireEvent.input(cpuSlider, { target: { value: '80' } });

    // Number input for Restart Count
    const input = screen.getByRole('spinbutton');
    fireEvent.input(input, { target: { value: '3' } });

    // Slider for Memory
    const memSlider = screen.getByTestId('slider-memory');
    fireEvent.input(memSlider, { target: { value: '60' } });

    fireEvent.click(screen.getByText(/Apply to 3 items/));
    expect(props.onSave).toHaveBeenCalledWith({
      cpu: 80,
      restartCount: 3,
      memory: 60,
    });
  });

  it('does not render when isOpen is false', () => {
    const props = defaultProps();
    props.isOpen = false;
    render(() => <BulkEditDialog {...props} />);

    // Dialog component wraps with <Show when={isOpen}>, so nothing renders
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it('passes correct bounds for percentage metrics (0-100)', () => {
    const props = defaultProps();
    props.columns = ['CPU %'];
    render(() => <BulkEditDialog {...props} />);

    const slider = screen.getByTestId('slider-cpu') as HTMLInputElement;
    expect(slider.min).toBe('0');
    expect(slider.max).toBe('100');
  });

  it('passes correct bounds for temperature metrics (20-120)', () => {
    const props = defaultProps();
    props.columns = ['Temp °C'];
    render(() => <BulkEditDialog {...props} />);

    const slider = screen.getByTestId('slider-temperature') as HTMLInputElement;
    expect(slider.min).toBe('20');
    expect(slider.max).toBe('120');
  });

  it('passes correct bounds for restartWindow (0-3600 step 60)', () => {
    const props = defaultProps();
    props.columns = ['Restart Window (s)'];
    render(() => <BulkEditDialog {...props} />);

    const input = screen.getByRole('spinbutton') as HTMLInputElement;
    expect(input.min).toBe('0');
    expect(input.max).toBe('3600');
    expect(input.step).toBe('60');
  });

  it('passes default bounds for unknown metrics (0-1000 step 1)', () => {
    const props = defaultProps();
    props.columns = ['Custom Widget'];
    render(() => <BulkEditDialog {...props} />);

    const input = screen.getByRole('spinbutton') as HTMLInputElement;
    expect(input.min).toBe('0');
    expect(input.max).toBe('1000');
    expect(input.step).toBe('1');
  });

  it('renders correct Apply button text with selectedIds count', () => {
    const props = defaultProps();
    props.selectedIds = ['a', 'b', 'c', 'd', 'e'];
    render(() => <BulkEditDialog {...props} />);

    expect(screen.getByText('Apply to 5 items')).toBeInTheDocument();
  });
});
