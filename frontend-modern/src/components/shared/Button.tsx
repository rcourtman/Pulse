import { JSX, mergeProps, splitProps } from 'solid-js';

type ButtonVariant = 'primary' | 'secondary' | 'danger' | 'ghost' | 'outline';
type ButtonSize = 'sm' | 'md' | 'lg' | 'icon';

export interface ButtonProps extends JSX.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  size?: ButtonSize;
  isLoading?: boolean;
  class?: string;
}

const variantClasses: Record<ButtonVariant, string> = {
  primary: 'bg-blue-600 text-white hover:bg-blue-700 border border-transparent shadow-sm',
  secondary: 'bg-surface text-base-content hover:bg-surface-hover border border-border shadow-sm',
  danger: 'bg-red-600 text-white hover:bg-red-700 border border-transparent shadow-sm',
  ghost: 'bg-transparent text-base-content hover:bg-surface-hover border border-transparent',
  outline: 'bg-transparent text-base-content border border-border hover:bg-surface-hover',
};

const sizeClasses: Record<ButtonSize, string> = {
  sm: 'px-2.5 py-1.5 text-xs',
  md: 'px-4 py-2 text-sm',
  lg: 'px-6 py-3 text-base',
  icon: 'p-2',
};

export function Button(props: ButtonProps) {
  const merged = mergeProps(
    { variant: 'secondary' as ButtonVariant, size: 'md' as ButtonSize, type: 'button' as const },
    props,
  );
  const [local, rest] = splitProps(merged, [
    'variant',
    'size',
    'isLoading',
    'class',
    'children',
    'disabled',
  ]);

  const baseClasses =
    'inline-flex items-center justify-center font-medium rounded-md transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed';

  return (
    <button
      class={`${baseClasses} ${variantClasses[local.variant]} ${sizeClasses[local.size]} ${local.class || ''}`.trim()}
      disabled={local.disabled || local.isLoading}
      {...rest}
    >
      {local.isLoading ? (
        <svg
          class="animate-spin -ml-1 mr-2 h-4 w-4 text-current"
          xmlns="http://www.w3.org/2000/svg"
          fill="none"
          viewBox="0 0 24 24"
        >
          <circle
            class="opacity-25"
            cx="12"
            cy="12"
            r="10"
            stroke="currentColor"
            stroke-width="4"
          ></circle>
          <path
            class="opacity-75"
            fill="currentColor"
            d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
          ></path>
        </svg>
      ) : null}
      {local.children}
    </button>
  );
}

export default Button;
