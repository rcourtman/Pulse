import type { Component } from 'solid-js';
import { createMemo, For } from 'solid-js';
import { useNavigate } from '@solidjs/router';
import { useWebSocket } from '@/App';

type ProxmoxSection = 'overview' | 'storage' | 'ceph' | 'replication' | 'backups' | 'mail';

interface ProxmoxSectionNavProps {
  current: ProxmoxSection;
  class?: string;
}

const allSections: Array<{
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
      id: 'ceph',
      label: 'Ceph',
      path: '/proxmox/ceph',
    },
    {
      id: 'replication',
      label: 'Replication',
      path: '/proxmox/replication',
    },
    {
      id: 'mail',
      label: 'Mail Gateway',
      path: '/proxmox/mail',
    },
    {
      id: 'backups',
      label: 'Backups',
      path: '/proxmox/backups',
    },
  ];

export const ProxmoxSectionNav: Component<ProxmoxSectionNavProps> = (props) => {
  const navigate = useNavigate();
  const { state } = useWebSocket();

  // Only show tabs if the corresponding feature has data:
  // - Mail Gateway: requires PMG instances
  // - Ceph: requires Ceph clusters (from agent or Proxmox API)
  // - Replication: requires replication jobs
  const sections = createMemo(() => {
    const hasPMG = state.pmg && state.pmg.length > 0;
    const hasCeph = state.cephClusters && state.cephClusters.length > 0;
    const hasReplication = state.replicationJobs && state.replicationJobs.length > 0;
    return allSections.filter((section) =>
      (section.id !== 'mail' || hasPMG) &&
      (section.id !== 'ceph' || hasCeph) &&
      (section.id !== 'replication' || hasReplication)
    );
  });

  const baseClasses =
    'inline-flex items-center px-2 sm:px-3 py-1 text-xs sm:text-sm font-medium border-b-2 border-transparent text-gray-600 dark:text-gray-400 transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-400/60 focus-visible:ring-offset-2 focus-visible:ring-offset-white dark:focus-visible:ring-offset-gray-900';

  return (
    <div class={`flex flex-wrap items-center gap-3 sm:gap-4 ${props.class ?? ''}`} aria-label="Proxmox sections">
      <For each={sections()}>{(section) => {
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
      }}</For>
    </div>
  );
};
