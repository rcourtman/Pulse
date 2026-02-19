import { Component, For, Show } from 'solid-js';
import { HostNetworkInterface } from '@/types/api';

interface NetworkInterfacesCardProps {
  interfaces?: HostNetworkInterface[];
}

export const NetworkInterfacesCard: Component<NetworkInterfacesCardProps> = (props) => {
  if (!props.interfaces || props.interfaces.length === 0) return null;

  return (
    <div class="rounded border border-slate-200 bg-white p-3 shadow-sm dark:border-slate-600/70 dark:bg-slate-800">
      <div class="text-[11px] font-medium uppercase tracking-wide text-slate-700 dark:text-slate-200 mb-2">Network</div>
      <div class="max-h-[140px] overflow-y-auto custom-scrollbar space-y-2">
        <For each={props.interfaces}>
          {(iface) => (
            <div class="rounded border border-dashed border-slate-200 p-2 dark:border-slate-700 overflow-hidden">
              <div class="flex items-center gap-2 text-[11px] font-medium text-slate-700 dark:text-slate-200 min-w-0">
                <span class="truncate min-w-0">{iface.name}</span>
                <Show when={iface.mac}>
                  <span class="text-[9px] text-slate-400 dark:text-slate-500 font-normal truncate shrink-0 max-w-[100px]" title={iface.mac}>{iface.mac}</span>
                </Show>
              </div>
              <Show when={iface.addresses && iface.addresses.length > 0}>
                <div class="flex flex-wrap gap-1 mt-1">
                  <For each={iface.addresses}>
                    {(ip) => (
                      <span class="inline-block rounded bg-blue-100 px-1.5 py-0.5 text-[10px] text-blue-700 dark:bg-blue-900/40 dark:text-blue-200 max-w-full truncate" title={ip}>
                        {ip}
                      </span>
                    )}
                  </For>
                </div>
              </Show>
            </div>
          )}
        </For>
      </div>
    </div>
  );
};
