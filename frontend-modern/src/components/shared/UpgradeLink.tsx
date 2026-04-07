import { A } from '@solidjs/router';
import { Show, splitProps, type Component, type JSX } from 'solid-js';
import type { UpgradeDestination } from '@/utils/upgradeNavigation';

type UpgradeLinkProps = Omit<JSX.AnchorHTMLAttributes<HTMLAnchorElement>, 'href'> & {
  destination: UpgradeDestination;
};

export const UpgradeLink: Component<UpgradeLinkProps> = (props) => {
  const [local, others] = splitProps(props, ['destination', 'rel', 'target']);
  const newTab = () => local.destination.newTab ?? local.destination.external;
  const hardNavigation = () => local.destination.hardNavigation ?? local.destination.external;
  const useHardLink = () => Boolean(hardNavigation() || newTab());
  const rel = () => {
    if (local.rel) return local.rel;
    if (newTab() && !local.destination.preserveOpener) {
      return 'noopener noreferrer';
    }
    return undefined;
  };
  const target = () => {
    if (newTab()) {
      return local.target ?? '_blank';
    }
    return local.target;
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

export default UpgradeLink;
