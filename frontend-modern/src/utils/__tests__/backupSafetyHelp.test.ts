import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { renderDocMarkdown } from '@/features/docs/docMarkdown';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../..');
const read = (name: string) => readFileSync(path.join(root, name), 'utf8');
const guide = read('docs/VM_DISK_MONITORING.md');
const section =
  guide.split('### Pause Pulse for a planned freeze-enabled backup')[1]?.split('## 🚀 Setup')[0] ??
  '';
const words = section.replace(/\s+/g, ' ');

function render(markdown: string, name: string): HTMLElement {
  const article = document.createElement('article');
  article.innerHTML = renderDocMarkdown(markdown, name);
  return article;
}

describe('manual backup safety help', () => {
  it('makes missing-reading guidance observational, OS-specific and safe before setup', () => {
    const rendered = render(guide, 'VM_DISK_MONITORING');
    const troubleshooting = rendered.querySelector(
      '#a-missing-reading-is-not-an-installation-diagnosis',
    );
    expect(troubleshooting?.textContent).toBe('A missing reading is not an installation diagnosis');
    const explanation = guide
      .split('### A missing reading is not an installation diagnosis')[1]
      .split('| Observation |')[0];
    const text = render(explanation, 'VM_DISK_MONITORING').textContent?.replace(/\s+/g, ' ');
    expect(text).toContain('does not establish whether an agent is absent or stopped');
    expect(text).toContain(
      'Do not install, enable or restart an agent solely to clear a disk dash',
    );
    expect(text).toContain('defer setup and live probes');
    expect(text).toContain('every filesystem covered by the backup');
    expect(text).toContain('workload liveness');
    expect(text).toContain('Restore only services and timers that were previously active');
    expect(text).toContain('not a Windows or Android installation instruction');
    expect(text).toContain('missing Pulse usage is unknown, not zero');
    expect(render(explanation, 'VM_DISK_MONITORING').querySelector('pre')).toBeNull();
    for (const fragment of ['backup-safety', '-setup']) {
      const link = render(explanation, 'VM_DISK_MONITORING').querySelector(
        `a[href="#${fragment}"]`,
      );
      expect(link).not.toBeNull();
      expect(rendered.querySelector(`#${fragment}`)).not.toBeNull();
    }
    // This broad status comes from the current collector, not a reproduced
    // native failure or a new diagnosis of the guest's service state.
    const classifier = read('internal/monitoring/guest_disk_stability.go');
    expect(classifier).toMatch(
      /case strings.Contains\(errStr, "500"\):\s+return "agent-not-running"/,
    );
  });

  it('ships the complete precaution and makes it reachable from troubleshooting', () => {
    expect(read('frontend-modern/public/docs/VM_DISK_MONITORING.md')).toBe(guide);
    const troubleshooting = read('docs/TROUBLESHOOTING.md');
    expect(read('frontend-modern/public/docs/TROUBLESHOOTING.md')).toBe(troubleshooting);
    const link = render(troubleshooting, 'TROUBLESHOOTING').querySelector(
      'a[href="/docs/VM_DISK_MONITORING#pause-pulse-for-a-planned-freeze-enabled-backup"]',
    );
    expect(link?.textContent).toBe('manual backup precaution');
    expect(
      render(guide, 'VM_DISK_MONITORING').querySelector(
        '#pause-pulse-for-a-planned-freeze-enabled-backup',
      )?.textContent,
    ).toBe('Pause Pulse for a planned freeze-enabled backup');
  });

  it('keeps service control in the actual Pulse server container, not the guest or host agent', () => {
    expect(words).toContain('inside the Pulse LXC');
    expect(words).toContain('not the Proxmox host or the backed-up VM');
    expect(words).toContain('pulse-backend.service');
    expect(words).toContain('not pulse-agent.service');
    expect(words).toContain('LoadState=loaded');
    expect(words).toContain('If the unit is missing');
  });

  it('checks updater state before server stop and preserves the original timer state', () => {
    expect(words).toContain('Record whether the timer was active');
    expect(words).toContain('let it finish normally before continuing');
    expect(words).toContain('Do not interrupt an installation');
    expect(words).toContain('only if it was active beforehand');
    const blocks = [...render(section, 'VM_DISK_MONITORING').querySelectorAll('pre code')].map(
      (code) => code.textContent?.trim(),
    );
    expect(blocks).toEqual([
      'systemctl show pulse.service --property=LoadState,ActiveState,MainPID',
      'systemctl show pulse-update.timer --property=LoadState,ActiveState',
      'sudo systemctl stop pulse-update.timer\nsystemctl show pulse-update.timer pulse-update.service \\\n  --property=Id,LoadState,ActiveState,MainPID',
      'sudo systemctl stop pulse.service\nsystemctl show pulse.service --property=LoadState,ActiveState,MainPID',
      'sudo systemctl start pulse.service\nsystemctl is-active pulse.service',
      'sudo systemctl start pulse-update.timer',
    ]);
    // These defaults and the stopped-service condition are from the existing
    // installer, not a new service or a promise about a custom deployment.
    const installer = read('install.sh');
    expect(installer).toContain('DEFAULT_SERVICE_NAME="pulse"');
    expect(installer).toContain('SERVICE_NAME="pulse-backend"');
    expect(installer).toContain(
      "ExecCondition=/bin/sh -c 'systemctl is-active --quiet ${service_name}'",
    );
  });

  it('requires stopped-state readback and discloses the whole monitoring outage', () => {
    expect(words).toContain('ActiveState=inactive');
    expect(words).toContain('MainPID=0');
    expect(words).toContain('Do not start the backup if');
    expect(words).toContain('Pulse monitoring and alerts are unavailable');
    expect(words).toContain('Do not run an update, start another Pulse instance');
    expect(words).toContain('repeat the precaution for each affected backup window');
  });

  it('requires independent workload writes before restart, never merely an OK task or an open console', () => {
    expect(words).toContain('Keep Pulse stopped until');
    expect(words).toContain('fresh successful workload writes');
    expect(words).toContain('console connection or a successful read alone is not enough');
    expect(words).toContain('not proof that polling or alerts have recovered');
    expect(words).toContain('normal observation times');
    expect(words).toContain('without Run Diagnostics or manual guest-agent probes');
    expect(guide).toContain('An OK backup task or an absent VM lock');
    expect(guide).toContain('does not prove thaw succeeded');
    expect(guide).toContain('not a claim\nthat the monitoring defect is fixed');
  });

  it('does not turn a planned precaution into an incident recovery or an automatic restart hook', () => {
    expect(words).toContain('not a recovery procedure');
    expect(words).toContain('not an automatic backup hook');
    expect(words).toContain('If thaw cannot be confirmed, leave Pulse stopped');
    expect(guide).toContain(
      'Do not test freeze/thaw commands, clear backup locks, disable backup freezing',
    );
    const commands = [...render(section, 'VM_DISK_MONITORING').querySelectorAll('pre code')]
      .map((code) => code.textContent)
      .join('\n');
    expect(commands).not.toMatch(
      /qm |pct |pvesh |guest-fsfreeze|restart|disable|enable|mask|reset|rm |kill|curl|wget|Environment/,
    );
  });
});
