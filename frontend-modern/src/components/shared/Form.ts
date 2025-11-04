const baseField = 'flex flex-col gap-1';
const baseLabel = 'text-sm font-medium text-gray-700 dark:text-gray-300';
const baseHelp = 'text-xs text-gray-500 dark:text-gray-400';
const baseControl = [
  'w-full rounded-md border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm',
  'focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500',
  'dark:border-gray-600 dark:bg-gray-800 dark:text-gray-100',
].join(' ');
const baseCheckbox =
  'rounded border-gray-300 text-blue-600 focus:ring-blue-500 dark:border-gray-600 dark:bg-gray-800 dark:focus:ring-blue-400';

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

export const formLabelMuted = join(baseLabel, 'text-gray-500 dark:text-gray-400 font-normal');

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
