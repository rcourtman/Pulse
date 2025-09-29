import { JSX, Show, splitProps, mergeProps } from 'solid-js';

type SectionHeaderProps = {
  label?: JSX.Element;
  title: JSX.Element;
  description?: JSX.Element;
  align?: 'left' | 'center';
  size?: 'sm' | 'md' | 'lg';
  titleClass?: string;
  descriptionClass?: string;
} & Omit<JSX.HTMLAttributes<HTMLDivElement>, 'title'>;

export function SectionHeader(props: SectionHeaderProps) {
  const merged = mergeProps({ align: 'left' as const, size: 'md' as const, titleClass: '', descriptionClass: '' }, props);
  const [local, rest] = splitProps(merged, ['label', 'title', 'description', 'align', 'size', 'titleClass', 'descriptionClass', 'class']);

  const alignmentClass = local.align === 'center' ? 'text-center items-center' : 'text-left items-start';
  const sizeClass = () => {
    switch (local.size) {
      case 'sm':
        return 'text-base';
      case 'lg':
        return 'text-2xl';
      default:
        return 'text-lg';
    }
  };

  return (
    <div class={`flex flex-col gap-1 ${alignmentClass} ${local.class ?? ''}`.trim()} {...rest}>
      <Show when={local.label}>
        <span class="text-[0.7rem] font-semibold uppercase tracking-[0.18em] text-gray-500 dark:text-gray-400">
          {local.label}
        </span>
      </Show>
      <h2 class={`${sizeClass()} font-semibold text-gray-900 dark:text-gray-100 ${local.titleClass ?? ''}`.trim()}>
        {local.title}
      </h2>
      <Show when={local.description}>
        <p class={`text-sm text-gray-600 dark:text-gray-400 ${local.descriptionClass ?? ''}`.trim()}>
          {local.description}
        </p>
      </Show>
    </div>
  );
}

export default SectionHeader;
