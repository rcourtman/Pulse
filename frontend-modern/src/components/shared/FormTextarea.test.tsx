import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it } from 'vitest';
import { FormTextarea } from './FormTextarea';

describe('FormTextarea', () => {
  afterEach(() => {
    cleanup();
  });

  it('owns the label relationship for native textareas', () => {
    render(() => <FormTextarea label="Notes" value="Initial notes" data-testid="notes-field" />);

    const textarea = screen.getByLabelText('Notes');
    expect(textarea).toBe(screen.getByTestId('notes-field'));
    expect(textarea).toHaveValue('Initial notes');
  });

  it('preserves explicit ids and compact styling hooks', () => {
    render(() => (
      <FormTextarea
        id="incident-note"
        label="Incident note"
        value=""
        fieldBaseClass="flex"
        fieldClass="flex-row"
        labelClass="text-xs"
        textareaClass="min-h-16"
      />
    ));

    const textarea = screen.getByLabelText('Incident note');
    expect(textarea).toHaveAttribute('id', 'incident-note');
    expect(textarea).toHaveClass('min-h-16');
  });

  it('connects helper text without dropping an existing description', () => {
    render(() => (
      <>
        <p id="external-help">External help</p>
        <FormTextarea
          id="payload"
          label="Payload"
          value=""
          aria-describedby="external-help"
          help="Use JSON template variables."
        />
      </>
    ));

    const textarea = screen.getByLabelText('Payload');
    expect(textarea).toHaveAccessibleDescription('External help Use JSON template variables.');
    expect(textarea).toHaveAttribute('aria-describedby', 'external-help payload-help');
  });

  it('keeps the DOM value synchronized when controlled state changes', () => {
    const [value, setValue] = createSignal('first');

    render(() => (
      <>
        <button type="button" onClick={() => setValue('second')}>
          Update
        </button>
        <FormTextarea
          label="Controlled note"
          value={value()}
          onInput={(event) => setValue(event.currentTarget.value)}
        />
      </>
    ));

    const textarea = screen.getByLabelText('Controlled note');
    expect(textarea).toHaveValue('first');
    fireEvent.input(textarea, { target: { value: 'typed' } });
    expect(textarea).toHaveValue('typed');
    fireEvent.click(screen.getByRole('button', { name: 'Update' }));
    expect(textarea).toHaveValue('second');
  });
});
