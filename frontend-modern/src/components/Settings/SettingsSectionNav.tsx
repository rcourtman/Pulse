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
  return (
    <div
      class={`flex p-1 space-x-1 bg-gray-100 dark:bg-gray-800 rounded-md overflow-x-auto scrollbar-hide ${props.class ?? ''}`}
      style="-webkit-overflow-scrolling: touch;"
      aria-label="Settings sections"
    >
      <For each={allSections}>
        {(section) => {
          const isActive = () => section.id === props.current;
          const Icon = section.icon;

          return (
            <button
              type="button"
              onClick={() => props.onSelect(section.id)}
              class={`flex flex-1 justify-center sm:flex-none sm:justify-start items-center gap-2 px-3 sm:px-4 py-2.5 sm:py-2 text-sm font-medium rounded-md transition-all whitespace-nowrap outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2 dark:focus-visible:ring-offset-gray-900 ${isActive()
                ? 'bg-white dark:bg-gray-900 border border-gray-200 dark:border-gray-700 text-blue-600 dark:text-blue-400'
                : 'text-gray-600 dark:text-gray-400 border border-transparent hover:text-gray-900 dark:hover:text-gray-200 hover:bg-white dark:hover:bg-gray-800'
                }`}
              aria-pressed={isActive()}
            >
              <Icon size={18} stroke-width={2} class="w-4 h-4 sm:w-[18px] sm:h-[18px]" />
              <span class="hidden sm:inline">{section.label}</span>
              <span class="sm:hidden">{section.label.split(' ').pop()}</span>
            </button>
          );
        }}
      </For>
    </div>
  );
};
