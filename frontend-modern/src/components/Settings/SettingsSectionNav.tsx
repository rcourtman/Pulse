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
    <div class={`border-b border-border ${props.class ?? ''}`} aria-label="Settings sections">
      <div
        class="flex flex-wrap items-center gap-6 overflow-x-auto scrollbar-hide"
        style="-webkit-overflow-scrolling: touch;"
      >
        <For each={allSections}>
          {(section) => {
            const isActive = () => section.id === props.current;
            const Icon = section.icon;

            return (
              <button
                type="button"
                onClick={() => props.onSelect(section.id)}
                class={`inline-flex min-h-10 items-center gap-2 border-b-2 px-1 py-2 text-sm font-medium whitespace-nowrap transition-colors outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2 ${
                  isActive()
                    ? 'border-blue-600 text-base-content'
                    : 'border-transparent text-muted hover:text-base-content'
                }`}
                aria-pressed={isActive()}
              >
                <Icon size={18} stroke-width={2} class="h-4 w-4" />
                <span>{section.label}</span>
              </button>
            );
          }}
        </For>
      </div>
    </div>
  );
};
