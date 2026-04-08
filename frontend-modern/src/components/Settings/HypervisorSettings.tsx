/**
 * HypervisorSettings: Settings page for managing hypervisor/cloud provider connections.
 *
 * Allows users to:
 * - View all configured hypervisors with their connection status
 * - Add new hypervisor connections (VMware, KVM, Nutanix, AWS, Azure, GCP)
 * - Edit existing connections
 * - Test connectivity
 * - Remove connections
 */
import { Component, For, Show, createSignal, createResource, onMount } from 'solid-js';
import { HypervisorAPI, type HypervisorInstance, type HypervisorType, type HypervisorCreateRequest } from '@/api/hypervisors';

// Icons for each provider type.
const providerIcons: Record<string, string> = {
  vmware: 'VM',
  libvirt: 'KVM',
  nutanix: 'NX',
  aws: 'AWS',
  azure: 'AZ',
  gcp: 'GCP',
  hyperv: 'HV',
};

const providerColors: Record<string, string> = {
  vmware: 'bg-blue-600',
  libvirt: 'bg-orange-600',
  nutanix: 'bg-green-600',
  aws: 'bg-yellow-600',
  azure: 'bg-cyan-600',
  gcp: 'bg-red-600',
  hyperv: 'bg-purple-600',
};

export const HypervisorSettings: Component = () => {
  const [instances, setInstances] = createSignal<HypervisorInstance[]>([]);
  const [types, setTypes] = createSignal<HypervisorType[]>([]);
  const [showAddForm, setShowAddForm] = createSignal(false);
  const [editingId, setEditingId] = createSignal<string | null>(null);
  const [testingId, setTestingId] = createSignal<string | null>(null);
  const [testResult, setTestResult] = createSignal<{ id: string; success: boolean; message: string } | null>(null);
  const [error, setError] = createSignal<string | null>(null);

  // Form state
  const [formType, setFormType] = createSignal('vmware');
  const [formName, setFormName] = createSignal('');
  const [formHost, setFormHost] = createSignal('');
  const [formUsername, setFormUsername] = createSignal('');
  const [formPassword, setFormPassword] = createSignal('');
  const [formRegion, setFormRegion] = createSignal('');
  const [formEnabled, setFormEnabled] = createSignal(true);
  const [formVerifySSL, setFormVerifySSL] = createSignal(true);

  const loadData = async () => {
    try {
      const [inst, t] = await Promise.all([HypervisorAPI.list(), HypervisorAPI.getTypes()]);
      setInstances(inst);
      setTypes(t);
    } catch (err: any) {
      setError(err.message);
    }
  };

  onMount(loadData);

  const resetForm = () => {
    setFormType('vmware');
    setFormName('');
    setFormHost('');
    setFormUsername('');
    setFormPassword('');
    setFormRegion('');
    setFormEnabled(true);
    setFormVerifySSL(true);
  };

  const handleAdd = async () => {
    try {
      const req: HypervisorCreateRequest = {
        name: formName(),
        type: formType(),
        host: formHost(),
        username: formUsername(),
        password: formPassword(),
        enabled: formEnabled(),
        verifySSL: formVerifySSL(),
      };
      if (formRegion()) req.region = formRegion();
      await HypervisorAPI.add(req);
      setShowAddForm(false);
      resetForm();
      await loadData();
    } catch (err: any) {
      setError(err.message);
    }
  };

  const handleDelete = async (id: string) => {
    try {
      await HypervisorAPI.remove(id);
      await loadData();
    } catch (err: any) {
      setError(err.message);
    }
  };

  const handleTest = async (id: string) => {
    setTestingId(id);
    setTestResult(null);
    try {
      const result = await HypervisorAPI.test(id);
      setTestResult({ id, success: result.success, message: result.message });
    } catch (err: any) {
      setTestResult({ id, success: false, message: err.message });
    } finally {
      setTestingId(null);
    }
  };

  // Determine which fields to show based on provider type
  const needsHost = () => ['vmware', 'libvirt', 'nutanix', 'hyperv'].includes(formType());
  const needsCredentials = () => ['vmware', 'libvirt', 'nutanix', 'hyperv'].includes(formType());
  const needsRegion = () => ['aws', 'azure', 'gcp'].includes(formType());

  return (
    <div class="space-y-6">
      <div class="flex items-center justify-between">
        <div>
          <h3 class="text-lg font-medium text-gray-200">Hypervisor & Cloud Providers</h3>
          <p class="text-sm text-gray-500 mt-1">
            Connect additional hypervisors and cloud platforms for unified monitoring.
          </p>
        </div>
        <button
          class="inline-flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white text-sm font-medium rounded-lg transition-colors"
          onClick={() => { resetForm(); setShowAddForm(true); }}
        >
          <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M12 4v16m8-8H4" />
          </svg>
          Add Provider
        </button>
      </div>

      {/* Error banner */}
      <Show when={error()}>
        <div class="bg-red-900/30 border border-red-700 rounded-lg p-3 flex items-center justify-between">
          <p class="text-sm text-red-300">{error()}</p>
          <button class="text-red-400 hover:text-red-300" onClick={() => setError(null)}>Dismiss</button>
        </div>
      </Show>

      {/* Instances list */}
      <div class="space-y-3">
        <Show when={instances().length === 0}>
          <div class="bg-gray-800 border border-gray-700 rounded-lg p-8 text-center">
            <svg class="w-12 h-12 mx-auto mb-3 text-gray-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
            </svg>
            <p class="text-gray-400 text-sm">No hypervisor or cloud providers configured.</p>
            <p class="text-gray-500 text-xs mt-1">Add VMware, KVM, Nutanix, AWS, Azure, or GCP connections.</p>
          </div>
        </Show>

        <For each={instances()}>
          {(inst) => (
            <div class="bg-gray-800 border border-gray-700 rounded-lg p-4 flex items-center justify-between">
              <div class="flex items-center gap-4">
                {/* Provider badge */}
                <div class={`w-10 h-10 rounded-lg flex items-center justify-center text-white text-xs font-bold ${providerColors[inst.type] || 'bg-gray-600'}`}>
                  {providerIcons[inst.type] || inst.type.slice(0, 2).toUpperCase()}
                </div>
                <div>
                  <div class="flex items-center gap-2">
                    <span class="text-sm font-medium text-gray-200">{inst.name || inst.id}</span>
                    <span class={`inline-flex items-center px-1.5 py-0.5 text-[10px] font-medium rounded ${inst.enabled ? 'bg-green-900/50 text-green-400' : 'bg-gray-700 text-gray-500'}`}>
                      {inst.enabled ? 'Enabled' : 'Disabled'}
                    </span>
                  </div>
                  <div class="text-xs text-gray-500 mt-0.5">
                    {inst.host || inst.region || inst.projectId || inst.subscriptionId || 'Not configured'}
                    <span class="text-gray-600 ml-2">{inst.type}</span>
                  </div>
                </div>
              </div>
              <div class="flex items-center gap-2">
                {/* Test result */}
                <Show when={testResult()?.id === inst.id}>
                  <span class={`text-xs ${testResult()?.success ? 'text-green-400' : 'text-red-400'}`}>
                    {testResult()?.success ? 'Connected' : 'Failed'}
                  </span>
                </Show>
                <button
                  class="text-xs text-gray-400 hover:text-blue-400 px-2 py-1 rounded border border-gray-600 hover:border-blue-500 disabled:opacity-50"
                  onClick={() => handleTest(inst.id)}
                  disabled={testingId() === inst.id}
                >
                  {testingId() === inst.id ? 'Testing...' : 'Test'}
                </button>
                <button
                  class="text-xs text-red-400 hover:text-red-300 px-2 py-1 rounded border border-gray-600 hover:border-red-500"
                  onClick={() => handleDelete(inst.id)}
                >
                  Remove
                </button>
              </div>
            </div>
          )}
        </For>
      </div>

      {/* Add form modal */}
      <Show when={showAddForm()}>
        <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60" onClick={(e) => { if (e.target === e.currentTarget) setShowAddForm(false); }}>
          <div class="bg-gray-800 rounded-lg shadow-2xl border border-gray-700 w-full max-w-lg p-6">
            <h3 class="text-lg font-medium text-gray-200 mb-4">Add Hypervisor / Cloud Provider</h3>
            <div class="space-y-4">
              {/* Type selector */}
              <div>
                <label class="block text-sm text-gray-400 mb-1">Provider Type</label>
                <div class="grid grid-cols-4 gap-2">
                  <For each={types()}>
                    {(t) => (
                      <button
                        class="px-3 py-2 text-xs rounded-lg border transition-colors text-center"
                        classList={{
                          'border-blue-500 bg-blue-900/30 text-blue-300': formType() === t.type,
                          'border-gray-600 bg-gray-700 text-gray-400 hover:border-gray-500': formType() !== t.type,
                        }}
                        onClick={() => setFormType(t.type)}
                      >
                        <div class="font-medium">{t.name.split(' ')[0]}</div>
                      </button>
                    )}
                  </For>
                </div>
              </div>

              {/* Name */}
              <div>
                <label class="block text-sm text-gray-400 mb-1">Display Name</label>
                <input
                  type="text"
                  class="w-full bg-gray-700 text-gray-200 border border-gray-600 rounded px-3 py-2 text-sm"
                  placeholder="Production vCenter"
                  value={formName()}
                  onInput={(e) => setFormName(e.currentTarget.value)}
                />
              </div>

              {/* Host (for on-prem) */}
              <Show when={needsHost()}>
                <div>
                  <label class="block text-sm text-gray-400 mb-1">
                    {formType() === 'vmware' ? 'vCenter/ESXi Host' :
                     formType() === 'libvirt' ? 'Libvirt URI' :
                     formType() === 'nutanix' ? 'Prism Central Host' : 'Host'}
                  </label>
                  <input
                    type="text"
                    class="w-full bg-gray-700 text-gray-200 border border-gray-600 rounded px-3 py-2 text-sm"
                    placeholder={formType() === 'libvirt' ? 'qemu+ssh://root@kvm-host/system' : 'host.example.com'}
                    value={formHost()}
                    onInput={(e) => setFormHost(e.currentTarget.value)}
                  />
                </div>
              </Show>

              {/* Credentials (for on-prem) */}
              <Show when={needsCredentials()}>
                <div class="grid grid-cols-2 gap-3">
                  <div>
                    <label class="block text-sm text-gray-400 mb-1">Username</label>
                    <input
                      type="text"
                      class="w-full bg-gray-700 text-gray-200 border border-gray-600 rounded px-3 py-2 text-sm"
                      value={formUsername()}
                      onInput={(e) => setFormUsername(e.currentTarget.value)}
                    />
                  </div>
                  <div>
                    <label class="block text-sm text-gray-400 mb-1">Password</label>
                    <input
                      type="password"
                      class="w-full bg-gray-700 text-gray-200 border border-gray-600 rounded px-3 py-2 text-sm"
                      value={formPassword()}
                      onInput={(e) => setFormPassword(e.currentTarget.value)}
                    />
                  </div>
                </div>
              </Show>

              {/* Region (for cloud) */}
              <Show when={needsRegion()}>
                <div>
                  <label class="block text-sm text-gray-400 mb-1">
                    {formType() === 'aws' ? 'AWS Region' :
                     formType() === 'azure' ? 'Azure Region' : 'GCP Zone'}
                  </label>
                  <input
                    type="text"
                    class="w-full bg-gray-700 text-gray-200 border border-gray-600 rounded px-3 py-2 text-sm"
                    placeholder={formType() === 'aws' ? 'us-east-1' : formType() === 'azure' ? 'eastus' : 'us-central1-a'}
                    value={formRegion()}
                    onInput={(e) => setFormRegion(e.currentTarget.value)}
                  />
                </div>
              </Show>

              {/* Options */}
              <div class="flex items-center gap-4">
                <label class="flex items-center gap-2 text-sm text-gray-400 cursor-pointer">
                  <input type="checkbox" checked={formEnabled()} onChange={(e) => setFormEnabled(e.currentTarget.checked)} class="rounded" />
                  Enabled
                </label>
                <Show when={needsHost()}>
                  <label class="flex items-center gap-2 text-sm text-gray-400 cursor-pointer">
                    <input type="checkbox" checked={formVerifySSL()} onChange={(e) => setFormVerifySSL(e.currentTarget.checked)} class="rounded" />
                    Verify SSL
                  </label>
                </Show>
              </div>
            </div>

            <div class="flex justify-end gap-3 mt-6">
              <button
                class="px-4 py-2 text-sm text-gray-400 hover:text-gray-300 border border-gray-600 rounded-lg"
                onClick={() => setShowAddForm(false)}
              >
                Cancel
              </button>
              <button
                class="px-4 py-2 text-sm bg-blue-600 hover:bg-blue-500 text-white rounded-lg font-medium"
                onClick={handleAdd}
              >
                Add Provider
              </button>
            </div>
          </div>
        </div>
      </Show>
    </div>
  );
};
