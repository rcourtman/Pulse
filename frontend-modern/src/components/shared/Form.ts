const baseField = 'flex flex-col gap-1';
const baseLabel = 'text-sm font-medium text-slate-700 dark:text-slate-300';
const baseHelp = 'text-xs text-muted';
const baseControl = [
  'w-full min-h-10 sm:min-h-9 rounded-md border border-slate-300 bg-white px-3 py-2.5 text-sm text-slate-900',
  'focus:outline-none focus:ring-0 focus:border-blue-500 transition-colors',
  'dark:border-slate-600 dark:bg-slate-800 dark:text-slate-100 dark:focus:border-blue-500',
].join(' ');
const baseCheckbox =
  'h-4 w-4 rounded border-slate-300 text-blue-600 focus:ring-0 focus:ring-offset-0 dark:border-slate-600 dark:bg-slate-800 transition-colors';

const join = (base: string, extra?: string) => (extra ? `${base} ${extra}`.trim() : base);

export const formSection = 'space-y-6';
export const formField = baseField;
export const formFieldInline = 'flex flex-col gap-1 sm:flex-row sm:items-center sm:gap-3';
export const formLabel = baseLabel;
export const formHelpText = baseHelp;
export const formControl = baseControl;
export const formCheckbox = baseCheckbox;

export const formControlDense = join(baseControl, 'py-1.5 px-2');
export const formControlMono = join(baseControl, 'font-mono');

export const formSelect = join(baseControl, 'pr-8 appearance-none');
export const formTextarea = join(baseControl, 'min-h-[120px] resize-vertical');

export const formLabelMuted = join(baseLabel, 'text-muted font-normal');

export function labelClass(extra?: string) {
  return join(baseLabel, extra);
}

export function controlClass(extra?: string) {
  return join(baseControl, extra);
}

export default {
  formSection,
  formField,
  formFieldInline,
  formLabel,
  formLabelMuted,
  formHelpText,
  formControl,
  formControlDense,
  formControlMono,
  formSelect,
  formTextarea,
  formCheckbox,
  labelClass,
  controlClass,
};
