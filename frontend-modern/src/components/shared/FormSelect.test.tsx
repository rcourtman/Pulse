import { For, createSignal } from 'solid-js';
import { cleanup, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';
import { FormSelect } from './FormSelect';

describe('FormSelect', () => {
  afterEach(() => {
    cleanup();
  });

  it('owns the label relationship for native selects', () => {
    render(() => (
      <FormSelect label="Delivery mode" value="cli" data-testid="delivery-mode">
        <option value="cli">Local CLI</option>
        <option value="http">Remote API</option>
      </FormSelect>
    ));

    const select = screen.getByLabelText('Delivery mode');
    expect(select).toBe(screen.getByTestId('delivery-mode'));
    expect(select).toHaveValue('cli');
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
