import { Component, Show, createSignal, createEffect } from 'solid-js';
import { Portal } from 'solid-js/web';
import type { NodeConfig } from '@/types/nodes';

interface NodeModalProps {
  isOpen: boolean;
  onClose: () => void;
  nodeType: 'pve' | 'pbs';
  editingNode?: NodeConfig;
  onSave: (nodeData: Partial<NodeConfig>) => void;
}

export const NodeModal: Component<NodeModalProps> = (props) => {
  const [formData, setFormData] = createSignal({
    name: '',
    host: '',
    authType: 'password' as 'password' | 'token',
    user: '',
    password: '',
    tokenName: '',
    tokenValue: '',
    fingerprint: '',
    verifySSL: true,
    // PVE specific
    monitorVMs: true,
    monitorContainers: true,
    monitorStorage: true,
    monitorBackups: true,
    // PBS specific
    monitorDatastores: true,
    monitorSyncJobs: true,
    monitorVerifyJobs: true,
    monitorPruneJobs: true,
    monitorGarbageJobs: false
  });

  // Update form when editing node changes
  createEffect(() => {
    if (props.editingNode) {
      const node = props.editingNode;
      setFormData({
        name: node.name || '',
        host: node.host || '',
        authType: node.user ? 'password' : 'token',
        user: node.user || '',
        password: '', // Don't show existing password
        tokenName: node.tokenName || '',
        tokenValue: '', // Don't show existing token
        fingerprint: ('fingerprint' in node ? node.fingerprint : '') || '',
        verifySSL: node.verifySSL ?? true,
        monitorVMs: (node.type === 'pve' && 'monitorVMs' in node ? node.monitorVMs : true) ?? true,
        monitorContainers: (node.type === 'pve' && 'monitorContainers' in node ? node.monitorContainers : true) ?? true,
        monitorStorage: (node.type === 'pve' && 'monitorStorage' in node ? node.monitorStorage : true) ?? true,
        monitorBackups: (node.type === 'pve' && 'monitorBackups' in node ? node.monitorBackups : true) ?? true,
        monitorDatastores: (node.type === 'pbs' && 'monitorDatastores' in node ? node.monitorDatastores : true) ?? true,
        monitorSyncJobs: (node.type === 'pbs' && 'monitorSyncJobs' in node ? node.monitorSyncJobs : true) ?? true,
        monitorVerifyJobs: (node.type === 'pbs' && 'monitorVerifyJobs' in node ? node.monitorVerifyJobs : true) ?? true,
        monitorPruneJobs: (node.type === 'pbs' && 'monitorPruneJobs' in node ? node.monitorPruneJobs : true) ?? true,
        monitorGarbageJobs: (node.type === 'pbs' && 'monitorGarbageJobs' in node ? node.monitorGarbageJobs : false) ?? false
      });
    } else {
      // Reset form for new node
      setFormData({
        name: '',
        host: '',
        authType: 'password',
        user: '',
        password: '',
        tokenName: '',
        tokenValue: '',
        fingerprint: '',
        verifySSL: true,
        monitorVMs: true,
        monitorContainers: true,
        monitorStorage: true,
        monitorBackups: true,
        monitorDatastores: true,
        monitorSyncJobs: true,
        monitorVerifyJobs: true,
        monitorPruneJobs: true,
        monitorGarbageJobs: false
      });
    }
  });

  const handleSubmit = (e: Event) => {
    e.preventDefault();
    const data = formData();
    
    // Prepare data based on auth type
    const nodeData: Partial<NodeConfig> = {
      type: props.nodeType,
      name: data.name,
      host: data.host,
      fingerprint: data.fingerprint,
      verifySSL: data.verifySSL
    };

    if (data.authType === 'password') {
      nodeData.user = data.user;
      if (data.password) {
        nodeData.password = data.password;
      }
    } else {
      nodeData.tokenName = data.tokenName;
      if (data.tokenValue) {
        nodeData.tokenValue = data.tokenValue;
      }
    }

    // Add monitor settings based on type
    if (props.nodeType === 'pve') {
      Object.assign(nodeData, {
        monitorVMs: data.monitorVMs,
        monitorContainers: data.monitorContainers,
        monitorStorage: data.monitorStorage,
        monitorBackups: data.monitorBackups
      });
    } else {
      Object.assign(nodeData, {
        monitorDatastores: data.monitorDatastores,
        monitorSyncJobs: data.monitorSyncJobs,
        monitorVerifyJobs: data.monitorVerifyJobs,
        monitorPruneJobs: data.monitorPruneJobs,
        monitorGarbageJobs: data.monitorGarbageJobs
      });
    }

    props.onSave(nodeData);
  };

  const updateField = (field: string, value: string | boolean) => {
    setFormData(prev => ({ ...prev, [field]: value }));
  };

  return (
    <Portal>
      <Show when={props.isOpen}>
        <div class="fixed inset-0 z-50 overflow-y-auto">
          <div class="flex min-h-screen items-center justify-center p-4">
            {/* Backdrop */}
            <div 
              class="fixed inset-0 bg-black/50 transition-opacity"
              onClick={props.onClose}
            />
            
            {/* Modal */}
            <div class="relative w-full max-w-2xl bg-white dark:bg-gray-800 rounded-lg shadow-xl">
              <form onSubmit={handleSubmit}>
                {/* Header */}
                <div class="flex items-center justify-between p-4 border-b border-gray-200 dark:border-gray-700">
                  <h3 class="text-lg font-semibold text-gray-900 dark:text-gray-100">
                    {props.editingNode ? 'Edit' : 'Add'} {props.nodeType === 'pve' ? 'Proxmox VE' : 'Proxmox Backup Server'} Node
                  </h3>
                  <button
                    type="button"
                    onClick={props.onClose}
                    class="text-gray-400 hover:text-gray-500 dark:hover:text-gray-300"
                  >
                    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                      <line x1="18" y1="6" x2="6" y2="18"></line>
                      <line x1="6" y1="6" x2="18" y2="18"></line>
                    </svg>
                  </button>
                </div>
                
                {/* Body */}
                <div class="p-6 space-y-6">
                  {/* Basic Information */}
                  <div>
                    <h4 class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-4">Basic Information</h4>
                    <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                      <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                          Node Name <span class="text-red-500">*</span>
                        </label>
                        <input
                          type="text"
                          value={formData().name}
                          onInput={(e) => updateField('name', e.currentTarget.value)}
                          placeholder="My PVE Node"
                          required
                          class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
                        />
                      </div>
                      
                      <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                          Host URL <span class="text-red-500">*</span>
                        </label>
                        <input
                          type="text"
                          value={formData().host}
                          onInput={(e) => updateField('host', e.currentTarget.value)}
                          placeholder="https://192.168.1.100:8006"
                          required
                          class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
                        />
                      </div>
                    </div>
                  </div>
                  
                  {/* Authentication */}
                  <div>
                    <h4 class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-4">Authentication</h4>
                    
                    {/* Auth Type Selector */}
                    <div class="mb-4">
                      <div class="flex gap-4">
                        <label class="flex items-center">
                          <input
                            type="radio"
                            name="authType"
                            value="password"
                            checked={formData().authType === 'password'}
                            onChange={() => updateField('authType', 'password')}
                            class="mr-2"
                          />
                          <span class="text-sm text-gray-700 dark:text-gray-300">Username & Password</span>
                        </label>
                        <label class="flex items-center">
                          <input
                            type="radio"
                            name="authType"
                            value="token"
                            checked={formData().authType === 'token'}
                            onChange={() => updateField('authType', 'token')}
                            class="mr-2"
                          />
                          <span class="text-sm text-gray-700 dark:text-gray-300">API Token</span>
                        </label>
                      </div>
                    </div>
                    
                    {/* Password Auth Fields */}
                    <Show when={formData().authType === 'password'}>
                      <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                        <div>
                          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                            Username <span class="text-red-500">*</span>
                          </label>
                          <input
                            type="text"
                            value={formData().user}
                            onInput={(e) => updateField('user', e.currentTarget.value)}
                            placeholder="root@pam"
                            required={formData().authType === 'password'}
                            class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
                          />
                        </div>
                        
                        <div>
                          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                            Password {!props.editingNode && <span class="text-red-500">*</span>}
                          </label>
                          <input
                            type="password"
                            value={formData().password}
                            onInput={(e) => updateField('password', e.currentTarget.value)}
                            placeholder={props.editingNode ? 'Leave blank to keep existing' : 'Password'}
                            required={formData().authType === 'password' && !props.editingNode}
                            class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
                          />
                        </div>
                      </div>
                    </Show>
                    
                    {/* Token Auth Fields */}
                    <Show when={formData().authType === 'token'}>
                      <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                        <div>
                          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                            Token Name <span class="text-red-500">*</span>
                          </label>
                          <input
                            type="text"
                            value={formData().tokenName}
                            onInput={(e) => updateField('tokenName', e.currentTarget.value)}
                            placeholder="root@pam!monitoring"
                            required={formData().authType === 'token'}
                            class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
                          />
                        </div>
                        
                        <div>
                          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                            Token Value {!props.editingNode && <span class="text-red-500">*</span>}
                          </label>
                          <input
                            type="password"
                            value={formData().tokenValue}
                            onInput={(e) => updateField('tokenValue', e.currentTarget.value)}
                            placeholder={props.editingNode ? 'Leave blank to keep existing' : 'Token value'}
                            required={formData().authType === 'token' && !props.editingNode}
                            class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
                          />
                        </div>
                      </div>
                    </Show>
                  </div>
                  
                  {/* SSL Settings */}
                  <div>
                    <h4 class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-4">SSL Settings</h4>
                    <div class="space-y-3">
                      <div class="flex items-center">
                        <input
                          type="checkbox"
                          id="verifySSL"
                          checked={formData().verifySSL}
                          onChange={(e) => updateField('verifySSL', e.currentTarget.checked)}
                          class="mr-2"
                        />
                        <label for="verifySSL" class="text-sm text-gray-700 dark:text-gray-300">
                          Verify SSL Certificate
                        </label>
                      </div>
                      
                      <div>
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                          SSL Fingerprint (Optional)
                        </label>
                        <input
                          type="text"
                          value={formData().fingerprint}
                          onInput={(e) => updateField('fingerprint', e.currentTarget.value)}
                          placeholder="AA:BB:CC:DD:EE:FF:..."
                          class="w-full px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 font-mono"
                        />
                      </div>
                    </div>
                  </div>
                  
                  {/* Monitoring Options */}
                  <div>
                    <h4 class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-4">Monitoring Options</h4>
                    <div class="space-y-2">
                      {props.nodeType === 'pve' ? (
                        <>
                          <label class="flex items-center">
                            <input
                              type="checkbox"
                              checked={formData().monitorVMs}
                              onChange={(e) => updateField('monitorVMs', e.currentTarget.checked)}
                              class="mr-2"
                            />
                            <span class="text-sm text-gray-700 dark:text-gray-300">Monitor Virtual Machines</span>
                          </label>
                          <label class="flex items-center">
                            <input
                              type="checkbox"
                              checked={formData().monitorContainers}
                              onChange={(e) => updateField('monitorContainers', e.currentTarget.checked)}
                              class="mr-2"
                            />
                            <span class="text-sm text-gray-700 dark:text-gray-300">Monitor Containers</span>
                          </label>
                          <label class="flex items-center">
                            <input
                              type="checkbox"
                              checked={formData().monitorStorage}
                              onChange={(e) => updateField('monitorStorage', e.currentTarget.checked)}
                              class="mr-2"
                            />
                            <span class="text-sm text-gray-700 dark:text-gray-300">Monitor Storage</span>
                          </label>
                          <label class="flex items-center">
                            <input
                              type="checkbox"
                              checked={formData().monitorBackups}
                              onChange={(e) => updateField('monitorBackups', e.currentTarget.checked)}
                              class="mr-2"
                            />
                            <span class="text-sm text-gray-700 dark:text-gray-300">Monitor Backups</span>
                          </label>
                        </>
                      ) : (
                        <>
                          <label class="flex items-center">
                            <input
                              type="checkbox"
                              checked={formData().monitorDatastores}
                              onChange={(e) => updateField('monitorDatastores', e.currentTarget.checked)}
                              class="mr-2"
                            />
                            <span class="text-sm text-gray-700 dark:text-gray-300">Monitor Datastores</span>
                          </label>
                          <label class="flex items-center">
                            <input
                              type="checkbox"
                              checked={formData().monitorSyncJobs}
                              onChange={(e) => updateField('monitorSyncJobs', e.currentTarget.checked)}
                              class="mr-2"
                            />
                            <span class="text-sm text-gray-700 dark:text-gray-300">Monitor Sync Jobs</span>
                          </label>
                          <label class="flex items-center">
                            <input
                              type="checkbox"
                              checked={formData().monitorVerifyJobs}
                              onChange={(e) => updateField('monitorVerifyJobs', e.currentTarget.checked)}
                              class="mr-2"
                            />
                            <span class="text-sm text-gray-700 dark:text-gray-300">Monitor Verify Jobs</span>
                          </label>
                          <label class="flex items-center">
                            <input
                              type="checkbox"
                              checked={formData().monitorPruneJobs}
                              onChange={(e) => updateField('monitorPruneJobs', e.currentTarget.checked)}
                              class="mr-2"
                            />
                            <span class="text-sm text-gray-700 dark:text-gray-300">Monitor Prune Jobs</span>
                          </label>
                          <label class="flex items-center">
                            <input
                              type="checkbox"
                              checked={formData().monitorGarbageJobs}
                              onChange={(e) => updateField('monitorGarbageJobs', e.currentTarget.checked)}
                              class="mr-2"
                            />
                            <span class="text-sm text-gray-700 dark:text-gray-300">Monitor Garbage Collection Jobs</span>
                          </label>
                        </>
                      )}
                    </div>
                  </div>
                </div>
                
                {/* Footer */}
                <div class="flex items-center justify-end gap-3 px-6 py-4 border-t border-gray-200 dark:border-gray-700">
                  <button
                    type="button"
                    onClick={props.onClose}
                    class="px-4 py-2 text-sm border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors"
                  >
                    Cancel
                  </button>
                  <button
                    type="submit"
                    class="px-4 py-2 text-sm bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors"
                  >
                    {props.editingNode ? 'Update' : 'Add'} Node
                  </button>
                </div>
              </form>
            </div>
          </div>
        </div>
      </Show>
    </Portal>
  );
};