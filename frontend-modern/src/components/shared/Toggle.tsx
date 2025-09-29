import { JSX, mergeProps, splitProps } from 'solid-js';

export type ToggleProps = {
  label?: JSX.Element;
  description?: JSX.Element;
  containerClass?: string;
} & JSX.InputHTMLAttributes<HTMLInputElement>;

export function Toggle(props: ToggleProps) {
  const merged = mergeProps({ containerClass: '' }, props);
  const [local, rest] = splitProps(merged, ['label', 'description', 'containerClass', 'class', 'disabled']);

  const isDisabled = () => Boolean(local.disabled);
  const isChecked = () => {
    const value = rest.checked as unknown;
    if (typeof value === 'function') {
      try {
        return Boolean((value as () => unknown)());
      } catch {
        return false;
      }
    }
    return Boolean(value);
  };

  return (
    <label class={`flex items-center gap-3 ${local.containerClass ?? ''} ${local.class ?? ''}`.trim()}>
      <span class={`relative inline-flex h-6 w-11 flex-shrink-0 items-center ${isDisabled() ? 'opacity-60 cursor-not-allowed' : 'cursor-pointer'}`}>
        <input type="checkbox" class="sr-only" disabled={local.disabled} {...rest} />
        <span
          class={`absolute inset-0 rounded-full transition ${
            isChecked()
              ? 'bg-blue-600 dark:bg-blue-500'
              : isDisabled()
                ? 'bg-gray-300 dark:bg-gray-600'
                : 'bg-gray-200 dark:bg-gray-700'
          }`}
        />
        <span
          class="absolute left-1 top-1 h-4 w-4 rounded-full bg-white shadow transition-transform dark:bg-gray-100"
          style={{ transform: isChecked() ? 'translateX(20px)' : 'translateX(0)' }}
        />
      </span>
      {(local.label || local.description) && (
        <span class="flex flex-col text-sm text-gray-700 dark:text-gray-300">
          {local.label}
          <span class="text-xs text-gray-500 dark:text-gray-400">
            {local.description}
          </span>
        </span>
      )}
    </label>
  );
}

export default Toggle;
