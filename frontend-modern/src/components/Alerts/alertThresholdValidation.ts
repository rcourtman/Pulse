/** Keep invalid visible drafts out of both row commits and policy writes. */
export function getAlertThresholdInputError(input: HTMLInputElement): string {
  if (input.disabled) return '';
  const missing = input.value === '';
  if (!input.validity.badInput && missing && !input.required) return '';
  const value = input.valueAsNumber;
  if (
    input.validity.badInput ||
    missing ||
    !Number.isFinite(value) ||
    value < Number(input.min) ||
    value > Number(input.max)
  ) {
    return `Enter a number from ${input.min} to ${input.max}.`;
  }
  // Existing metric limits accept fractions, even where the spinner increments
  // by one. Do not turn the native step hint into a new persistence restriction.
  return '';
}

export function validateAlertThresholdEditors(root: ParentNode): HTMLInputElement | undefined {
  let firstInvalid: HTMLInputElement | undefined;
  for (const input of root.querySelectorAll<HTMLInputElement>('[data-alert-threshold-editor]')) {
    const message = getAlertThresholdInputError(input);
    input.setCustomValidity(message);
    if (!message) continue;
    // The field's invalid handler owns its inline, labelled error. Checking all
    // fields exposes every problem, not just the first field reached by Save.
    input.dispatchEvent(new Event('invalid', { cancelable: true }));
    firstInvalid ??= input;
  }
  if (firstInvalid) {
    // A group can be folded after an invalid edit. Use its existing disclosure
    // control to reveal the error before moving keyboard focus to the field.
    const folded = firstInvalid.closest<HTMLElement>('[inert][id]');
    if (folded) {
      const disclosure = Array.from(
        root.querySelectorAll<HTMLButtonElement>('button[aria-controls]'),
      ).find((button) => button.getAttribute('aria-controls') === folded.id);
      disclosure?.click();
    }
    firstInvalid.focus({ preventScroll: true });
    firstInvalid.scrollIntoView?.({ block: 'center', behavior: 'instant' });
  }
  return firstInvalid;
}
