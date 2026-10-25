import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { renderDocMarkdown } from '../docMarkdown';

const frontendRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../..');
const repoRoot = path.resolve(frontendRoot, '..');
const readDoc = (name: string) => readFileSync(path.join(repoRoot, 'docs', `${name}.md`), 'utf8');
const articleFor = (name: string) => {
  const article = document.createElement('article');
  article.innerHTML = renderDocMarkdown(readDoc(name), name);
  return article;
};

function safetySection() {
  const article = articleFor('KUBERNETES');
  const heading = article.querySelector('#proxmox-backup-safety');
  expect(heading).not.toBeNull();
  const section = document.createElement('section');
  for (let node = heading!.nextElementSibling; node && node.tagName !== 'H2';) {
    const next = node.nextElementSibling;
    section.appendChild(node);
    node = next;
  }
  return section;
}

const text = (element: Element) => element.textContent!.replace(/\s+/g, ' ');

describe('Kubernetes server backup safety in shipped documentation', () => {
  it('puts server safety before installation, separately from the collector', () => {
    const article = articleFor('KUBERNETES');
    const headings = [...article.querySelectorAll('h2')].map((heading) => heading.id);
    expect(headings.indexOf('proxmox-backup-safety')).toBeGreaterThanOrEqual(0);
    expect(headings.indexOf('proxmox-backup-safety')).toBeLessThan(
      headings.indexOf('1-deploying-the-pulse-server'),
    );
    const safety = safetySection();
    expect(text(safety)).toContain('including with API-only monitoring');
    expect(text(safety)).toContain('stopping its DaemonSet does not stop the server');
    expect(safety.querySelectorAll('ol > li')).toHaveLength(4);
    // No copied scale/delete/Helm command can discard the required identity,
    // reconciliation and prior-state checks when used on a different cluster.
    expect(safety.querySelector('pre')).toBeNull();
  });

  it('blocks pod shutdown when state would be lost, matching the chart storage branches', () => {
    const section = safetySection();
    const copy = text(section);
    for (const phrase of [
      'Check data persistence before stopping pods',
      'effective data paths use retained persistent storage',
      'deleting those pods can lose configuration, history and credentials',
      'persistence.enabled=false uses emptyDir',
      'If persistence is unknown or state is ephemeral, stop here',
      'do not scale down as a diagnostic',
      'A configuration export alone does not cover full-state recovery',
    ])
      expect(copy).toContain(phrase);
    const persistence = [...section.querySelectorAll('p')].find((p) =>
      p.textContent?.includes('Check data persistence'),
    )!;
    expect(persistence.compareDocumentPosition(section.querySelector('ol')!)).toBe(
      Node.DOCUMENT_POSITION_FOLLOWING,
    );
    const chart = readFileSync(
      path.join(repoRoot, 'deploy/helm/pulse/templates/deployment.yaml'),
      'utf8',
    );
    expect(chart).toContain('.Values.persistence.enabled');
    expect(chart).toContain('persistentVolumeClaim:');
    expect(chart).toContain('emptyDir: {}');
    expect(section.querySelector('a[href="/docs/MIGRATION#full-state-recovery"]')).not.toBeNull();
    expect(articleFor('MIGRATION').querySelector('#full-state-recovery')).not.toBeNull();
  });

  it('requires a controller-aware stop, not pod deletion or a paused rollout', () => {
    const steps = [...safetySection().querySelectorAll('ol > li')].map(text);
    for (const phrase of [
      'every Pulse server',
      'servers outside this cluster',
      'actual desired replica count',
      'autoscaler, GitOps reconciler and automatic updater',
      'unknown owner or another polling server',
    ])
      expect(steps[0]).toContain(phrase);
    for (const phrase of [
      'before deliberately scaling the server to zero',
      'Deleting a pod only invites its controller to replace it',
      'pausing a Deployment rollout does not stop existing pods',
      'every previous server process has terminated normally',
      'no replacement is polling',
      'missing termination evidence is not a confirmed stop',
      'Do not force deletion, uninstall Helm, delete PVCs or recreate Secrets',
      'Pulse monitoring and alerts are unavailable while stopped',
    ])
      expect(steps[1]).toContain(phrase);
  });

  it('keeps unknown operations stopped and requires independent every-filesystem recovery', () => {
    const section = safetySection();
    const step = text(section.querySelectorAll('ol > li')[2]);
    for (const phrase of [
      'does not cancel a guest-agent request already issued',
      'if their state is unknown, do not start the planned backup',
      'After the backup ends',
      'independent of Pulse and the QEMU Guest Agent',
      'thaw, fresh successful workload writes to every filesystem covered by the backup, and workload liveness',
      'a write to only the OS disk is not enough',
      'If any check fails or is unavailable, keep the pause',
      'without a new backup or probe',
    ])
      expect(step).toContain(phrase);
    expect(text(section)).toContain('not incident recovery or an automatic backup hook');
    expect(text(section)).toContain('do not restart pods or send guest-agent probes');
    expect(text(section)).toContain(
      'does not establish that a native freeze/thaw defect has been fixed',
    );
  });

  it('restores prior state without replaying installation or losing credentials', () => {
    const step = text(safetySection().querySelectorAll('ol > li')[3]);
    for (const phrase of [
      'after all checks pass',
      "each server's original desired count",
      'controls that were active beforehand',
      'previously zero-replica server stays at zero',
      'unknown prior state is not permission to start it',
      'Preserve the existing image, PVCs, configuration, identities and credentials',
      'monitoring is fresh before relying on alerts again',
      'Do not use an install command from this page as a restart or recovery shortcut',
    ])
      expect(step).toContain(phrase);
  });

  it('connects both deployment guides and troubleshooting to real shipped safety headings', () => {
    const kubernetes = articleFor('KUBERNETES');
    const vm = articleFor('VM_DISK_MONITORING');
    const backupLink = kubernetes.querySelector('a[href="/docs/VM_DISK_MONITORING#backup-safety"]');
    expect(backupLink).not.toBeNull();
    expect(vm.querySelector('#backup-safety')).not.toBeNull();
    expect(vm.querySelector('a[href="/docs/KUBERNETES#proxmox-backup-safety"]')).not.toBeNull();
    const troubleshooting = kubernetes.querySelector('[id="4-troubleshooting"]')!;
    const warning = troubleshooting.nextElementSibling!;
    expect(warning.querySelector('a[href="#proxmox-backup-safety"]')).not.toBeNull();
    expect(text(warning)).toContain("A Kubernetes pod's Ready state does not prove guest thaw");
    for (const name of ['KUBERNETES', 'VM_DISK_MONITORING']) {
      expect(readFileSync(path.join(frontendRoot, 'public/docs', `${name}.md`), 'utf8')).toBe(
        readDoc(name),
      );
    }
  });
});
