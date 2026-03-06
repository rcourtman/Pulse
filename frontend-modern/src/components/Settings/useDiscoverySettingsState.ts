import { createSignal } from 'solid-js';

export function useDiscoverySettingsState() {
  const [discoveryEnabled, setDiscoveryEnabled] = createSignal(false);
  const [discoverySubnet, setDiscoverySubnet] = createSignal('auto');
  const [discoveryMode, setDiscoveryMode] = createSignal<'auto' | 'custom'>('auto');
  const [discoverySubnetDraft, setDiscoverySubnetDraft] = createSignal('');
  const [lastCustomSubnet, setLastCustomSubnet] = createSignal('');
  const [discoverySubnetError, setDiscoverySubnetError] = createSignal<string | undefined>();
  const [savingDiscoverySettings, setSavingDiscoverySettings] = createSignal(false);
  let discoverySubnetInputRef: HTMLInputElement | undefined;

  const parseSubnetList = (value: string) => {
    const seen = new Set<string>();
    return value
      .split(',')
      .map((token) => token.trim())
      .filter((token) => {
        if (!token || token.toLowerCase() === 'auto' || seen.has(token)) {
          return false;
        }
        seen.add(token);
        return true;
      });
  };

  const normalizeSubnetList = (value: string) => parseSubnetList(value).join(', ');

  const currentDraftSubnetValue = () => {
    if (discoveryMode() === 'custom') {
      return discoverySubnetDraft();
    }
    const draft = discoverySubnetDraft();
    if (draft.trim() !== '') {
      return draft;
    }
    const saved = discoverySubnet();
    return saved.toLowerCase() === 'auto' ? '' : saved;
  };

  const isValidCIDR = (value: string) => {
    const subnets = parseSubnetList(value);
    if (subnets.length === 0) {
      return false;
    }

    return subnets.every((token) => {
      const [network, prefix] = token.split('/');
      if (!network || typeof prefix === 'undefined') {
        return false;
      }

      const prefixNumber = Number(prefix);
      if (!Number.isInteger(prefixNumber) || prefixNumber < 0 || prefixNumber > 32) {
        return false;
      }

      const octets = network.split('.');
      if (octets.length !== 4) {
        return false;
      }

      return octets.every((octet) => {
        if (octet === '') return false;
        if (!/^\d+$/.test(octet)) return false;
        const valueNumber = Number(octet);
        return valueNumber >= 0 && valueNumber <= 255;
      });
    });
  };

  const applySavedDiscoverySubnet = (subnet?: string | null) => {
    const raw = typeof subnet === 'string' ? subnet.trim() : '';
    if (raw === '' || raw.toLowerCase() === 'auto') {
      setDiscoverySubnet('auto');
      setDiscoveryMode('auto');
      setDiscoverySubnetDraft('');
    } else {
      setDiscoveryMode('custom');
      const normalizedValue = normalizeSubnetList(raw);
      setDiscoverySubnet(normalizedValue);
      setDiscoverySubnetDraft(normalizedValue);
      setLastCustomSubnet(normalizedValue);
      setDiscoverySubnetError(undefined);
      return;
    }
    setDiscoverySubnetError(undefined);
  };

  const assignDiscoverySubnetInputRef = (el: HTMLInputElement) => {
    discoverySubnetInputRef = el;
  };

  const getDiscoverySubnetInputRef = () => discoverySubnetInputRef;

  return {
    discoveryEnabled,
    setDiscoveryEnabled,
    discoverySubnet,
    discoveryMode,
    setDiscoveryMode,
    discoverySubnetDraft,
    setDiscoverySubnetDraft,
    lastCustomSubnet,
    setLastCustomSubnet,
    discoverySubnetError,
    setDiscoverySubnetError,
    savingDiscoverySettings,
    setSavingDiscoverySettings,
    parseSubnetList,
    normalizeSubnetList,
    currentDraftSubnetValue,
    isValidCIDR,
    applySavedDiscoverySubnet,
    assignDiscoverySubnetInputRef,
    getDiscoverySubnetInputRef,
  };
}
