import type { Component, JSX } from 'solid-js';
import { useNavigate } from '@solidjs/router';
import { ProxmoxIcon } from '@/components/icons/ProxmoxIcon';

type ProxmoxSection = 'overview' | 'storage' | 'backups';

interface ProxmoxSectionNavProps {
  current: ProxmoxSection;
  class?: string;
}

const sections: Array<{
  id: ProxmoxSection;
  label: string;
  path: string;
  icon: () => JSX.Element;
}> = [
  {
    id: 'overview',
    label: 'Overview',
    path: '/proxmox/overview',
    icon: () => <ProxmoxIcon class="w-3.5 h-3.5" />,
  },
  {
    id: 'storage',
    label: 'Storage',
    path: '/proxmox/storage',
    icon: () => (
      <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <ellipse cx="12" cy="5" rx="9" ry="3"></ellipse>
        <path d="M21 12c0 1.66-4 3-9 3s-9-1.34-9-3"></path>
        <path d="M3 5v14c0 1.66 4 3 9 3s9-1.34 9-3V5"></path>
      </svg>
    ),
  },
  {
    id: 'backups',
    label: 'Backups',
    path: '/proxmox/backups',
    icon: () => (
      <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <rect x="3" y="3" width="18" height="18" rx="2" ry="2"></rect>
        <line x1="3" y1="9" x2="21" y2="9"></line>
        <line x1="9" y1="21" x2="9" y2="9"></line>
      </svg>
    ),
  },
];

export const ProxmoxSectionNav: Component<ProxmoxSectionNavProps> = (props) => {
  const navigate = useNavigate();

  const baseClasses =
    'inline-flex items-center gap-1 px-2 sm:px-3 py-1 rounded-full border text-xs sm:text-sm transition-colors focus:outline-none focus-visible:ring focus-visible:ring-blue-400';

  return (
    <div class={`flex flex-wrap items-center gap-1 sm:gap-2 ${props.class ?? ''}`} aria-label="Proxmox sections">
      {sections.map((section) => {
        const Icon = section.icon;
        const isActive = section.id === props.current;
        const classes = isActive
          ? `${baseClasses} bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300 border-blue-200 dark:border-blue-700 shadow-sm`
          : `${baseClasses} border-transparent text-gray-600 dark:text-gray-400 hover:bg-gray-200 dark:hover:bg-gray-700/60`;

        return (
          <button
            type="button"
            class={classes}
            onClick={() => navigate(section.path)}
            aria-pressed={isActive}
          >
            <Icon />
            <span>{section.label}</span>
          </button>
        );
      })}
    </div>
  );
};
