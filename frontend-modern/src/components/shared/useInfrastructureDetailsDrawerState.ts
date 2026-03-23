import { createSignal } from 'solid-js';

export type InfrastructureDetailsDrawerTab = 'overview' | 'discovery';

export function useInfrastructureDetailsDrawerState() {
  const [activeTab, setActiveTab] = createSignal<InfrastructureDetailsDrawerTab>('overview');

  return {
    activeTab,
    setActiveTab,
  };
}
