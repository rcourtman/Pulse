import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { AlertResourceMetricEditor } from '../AlertResourceMetricEditor';
import {
  getAlertThresholdInputError,
  validateAlertThresholdEditors,
} from '../alertThresholdValidation';

afterEach(cleanup);

function editor(scope: 'default' | 'override' = 'default', initial: number | undefined = 80) {
  const change = vi.fn();
  const view = render(() => {
    const [value, setValue] = createSignal<number | undefined>(initial);
    return (
      <AlertResourceMetricEditor
        metric="cpu"
        column="CPU %"
        scope={scope}
        layout="card"
        value={value()}
        inheritedDefault={80}
        resourceName="test-guest"
        onChange={(next) => {
          change(next);
          setValue(next);
        }}
      />
    );
  });
  const input = screen.getByRole('spinbutton') as HTMLInputElement;
  return { ...view, input, change };
}

describe('threshold draft validation', () => {
  it('keeps an invalid draft and error through an unrelated parent update', () => {
    let update!: () => void;
    const view = render(() => {
      const [limits, setLimits] = createSignal({ cpu: 80, memory: 85 });
      update = () => setLimits((current) => ({ ...current, memory: 86 }));
      return (
        <AlertResourceMetricEditor
          metric="cpu"
          column="CPU %"
          scope="default"
          layout="table"
          value={limits().cpu}
          onChange={() => {}}
        />
      );
    });
    const input = screen.getByRole('spinbutton');
    fireEvent.input(input, { target: { value: '150' } });
    update();
    expect(input).toHaveValue(150);
    expect(input).toHaveAttribute('aria-invalid', 'true');
    expect(input).toHaveAccessibleDescription('Enter a number from 1 to 100.');
    expect(validateAlertThresholdEditors(view.container, false)).toBe(input);
  });

  it.each(['0', '-1', '101', '150'])('keeps %s out of the draft and names the error', (value) => {
    const { input, change, container } = editor();
    fireEvent.input(input, { target: { value } });
    expect(change).not.toHaveBeenCalled();
    expect(input).toHaveValue(Number(value));
    expect(input).toHaveAttribute('aria-invalid', 'true');
    expect(input).toHaveAccessibleDescription('Enter a number from 1 to 100.');
    expect(validateAlertThresholdEditors(container)).toBe(input);
    expect(input).toHaveFocus();
  });

  it('does not let a blank enabled global limit silently save its previous value', () => {
    const { input, change, container } = editor();
    fireEvent.input(input, { target: { value: '' } });
    expect(change).not.toHaveBeenCalled();
    expect(validateAlertThresholdEditors(container)).toBe(input);
    expect(screen.getByRole('alert')).toHaveTextContent('Enter a number from 1 to 100.');
  });

  it('accepts supported decimal limits and withdraws the prior error', () => {
    const { input, change, container } = editor();
    fireEvent.input(input, { target: { value: '150' } });
    fireEvent.input(input, { target: { value: '90.5' } });
    expect(change).toHaveBeenLastCalledWith(90.5);
    expect(input).not.toHaveAttribute('aria-invalid');
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(validateAlertThresholdEditors(container)).toBeUndefined();
  });

  it('keeps explicit Off available after invalid input', () => {
    const { input, change, container } = editor();
    fireEvent.input(input, { target: { value: '150' } });
    fireEvent.click(screen.getByRole('button', { name: 'CPU % default alerts: On' }));
    expect(change).toHaveBeenLastCalledWith(-1);
    expect(input).toBeDisabled();
    expect(input).not.toHaveAttribute('aria-invalid');
    expect(validateAlertThresholdEditors(container)).toBeUndefined();
  });

  it('keeps a genuinely empty override as inherit, rather than Off', () => {
    const { input, change, container } = editor('override');
    fireEvent.input(input, { target: { value: '' } });
    expect(change).toHaveBeenLastCalledWith(undefined);
    expect(validateAlertThresholdEditors(container)).toBeUndefined();
  });

  it('does not interpret bad numeric input with an empty value as inherit', () => {
    const input = document.createElement('input');
    input.type = 'number';
    input.min = '1';
    input.max = '100';
    Object.defineProperty(input, 'validity', { value: { badInput: true } });
    expect(getAlertThresholdInputError(input)).toBe('Enter a number from 1 to 100.');
  });

  it('allows blank inherited defaults', () => {
    const view = render(() => (
      <AlertResourceMetricEditor
        metric="cpu"
        column="CPU %"
        scope="default"
        layout="card"
        value={undefined}
        fallback={{ enableValue: 80, label: 'Inherited', title: 'Inherits parent limit' }}
        onChange={() => {}}
      />
    ));
    const input = screen.getByRole('spinbutton');
    expect(input).not.toBeRequired();
    expect(validateAlertThresholdEditors(view.container)).toBeUndefined();
  });

  it('reveals a folded group before focusing its invalid field', () => {
    const { input, container } = editor();
    const group = document.createElement('div');
    group.id = 'folded';
    group.setAttribute('inert', '');
    input.parentElement!.appendChild(group);
    group.appendChild(input);
    const disclosure = document.createElement('button');
    disclosure.setAttribute('aria-controls', 'folded');
    const click = vi.fn(() => group.removeAttribute('inert'));
    disclosure.addEventListener('click', click);
    container.appendChild(disclosure);
    fireEvent.input(input, { target: { value: '150' } });
    expect(validateAlertThresholdEditors(container)).toBe(input);
    expect(click).toHaveBeenCalledOnce();
    expect(input).toHaveFocus();
  });

  it('does not reject an unrelated number control or a different editor scope', () => {
    const { input, container } = editor();
    const other = document.createElement('input');
    other.type = 'number';
    other.min = '1';
    other.value = '0';
    container.appendChild(other);
    expect(validateAlertThresholdEditors(container)).toBeUndefined();
    fireEvent.input(input, { target: { value: '150' } });
    expect(validateAlertThresholdEditors(document.createElement('div'))).toBeUndefined();
  });
});
