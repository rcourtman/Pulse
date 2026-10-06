import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { renderDocMarkdown } from '@/features/docs/docMarkdown';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../..');
const read = (name: string) => readFileSync(path.join(root, name), 'utf8');
const guide = read('docs/VM_DISK_MONITORING.md');
const section =
  guide
    .split('### Pause Pulse for a planned freeze-enabled backup')[1]
    ?.split('### Pause a Docker or Compose server for a planned backup')[0] ?? '';
const words = section.replace(/\s+/g, ' ');

function render(markdown: string, name: string): HTMLElement {
  const article = document.createElement('article');
  article.innerHTML = renderDocMarkdown(markdown, name);
  return article;
}

describe('manual backup safety help', () => {
  it('puts the complete restart boundary in the opening backup warning', () => {
    const opening = guide.split('### Pause Pulse for a planned freeze-enabled backup')[0];
    const text = render(opening, 'VM_DISK_MONITORING').textContent?.replace(/\s+/g, ' ');
    expect(text).toContain('until the backup has ended');
    expect(text).toContain(
      'thaw, fresh successful writes to every filesystem covered by the backup, and workload liveness',
    );
    expect(text).toContain('not Pulse readings or guest-agent probes');
    expect(text).toContain('Restore only services and timers that were active before the pause');
    expect(text).toContain('Pulse monitoring and alerts are unavailable');
  });

  it('requires post-backup evidence for every covered filesystem without manufacturing writes', () => {
    const text = render(section, 'VM_DISK_MONITORING').textContent?.replace(/\s+/g, ' ');
    expect(text).toContain('after the backup ended');
    expect(text).toContain('independent of Pulse and the QEMU Guest Agent');
    expect(text).toContain('a write to only the OS disk');
    expect(text).toContain('not forced writes, test-file commands or a new freeze/thaw cycle');
    expect(text).toContain('If any check is unavailable or fails, leave Pulse stopped');
  });

  it('renders each prior-state restoration choice without starting an originally inactive server', () => {
    const rows = [...render(section, 'VM_DISK_MONITORING').querySelectorAll('tbody tr')].map(
      (row) => [...row.querySelectorAll('td')].map((cell) => cell.textContent?.trim()),
    );
    expect(rows.map((row) => row.slice(0, 2))).toEqual([
      ['Active', 'Active'],
      ['Active', 'Inactive'],
      ['Inactive', 'Active'],
      ['Inactive', 'Inactive'],
    ]);
    expect(rows[0][2]).toContain('then restore and check the timer');
    expect(rows[1][2]).toContain('leave the timer inactive');
    expect(rows[2][2]).toContain('Leave Pulse inactive');
    expect(rows[2][2]).toContain('confirmed not to start an inactive Pulse server');
    expect(rows[2][2]).toContain('otherwise keep it paused');
    expect(rows[3][2]).toBe('Leave both inactive.');
    expect(words).toContain(
      'If either pre-pause state is unknown, do not guess or start either unit',
    );
    expect(words).toContain('If startup fails or its state is unknown, keep the timer paused');
    expect(words).toContain('A persistent timer may run a missed update immediately');
    expect(words).toContain('an older or customised installed unit may differ');
    expect(words).toContain('does not prove that an update or monitoring succeeded');
    expect(read('install.sh')).toContain('Persistent=true');
  });

  it('does not turn a FAQ disk dash into mandatory installation or unsafe recovery', () => {
    const faq = read('docs/FAQ.md');
    expect(read('frontend-modern/public/docs/FAQ.md')).toBe(faq);
    const section = faq.split('### Why do VMs show "-" for disk usage?')[1]?.split('\n### ')[0];
    expect(section).toBeTruthy();
    const rendered = render(section!, 'FAQ');
    const text = rendered.textContent?.replace(/\s+/g, ' ');
    expect(text).toContain('unavailable, not zero');
    expect(text).toContain('does not establish that the agent is absent or stopped');
    expect(text).toContain('rather than treating a retained Pulse value as current');
    expect(text).toContain(
      'Do not install, enable or restart an agent solely to clear a disk dash',
    );
    expect(text).toContain('defer setup and live probes');
    expect(text).toContain('an OK backup does not prove thaw');
    expect(text).toContain('monitoring-outage precaution');
    expect(text).toContain('fresh writes to every covered filesystem and workload liveness');
    expect(text).toContain('Restore only services and timers that were previously active');
    expect(text).toContain('not a Windows or Android installation instruction');
    expect(text).not.toMatch(/You must install|Enable it in VM Options|qm agent/);
    expect(rendered.querySelector('pre')).toBeNull();
    const target = render(guide, 'VM_DISK_MONITORING');
    for (const [label, fragment] of [
      ['Backup safety', 'backup-safety'],
      ['Missing-reading and setup guidance', 'a-missing-reading-is-not-an-installation-diagnosis'],
    ]) {
      const link = rendered.querySelector(`a[href="/docs/VM_DISK_MONITORING#${fragment}"]`);
      expect(link?.textContent).toBe(label);
      expect(target.querySelector(`#${fragment}`)).not.toBeNull();
    }
  });

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
    // Keep this conservative for older or retained explanations too. The
    // current collector uses typed errors; safe help must not require the
    // retired HTTP-500 misclassification to remain in the implementation.
  });

  it('ships the complete precaution and makes it reachable from troubleshooting', () => {
    expect(read('frontend-modern/public/docs/VM_DISK_MONITORING.md')).toBe(guide);
    const troubleshooting = read('docs/TROUBLESHOOTING.md');
    expect(read('frontend-modern/public/docs/TROUBLESHOOTING.md')).toBe(troubleshooting);
    const link = render(troubleshooting, 'TROUBLESHOOTING').querySelector(
      'a[href="/docs/VM_DISK_MONITORING#backup-safety"]',
    );
    expect(link?.textContent).toBe('manual backup precaution');
    const target = render(guide, 'VM_DISK_MONITORING');
    expect(target.querySelector('#backup-safety')?.textContent).toBe('Backup safety');
    for (const fragment of [
      'pause-pulse-for-a-planned-freeze-enabled-backup',
      'pause-a-docker-or-compose-server-for-a-planned-backup',
    ]) {
      expect(target.querySelector(`#${fragment}`)).not.toBeNull();
    }
  });

  const troubleshootingDiskHelp = () => {
    const section = read('docs/TROUBLESHOOTING.md')
      .split('#### VMs show "-" for disk usage')[1]
      .split('#### Backup health disagrees with PBS')[0];
    const article = render(section, 'TROUBLESHOOTING');
    return { article, text: article.textContent?.replace(/\s+/g, ' ') };
  };

  it('keeps the troubleshooting entry point deployment-aware and separate from incident recovery', () => {
    const { article, text } = troubleshootingDiskHelp();
    expect(text).toContain('actual server deployment: systemd or Docker/Compose');
    expect(text).toContain('A planned pause is not incident recovery');
    expect(text).toContain('stopping Pulse does not cancel a guest-agent request already issued');
    expect(text).toContain("If an existing operation's state is unknown, do not start a backup");
    expect(text).toContain('a stopped server or an elapsed wait');
    expect(article.querySelector('pre')).toBeNull();
  });

  it('requires all independent recovery checks in troubleshooting, not merely thaw', () => {
    const { text } = troubleshootingDiskHelp();
    expect(text).toContain('until the backup has ended and independent post-backup checks confirm');
    expect(text).toContain(
      'thaw, fresh successful workload writes to every filesystem covered by the backup, and workload liveness',
    );
    expect(text).toContain('independent of Pulse and the QEMU Guest Agent');
    expect(text).toContain('a console connection or a successful read alone is not enough');
    expect(text).toContain('If any check fails or is unavailable');
    expect(text).toContain('leave Pulse and its automatic updater paused');
    expect(text).toContain('not new probes, forced writes or another backup');
    expect(text).not.toContain('thaw confirmation before starting Pulse again');
  });

  it('preserves prior-active restoration and independent outage coverage in troubleshooting', () => {
    const { text } = troubleshootingDiskHelp();
    expect(text).toContain('After all checks pass');
    expect(text).toContain('restore only services and timers that were active before the pause');
    expect(text).toContain('following the deployment-specific precaution');
    expect(text).toContain('Unknown pre-pause states are not permission to start them');
    expect(text).toContain('Pulse monitoring and alerts are unavailable while stopped');
    expect(text).toContain('arrange independent outage coverage');
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
      'sudo systemctl stop pulse.service\nsystemctl show pulse.service \\\n  --property=LoadState,ActiveState,MainPID,Result,ExecMainCode,ExecMainStatus',
      'sudo systemctl start pulse.service\nsystemctl is-active pulse.service',
      'sudo systemctl start pulse-update.timer\nsystemctl is-active pulse-update.timer',
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

  it('checks the new shutdown result without clearing failure or starting a previously inactive server', () => {
    const text = render(section, 'VM_DISK_MONITORING').textContent?.replace(/\s+/g, ' ');
    expect(read('frontend-modern/public/docs/VM_DISK_MONITORING.md')).toBe(guide);
    expect(text).toContain('For a server stopped in this step');
    expect(text).toContain('Result=success');
    expect(text).toContain('ExecMainCode=1 (normal process exit)');
    expect(text).toContain('ExecMainStatus=0');
    expect(text).toContain('signal termination, non-zero exit or unavailable shutdown result');
    expect(text).toContain('Do not force-kill Pulse or clear its failed state');
    expect(text).toContain('an old exit result is not evidence of a new shutdown');
  });

  it('does not equate a stopped service or waiting period with completion of an issued guest request', () => {
    const text = render(section, 'VM_DISK_MONITORING').textContent?.replace(/\s+/g, ' ');
    expect(text).toContain('Stopping Pulse does not cancel a guest-agent request already issued');
    expect(text).toContain('Even a successful process exit does not prove');
    expect(text).toContain(
      'Let existing guest/backup operations finish normally before the planned backup',
    );
    expect(text).toContain('not new guest-agent probes');
    expect(text).toContain('If their state is unknown, do not start the backup');
    expect(text).toContain('a stopped service or an arbitrary waiting period');
    expect(text).toContain('A disk dash or cooldown is not evidence of completion');
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
    expect(words).toContain('If any check is unavailable or fails, leave Pulse stopped');
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
  it('offers existing Machines disk readings without assuming identity, freshness or a repaired link', () => {
    expect(read('frontend-modern/public/docs/VM_DISK_MONITORING.md')).toBe(guide);
    const alternative = guide
      .split('### Use existing Machines readings without changing the guest')[1]
      ?.split('| Observation |')[0];
    expect(alternative).toBeTruthy();
    const rendered = render(alternative!, 'VM_DISK_MONITORING');
    const text = rendered.textContent?.replace(/\s+/g, ' ');
    expect(text).toContain('use different collection paths');
    expect(text).toContain('already has a Pulse Agent');
    expect(text).toContain('existing entry in Machines');
    expect(text).toContain('same guest');
    expect(text).toContain('not a similar name alone');
    expect(text).toContain(
      "a retained History sample or an agent's recent contact is not proof of a current disk reading",
    );
    expect(text).toContain('does not repair the missing Proxmox reading');
    expect(text).toContain('prove that the two entries are linked correctly');
    expect(text).toContain('no current filesystem readings, use its own filesystem tools');
    expect(rendered.querySelector('pre')).toBeNull();
  });

  it('keeps the Machines workaround separate from guest changes and independent backup recovery', () => {
    const alternative = guide
      .split('### Use existing Machines readings without changing the guest')[1]
      ?.split('| Observation |')[0];
    expect(alternative).toBeTruthy();
    const rendered = render(alternative!, 'VM_DISK_MONITORING');
    const text = rendered.textContent?.replace(/\s+/g, ' ');
    expect(text).toContain(
      'Do not install another agent, restart services, restore a VM or force guest-agent checks',
    );
    expect(text).toContain('does not relax Backup safety');
    expect(text).toContain(
      'A responsive guest, running agent services, working Machines readings or an expired cooldown is not proof of thaw',
    );
    expect(text).toContain('successful writes to every filesystem covered by the backup');
    expect(text).toContain('Keep the monitoring-outage precaution');
    expect(text).toContain('do not clear or bypass a guest-read pause');
    const target = render(guide, 'VM_DISK_MONITORING');
    expect(
      target.querySelector('#use-existing-machines-readings-without-changing-the-guest'),
    ).not.toBeNull();
    expect(rendered.querySelector('a[href="#backup-safety"]')).not.toBeNull();
    expect(target.querySelector('#backup-safety')).not.toBeNull();
  });
});
