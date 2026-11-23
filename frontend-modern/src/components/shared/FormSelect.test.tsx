import { For, createSignal } from 'solid-js';
import { cleanup, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';
import { FormSelect } from './FormSelect';

describe('FormSelect', () => {
  afterEach(() => {
    cleanup();
  });

  it('owns the label relationship and renders an accessible dropdown chevron', () => {
    const { container } = render(() => (
      <FormSelect label="Delivery mode" value="cli" data-testid="delivery-mode">
        <option value="cli">Local CLI</option>
        <option value="http">Remote API</option>
      </FormSelect>
    ));

    const select = screen.getByLabelText('Delivery mode');
    expect(select).toBe(screen.getByTestId('delivery-mode'));
    expect(select).toHaveValue('cli');
    expect(select).toHaveClass('min-h-11', 'sm:min-h-9');
    // Keeps shared classes for existing callers while drawing the SVG chevron.
    expect(select).toHaveClass('appearance-none', 'form-select-arrow', 'bg-none!', 'pr-9!');

    const chevron = container.querySelector('svg');
    expect(chevron).not.toBeNull();
    expect(chevron).toHaveAttribute('aria-hidden', 'true');
    expect(chevron).toHaveClass('pointer-events-none', 'text-muted');
    expect(chevron).not.toHaveClass('opacity-50');
  });

  it('renders a visible chevron and reserved right padding for custom selectBaseClass callers', () => {
    const { container } = render(() => (
      <FormSelect
        id="system-log-level"
        label="Server Log Level:"
        value="info"
        disabled
        fieldBaseClass="flex"
        fieldClass="items-center space-x-3"
        selectBaseClass="form-select min-h-11 sm:min-h-9 text-sm py-2.5 px-3 rounded-md border-border bg-surface text-muted"
      >
        <option value="info">Info</option>
      </FormSelect>
    ));

    const select = screen.getByLabelText('Server Log Level:');
    expect(select).toHaveClass('form-select', 'appearance-none', 'bg-none!', 'pr-9!');

    const chevron = container.querySelector('svg');
    expect(chevron).not.toBeNull();
    expect(chevron).toHaveAttribute('aria-hidden', 'true');
    expect(chevron).toHaveClass('pointer-events-none', 'opacity-50');
  });

  it('omits the chevron wrapper for hidden overlay or multi-select controls', () => {
    const { container } = render(() => (
      <>
        <FormSelect
          label="Organization"
          value="org-1"
          selectBaseClass="z-10 col-start-1 row-start-1 opacity-0"
        >
          <option value="org-1">Default</option>
        </FormSelect>
        <FormSelect label="Regions" multiple value={['us']}>
          <option value="us">US</option>
          <option value="eu">EU</option>
        </FormSelect>
      </>
    ));

    expect(container.querySelector('svg')).toBeNull();
  });

  it('preserves explicit ids and compact styling hooks', () => {
    render(() => (
      <FormSelect
        id="log-level"
        label="Log level"
        value="debug"
        fieldBaseClass="flex"
        fieldClass="flex-row"
        labelClass="text-xs"
        selectClass="w-auto"
      >
        <option value="debug">Debug</option>
      </FormSelect>
    ));

    const select = screen.getByLabelText('Log level');
    expect(select).toHaveAttribute('id', 'log-level');
    expect(select).toHaveClass('w-auto');
    expect(select.parentElement).toHaveClass('inline-grid');
  });

  it('provides a canonical compact drawer density', () => {
    const { container } = render(() => (
      <FormSelect label="Monitoring" value="normal" density="compact">
        <option value="normal">Normal monitoring</option>
      </FormSelect>
    ));

    expect(screen.getByLabelText('Monitoring')).toHaveClass(
      'min-h-11',
      'sm:min-h-8',
      'px-2',
      'py-1.5',
      'text-xs',
      'pr-8!',
    );
    expect(container.querySelector('svg')).toHaveClass('h-3.5', 'w-3.5');
  });

  it('connects helper text without dropping an existing description', () => {
    render(() => (
      <>
        <p id="external-help">External help</p>
        <FormSelect
          id="provider"
          label="Provider"
          value="local"
          aria-describedby="external-help"
          help="Choose the provider this feature should use."
        >
          <option value="local">Local</option>
        </FormSelect>
      </>
    ));

    const select = screen.getByLabelText('Provider');
    expect(select).toHaveAccessibleDescription(
      'External help Choose the provider this feature should use.',
    );
    expect(select).toHaveAttribute('aria-describedby', 'external-help provider-help');
  });

  it('reapplies controlled values when options materialize asynchronously', async () => {
    const [options, setOptions] = createSignal<{ value: string; label: string }[]>([]);

    render(() => (
      <>
        <button
          type="button"
          onClick={() =>
            setOptions([
              { value: 'all', label: 'All platforms' },
              { value: 'truenas', label: 'TrueNAS' },
            ])
          }
        >
          Load options
        </button>
        <FormSelect label="Platform" value="truenas" data-testid="platform-filter">
          <For each={options()}>
            {(option) => <option value={option.value}>{option.label}</option>}
          </For>
        </FormSelect>
      </>
    ));

    const select = screen.getByTestId('platform-filter') as HTMLSelectElement;
    expect(select.value).toBe('');

    screen.getByRole('button', { name: 'Load options' }).click();

    await waitFor(() => expect(select.value).toBe('truenas'));
    expect((screen.getByRole('option', { name: 'TrueNAS' }) as HTMLOptionElement).selected).toBe(
      true,
    );
  });
});
