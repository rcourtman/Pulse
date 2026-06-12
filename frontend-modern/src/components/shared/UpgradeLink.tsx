import { A } from '@solidjs/router';
import { mergeProps, Show, splitProps, type Component, type JSX } from 'solid-js';
import { ButtonLink, type ButtonLinkProps } from './Button';
import type { ButtonSize, ButtonVariant } from './buttonModel';
import type { UpgradeDestination } from '@/utils/upgradeNavigation';

type UpgradeLinkProps = Omit<JSX.AnchorHTMLAttributes<HTMLAnchorElement>, 'href'> & {
  destination: UpgradeDestination;
};

export type UpgradeButtonTone = 'primary' | 'warning';

type UpgradeButtonLinkProps = Omit<
  ButtonLinkProps,
  'hardNavigation' | 'href' | 'preserveOpener'
> & {
  destination: UpgradeDestination;
  tone?: UpgradeButtonTone;
  mobileFullWidth?: boolean;
};

const getUpgradeNewTab = (destination: UpgradeDestination) =>
  destination.newTab ?? destination.external;

const getUpgradeHardNavigation = (destination: UpgradeDestination) =>
  destination.hardNavigation ?? destination.external;

const getUpgradeTarget = (destination: UpgradeDestination, target?: string) => {
  if (getUpgradeNewTab(destination)) {
    return target ?? '_blank';
  }
  return target;
};

const getUpgradeRel = (
  destination: UpgradeDestination,
  target?: string,
  rel?: string,
) => {
  if (rel) return rel;
  if ((getUpgradeNewTab(destination) || target === '_blank') && !destination.preserveOpener) {
    return 'noopener noreferrer';
  }
  return undefined;
};

const getUpgradeButtonVariant = (tone: UpgradeButtonTone): ButtonVariant =>
  tone === 'warning' ? 'warning' : 'primaryFlat';

const getUpgradeButtonClass = (mobileFullWidth: boolean, className?: string) =>
  [mobileFullWidth ? 'w-full sm:w-auto' : 'w-auto', 'gap-2', className]
    .filter(Boolean)
    .join(' ');

export const UpgradeLink: Component<UpgradeLinkProps> = (props) => {
  const [local, others] = splitProps(props, ['destination', 'rel', 'target']);
  const newTab = () => getUpgradeNewTab(local.destination);
  const hardNavigation = () => getUpgradeHardNavigation(local.destination);
  const useHardLink = () => Boolean(hardNavigation() || newTab());
  const rel = () => {
    return getUpgradeRel(local.destination, local.target, local.rel);
  };
  const target = () => {
    return getUpgradeTarget(local.destination, local.target);
  };

  return (
    <Show
      when={useHardLink()}
      fallback={<A {...others} href={local.destination.href} />}
    >
      <a
        {...others}
        href={local.destination.href}
        target={target()}
        rel={rel()}
      />
    </Show>
  );
};

export const UpgradeButtonLink: Component<UpgradeButtonLinkProps> = (props) => {
  const merged = mergeProps(
    {
      mobileFullWidth: true,
      size: 'settingsAction' as ButtonSize,
      tone: 'primary' as UpgradeButtonTone,
    },
    props,
  );
  const [local, others] = splitProps(merged, [
    'destination',
    'tone',
    'mobileFullWidth',
    'variant',
    'size',
    'class',
    'rel',
    'target',
  ]);

  return (
    <ButtonLink
      {...others}
      href={local.destination.href}
      hardNavigation={getUpgradeHardNavigation(local.destination)}
      preserveOpener={local.destination.preserveOpener}
      target={getUpgradeTarget(local.destination, local.target)}
      rel={getUpgradeRel(local.destination, local.target, local.rel)}
      variant={local.variant ?? getUpgradeButtonVariant(local.tone)}
      size={local.size}
      class={getUpgradeButtonClass(local.mobileFullWidth, local.class)}
    />
  );
};

export default UpgradeLink;
