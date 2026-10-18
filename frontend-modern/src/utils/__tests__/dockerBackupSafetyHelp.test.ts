import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { renderDocMarkdown } from '@/features/docs/docMarkdown';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../..');
const read = (name: string) => readFileSync(path.join(root, name), 'utf8');
const guide = read('docs/VM_DISK_MONITORING.md');
const heading = '### Pause a Docker or Compose server for a planned backup';
const fragment = 'pause-a-docker-or-compose-server-for-a-planned-backup';
const section = guide.split(heading)[1]?.split('## 🚀 Setup')[0] ?? '';

function render(markdown: string, name = 'VM_DISK_MONITORING'): HTMLElement {
  const article = document.createElement('article');
  article.innerHTML = renderDocMarkdown(markdown, name);
  return article;
}

const text = () => render(section).textContent?.replace(/\s+/g, ' ');
const stateCommand =
  'docker inspect --format \'Id={{.Id}} Running={{.State.Running}} Paused={{.State.Paused}} Restarting={{.State.Restarting}} Status={{.State.Status}} Pid={{.State.Pid}} ExitCode={{.State.ExitCode}} OOMKilled={{.State.OOMKilled}}\' "$PULSE_CONTAINER_ID"';

describe('Docker backup-pause help', () => {
  it('ships the procedure and routes both Docker and systemd readers to its real heading', () => {
    expect(section).not.toBe('');
    expect(read('frontend-modern/public/docs/VM_DISK_MONITORING.md')).toBe(guide);
    const docker = read('docs/DOCKER.md');
    expect(read('frontend-modern/public/docs/DOCKER.md')).toBe(docker);
    expect(render(guide).querySelector(`#${fragment}`)?.textContent).toBe(
      heading.replace('### ', ''),
    );
    expect(
      render(docker, 'DOCKER').querySelector(`a[href="/docs/VM_DISK_MONITORING#${fragment}"]`)
        ?.textContent,
    ).toBe('planned server pause and checked restoration');
    expect(render(guide).querySelector(`a[href="#${fragment}"]`)?.textContent).toBe(
      'container pause procedure',
    );
    expect(render(docker, 'DOCKER').textContent).toContain(
      'an OK backup alone does not prove recovery',
    );
  });

  it('identifies the server and every existing ID without exposing environment or configuration', () => {
    expect(text()).toContain('on its Docker host');
    expect(text()).toContain(
      'not a monitored workload, the backed-up VM or a Pulse Agent container',
    );
    expect(text()).toContain('original project directory');
    expect(text()).toContain('same project/file options');
    expect(text()).toContain('Record every returned server container ID privately');
    expect(text()).toContain('An empty list or a failed command is not proof');
    expect(text()).toContain('Repeat the state check for every server');
    expect(text()).toContain('Do not share full docker inspect');
    const commands = [...render(section).querySelectorAll('pre code')].map((e) => e.textContent);
    expect(commands).toEqual([
      'docker compose ps --all --quiet pulse\n',
      `PULSE_CONTAINER_ID='paste-the-recorded-server-container-id'\n${stateCommand}\n`,
      `docker stop --timeout 60 "$PULSE_CONTAINER_ID"\n${stateCommand}\n`,
      'docker start "$PULSE_CONTAINER_ID"\n',
    ]);
    expect(commands.join('\n')).not.toMatch(
      /\.Config|\.Mounts|\.Env|compose config|\brm\b|\bkill\b|\bexec\b|\brun\b|\bup\b|\bdown\b|\bpull\b|\bprune\b|qm |pct |pvesh /,
    );
  });

  it('requires stopped identity, shutdown evidence and restart controls rather than equating a stop with safety', () => {
    for (const state of [
      'same recorded ID',
      'Running=false',
      'Paused=false',
      'Restarting=false',
      'Status=exited',
      'Pid=0',
      'ExitCode=0',
      'OOMKilled=false',
    ])
      expect(text()).toContain(state);
    expect(text()).toContain('including 137 after forced termination');
    expect(text()).toContain('identity changes or any state is unknown, do not start the backup');
    expect(text()).toContain(
      'A container stop does not cancel a guest-agent request already issued',
    );
    expect(text()).toContain('do not start the backup on the strength of a stopped container');
    expect(text()).toContain('Swarm, Kubernetes and other controllers');
    expect(text()).toContain('does not stop a controller from replacing it');
    expect(text()).toContain('does not prevent an external updater or another operator');
    expect(text()).toContain('Do not change the restart policy, image, mounts or saved deployment');
    expect(text()).toContain('Let an in-progress update finish normally');
    expect(read('docker-compose.yml')).toContain('restart: unless-stopped');
    expect(read('docker-compose.yml')).toContain('container_name: pulse');
  });

  it('gates restoration on independent post-backup thaw, every covered write and workload liveness', () => {
    expect(text()).toContain(
      'evidence from after it ended, independent of Pulse and the QEMU Guest Agent',
    );
    expect(text()).toContain(
      'thaw, fresh successful workload writes to every filesystem covered by the backup, and workload liveness',
    );
    expect(text()).toContain('not forced writes, test files or another freeze/thaw cycle');
    expect(text()).toContain('a console connection or a successful read alone is not enough');
    expect(text()).toContain(
      'If any check fails or is unavailable, leave Pulse and its automatic updater paused',
    );
    expect(text()).toContain('without guest-agent probes or repeating the backup');
    expect(text()).toContain('not incident recovery or an automatic backup hook');
    expect(text()).toContain('Pulse monitoring and alerts are unavailable');
    expect(text()).toContain('independent outage coverage');
  });

  it('restores only prior-running identities and prior-active jobs, never a replacement or inactive peer', () => {
    expect(text()).toContain('Only for a container recorded as running before the pause');
    expect(text()).toContain('same existing ID');
    expect(text()).toContain('A previously stopped container stays stopped');
    expect(text()).toContain('original state is unknown, do not guess');
    expect(text()).toContain('Do not use a blanket Compose start or up');
    expect(text()).toContain(
      'If the original container is missing or startup fails, keep automation paused',
    );
    expect(text()).toContain('rather than creating another server against the same data');
    expect(text()).toContain('Running state is not proof of healthy collection or alert delivery');
    expect(text()).toContain('Restore only automatic jobs that were active before the pause');
    expect(text()).toContain('they will not start a previously inactive server');
    expect(text()).toContain(
      'Unknown original job state or restart behaviour means leave that job paused',
    );
  });
});
