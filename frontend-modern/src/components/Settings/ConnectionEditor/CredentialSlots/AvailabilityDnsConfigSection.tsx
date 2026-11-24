import { Component } from 'solid-js';
import { formControl, formField, formHelpText, formLabel } from '@/components/shared/Form';
import { FormSelect } from '@/components/shared/FormSelect';
import type {
  AvailabilityDNSRecordType,
  AvailabilityDNSTransport,
} from '@/api/availabilityTargets';

export interface AvailabilityDnsConfigSectionProps {
  queryName: string;
  recordType: AvailabilityDNSRecordType;
  transport: AvailabilityDNSTransport;
  expectedValue: string;
  onChange: (patch: {
    dnsQueryName?: string;
    dnsRecordType?: AvailabilityDNSRecordType;
    dnsTransport?: AvailabilityDNSTransport;
    dnsExpectedValue?: string;
  }) => void;
}

const AvailabilityDnsConfigSection: Component<AvailabilityDnsConfigSectionProps> = (props) => {
  return (
    <section
      class="space-y-4 rounded-lg border border-border p-4 sm:col-span-2"
      data-testid="availability-dns-section"
    >
      <div>
        <h3 class="text-sm font-semibold text-base-content">DNS query</h3>
        <p class="mt-1 text-xs text-muted">
          Send a real DNS query to this resolver and verify that it returns NOERROR with at least
          one matching record.
        </p>
      </div>
      <div class="grid gap-4 sm:grid-cols-3">
        <label class={`${formField} sm:col-span-2`}>
          <span class={formLabel}>Query hostname</span>
          <input
            class={formControl}
            value={props.queryName}
            onInput={(event) => props.onChange({ dnsQueryName: event.currentTarget.value })}
            placeholder="example.com or nas.home.arpa"
            data-testid="availability-dns-query-name"
          />
        </label>
        <FormSelect
          label="Record type"
          value={props.recordType}
          onChange={(event) =>
            props.onChange({
              dnsRecordType: event.currentTarget.value as AvailabilityDNSRecordType,
            })
          }
          data-testid="availability-dns-record-type"
        >
          <option value="A">A</option>
          <option value="AAAA">AAAA</option>
          <option value="CNAME">CNAME</option>
          <option value="TXT">TXT</option>
          <option value="MX">MX</option>
          <option value="NS">NS</option>
          <option value="SRV">SRV</option>
          <option value="PTR">PTR</option>
        </FormSelect>
      </div>
      <div class="grid gap-4 sm:grid-cols-3">
        <FormSelect
          label="Transport"
          value={props.transport}
          onChange={(event) =>
            props.onChange({
              dnsTransport: event.currentTarget.value as AvailabilityDNSTransport,
            })
          }
          data-testid="availability-dns-transport"
        >
          <option value="udp">UDP (default)</option>
          <option value="tcp">TCP</option>
        </FormSelect>
        <label class={`${formField} sm:col-span-2`}>
          <span class={formLabel}>Expected answer (optional)</span>
          <input
            class={formControl}
            value={props.expectedValue}
            onInput={(event) => props.onChange({ dnsExpectedValue: event.currentTarget.value })}
            placeholder="192.168.1.50"
            data-testid="availability-dns-expected-value"
          />
          <span class={formHelpText}>
            Leave empty to accept any valid record of that type, or enter an IP, target hostname, or
            TXT substring to match.
          </span>
        </label>
      </div>
    </section>
  );
};

export default AvailabilityDnsConfigSection;
