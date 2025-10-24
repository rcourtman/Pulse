import type { Component } from 'solid-js';
import { For } from 'solid-js';
import SquareTerminal from 'lucide-solid/icons/square-terminal';

type HostsSection = 'linux' | 'macos' | 'windows';

interface HostsSectionNavProps {
  current: HostsSection;
  onSelect: (section: HostsSection) => void;
  class?: string;
}

const AppleIcon = () => (
  <svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
    <path d="M14.94 5.19A4.38 4.38 0 0 0 16 2a4.44 4.44 0 0 0-3 1.52 4.17 4.17 0 0 0-1 3.09 3.69 3.69 0 0 0 2.94-1.42zm2.52 7.44a4.51 4.51 0 0 1 2.16-3.81 4.66 4.66 0 0 0-3.66-2c-1.56-.16-3 .91-3.83.91s-2-.89-3.3-.87A4.92 4.92 0 0 0 4.69 9.39C2.93 12.45 4.24 17 6 19.47c.8 1.21 1.8 2.58 3.12 2.53s1.75-.82 3.28-.82 2 .82 3.3.79 2.22-1.24 3.06-2.45a11 11 0 0 0 1.38-2.85 4.41 4.41 0 0 1-2.68-4.08z" />
  </svg>
);

const WindowsIcon = () => (
  <svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
    <path d="M0 3.449L9.75 2.1v9.451H0m10.949-9.602L24 0v11.4H10.949M0 12.6h9.75v9.451L0 20.699M10.949 12.6H24V24l-12.9-1.801" />
  </svg>
);

const allSections: Array<{
  id: HostsSection;
  label: string;
  icon: Component<{ size?: number; 'stroke-width'?: number }>;
}> = [
  {
    id: 'linux',
    label: 'Linux',
    icon: SquareTerminal,
  },
  {
    id: 'macos',
    label: 'macOS',
    icon: AppleIcon,
  },
  {
    id: 'windows',
    label: 'Windows',
    icon: WindowsIcon,
  },
];

export const HostsSectionNav: Component<HostsSectionNavProps> = (props) => {
  const baseClasses =
    'inline-flex items-center gap-2 px-2 sm:px-3 py-1 text-xs sm:text-sm font-medium border-b-2 border-transparent text-gray-600 dark:text-gray-400 transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-400/60 focus-visible:ring-offset-2 focus-visible:ring-offset-white dark:focus-visible:ring-offset-gray-900';

  return (
    <div class={`flex flex-wrap items-center gap-3 sm:gap-4 ${props.class ?? ''}`} aria-label="Host platform sections">
      <For each={allSections}>
        {(section) => {
          const isActive = () => section.id === props.current;
          const classes = () => isActive()
            ? `${baseClasses} text-blue-600 dark:text-blue-300 border-blue-500 dark:border-blue-400`
            : `${baseClasses} hover:text-blue-500 dark:hover:text-blue-300 hover:border-blue-300/70 dark:hover:border-blue-500/50`;

          const Icon = section.icon;

          return (
            <button
              type="button"
              class={classes()}
              onClick={() => props.onSelect(section.id)}
              aria-pressed={isActive()}
            >
              <Icon size={16} stroke-width={2} />
              <span>{section.label}</span>
            </button>
          );
        }}
      </For>
    </div>
  );
};
