import type { Component } from 'solid-js';
import { For } from 'solid-js';
import Server from 'lucide-solid/icons/server';
import HardDrive from 'lucide-solid/icons/hard-drive';
import Mail from 'lucide-solid/icons/mail';

type SettingsSection = 'pve' | 'pbs' | 'pmg';

interface SettingsSectionNavProps {
  current: SettingsSection;
  onSelect: (section: SettingsSection) => void;
  class?: string;
}

const allSections: Array<{
  id: SettingsSection;
  label: string;
  icon: typeof Server;
}> = [
    {
      id: 'pve',
      label: 'Virtual Environment',
      icon: Server,
    },
    {
      id: 'pbs',
      label: 'Backup Server',
      icon: HardDrive,
    },
    {
      id: 'pmg',
      label: 'Mail Gateway',
      icon: Mail,
    },
  ];

export const SettingsSectionNav: Component<SettingsSectionNavProps> = (props) => {
  const baseClasses =
    'inline-flex items-center gap-2 px-2 sm:px-3 py-1 text-xs sm:text-sm font-medium border-b-2 border-transparent text-gray-600 dark:text-gray-400 transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-400/60 focus-visible:ring-offset-2 focus-visible:ring-offset-white dark:focus-visible:ring-offset-gray-900';

  return (
    <div class={`flex flex-wrap items-center gap-2 sm:gap-4 ${props.class ?? ''}`} aria-label="Settings sections">
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
              <Icon size={14} stroke-width={2} class="sm:w-4 sm:h-4" />
              <span class="whitespace-nowrap">{section.label}</span>
            </button>
          );
        }}
      </For>
    </div>
  );
};
