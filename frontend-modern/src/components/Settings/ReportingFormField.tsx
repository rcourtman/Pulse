import type { JSX } from 'solid-js';
import { formField, formHelpText, formLabel } from '@/components/shared/Form';

interface FormFieldProps {
  label: string;
  helpText?: string;
  children: JSX.Element;
}

export function ReportingFormField(props: FormFieldProps) {
  return (
    <div class={formField} role="group" aria-label={props.label}>
      <span class={formLabel}>{props.label}</span>
      {props.children}
      {props.helpText && <span class={formHelpText}>{props.helpText}</span>}
    </div>
  );
}
