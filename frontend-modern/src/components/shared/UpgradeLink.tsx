import { A } from '@solidjs/router';
import { Show, splitProps, type Component, type JSX } from 'solid-js';
import type { UpgradeDestination } from '@/utils/upgradeNavigation';

type UpgradeLinkProps = Omit<JSX.AnchorHTMLAttributes<HTMLAnchorElement>, 'href'> & {
  destination: UpgradeDestination;
};

export const UpgradeLink: Component<UpgradeLinkProps> = (props) => {
  const [local, others] = splitProps(props, ['destination', 'rel', 'target']);

  return (
    <Show
      when={local.destination.external}
      fallback={<A {...others} href={local.destination.href} />}
    >
      <a
        {...others}
        href={local.destination.href}
        target={local.target ?? '_blank'}
        rel={local.rel ?? 'noopener noreferrer'}
      />
    </Show>
  );
};

export default UpgradeLink;
