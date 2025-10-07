import type { Component } from 'solid-js';
import { useNavigate } from '@solidjs/router';

type ProxmoxSection = 'overview' | 'storage' | 'backups';

interface ProxmoxSectionNavProps {
  current: ProxmoxSection;
  class?: string;
}

const sections: Array<{
  id: ProxmoxSection;
  label: string;
  path: string;
}> = [
  {
    id: 'overview',
    label: 'Overview',
    path: '/proxmox/overview',
  },
  {
    id: 'storage',
    label: 'Storage',
    path: '/proxmox/storage',
  },
  {
    id: 'backups',
    label: 'Backups',
    path: '/proxmox/backups',
  },
];

export const ProxmoxSectionNav: Component<ProxmoxSectionNavProps> = (props) => {
  const navigate = useNavigate();

  const baseClasses =
    'inline-flex items-center px-2 sm:px-3 py-1 text-xs sm:text-sm font-medium border-b-2 border-transparent text-gray-600 dark:text-gray-400 transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-400/60 focus-visible:ring-offset-2 focus-visible:ring-offset-white dark:focus-visible:ring-offset-gray-900';

  return (
    <div class={`flex flex-wrap items-center gap-3 sm:gap-4 ${props.class ?? ''}`} aria-label="Proxmox sections">
      {sections.map((section) => {
        const isActive = section.id === props.current;
        const classes = isActive
          ? `${baseClasses} text-blue-600 dark:text-blue-300 border-blue-500 dark:border-blue-400`
          : `${baseClasses} hover:text-blue-500 dark:hover:text-blue-300 hover:border-blue-300/70 dark:hover:border-blue-500/50`;

        return (
          <button
            type="button"
            class={classes}
            onClick={() => navigate(section.path)}
            aria-pressed={isActive}
          >
            <span>{section.label}</span>
          </button>
        );
      })}
    </div>
  );
};
