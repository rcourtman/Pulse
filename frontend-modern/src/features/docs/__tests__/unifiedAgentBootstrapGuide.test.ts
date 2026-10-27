import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { renderDocMarkdown } from '../docMarkdown';

const frontendRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../..');
const source = readFileSync(path.join(frontendRoot, '../docs/UNIFIED_AGENT.md'), 'utf8');

function docSection(markdown: string, docPath: string, id: string) {
  const article = document.createElement('article');
  article.innerHTML = renderDocMarkdown(markdown, docPath);
  const heading = article.querySelector(`#${id}`);
  expect(heading, `${docPath}#${id} must be a real rendered destination`).not.toBeNull();
  const section = document.createElement('section');
  for (
    let node = heading!.nextElementSibling;
    node && !/^H[23]$/.test(node.tagName);
    node = node.nextElementSibling
  ) {
    section.append(node.cloneNode(true));
  }
  return section;
}

function setupSection() {
  return docSection(source, 'UNIFIED_AGENT', 'private-file-installation-linux-macos-and-nas');
}

describe('Unified Agent private-file setup help', () => {
  it('renders preparation and download as self-contained fences before installation', () => {
    const section = setupSection();
    const steps = [...section.querySelectorAll(':scope > ol > li')];
    expect(steps).toHaveLength(3);
    const commands = steps.map((step) => step.querySelector('pre code')?.textContent ?? '');
    for (const command of commands.slice(0, 2)) {
      expect(command.trim()).toMatch(/^\(\s+set -eu\s+umask 077/);
      expect(command.trim()).toMatch(/\)$/);
    }
    expect(commands[0]).toContain('vi "$credential_file"');
    expect(commands[1]).toContain('curl --disable --fail');
    expect(commands[1]).toContain('[ "$status" = 200 ]');
    expect(commands[2]).toContain('--token-file "$HOME/.config/pulse/agent-token"');
    expect(commands.join(' ')).not.toMatch(/--token\s|curl[^|]+\|\s*bash/);
  });

  it('keeps stopping, existing-file and retarget trust boundaries beside the commands', () => {
    const steps = [...setupSection().querySelectorAll(':scope > ol > li')];
    const preparation = steps[0].textContent!.replace(/\s+/g, ' ');
    const download = steps[1].textContent!.replace(/\s+/g, ' ');
    expect(preparation).toContain('Stop if preparation fails');
    expect(preparation).toContain('preserves an existing regular token file');
    expect(preparation).toContain('refuses symlinked or non-regular credential paths');
    expect(download).toContain('Stop if the download fails');
    expect(download).toContain('do not execute its temporary file');
    expect(download).toContain('does not follow redirects or overwrite an existing installer');
    expect(download).toContain('retargeting needs the installer from the new server');
    expect(download).toContain("add curl's --cacert after --disable");
    expect(download).toContain('A successful download is not signature verification');
  });

  it('keeps the existing profile link and ships an identical public help asset', () => {
    expect(setupSection().querySelector('a[href="#installation-options"]')).not.toBeNull();
    expect(readFileSync(path.join(frontendRoot, 'public/docs/UNIFIED_AGENT.md'), 'utf8')).toBe(
      source,
    );
  });
});

describe('Stopped agent-command help', () => {
  const failureId = 'copied-agent-command-fails-before-installation';
  const faq = readFileSync(path.join(frontendRoot, '../docs/FAQ.md'), 'utf8');
  const troubleshooting = readFileSync(
    path.join(frontendRoot, '../docs/TROUBLESHOOTING.md'),
    'utf8',
  );
  const normalizedText = (section: HTMLElement) => section.textContent!.replace(/\s+/g, ' ');

  it('makes the same failure destination reachable from setup and troubleshooting', () => {
    for (const [name, markdown] of [
      ['FAQ', faq],
      ['TROUBLESHOOTING', troubleshooting],
    ]) {
      const article = document.createElement('article');
      article.innerHTML = renderDocMarkdown(markdown, name);
      expect(
        article.querySelector(`a[data-doc-link][href="/docs/UNIFIED_AGENT#${failureId}"]`),
      ).not.toBeNull();
      expect(readFileSync(path.join(frontendRoot, `public/docs/${name}.md`), 'utf8')).toBe(
        markdown,
      );
    }
    const setup = docSection(faq, 'FAQ', 'how-do-i-add-a-node');
    expect(
      setup.querySelector(
        'a[href="/docs/UNIFIED_AGENT#private-file-installation-linux-macos-and-nas"]',
      ),
    ).not.toBeNull();
    expect(normalizedText(setup)).toContain('token only at its private prompt');
  });

  it('distinguishes stopped prompt, read-only inspection and destructive removal', () => {
    const section = docSection(source, 'UNIFIED_AGENT', failureId);
    const text = normalizedText(section);
    expect(section.querySelectorAll(':scope > ul > li')).toHaveLength(3);
    for (const fact of [
      'Passing preflight is not installation',
      'A real terminal can still produce this error',
      'do not assume a missing tool from the terminal message alone',
      'standalone read-only action, not removal',
      'The error alone does not prove that removal ran',
      'failed new command does not establish the cause of the earlier outage',
    ]) {
      expect(text).toContain(fact);
    }
    const producer = readFileSync(
      path.join(frontendRoot, 'src/utils/agentInstallCommand.ts'),
      'utf8',
    );
    expect(producer).toContain('--preflight-only');
    expect(producer).toContain('--token-file "$token_file"');
    const installer = readFileSync(path.join(frontendRoot, '../scripts/install.sh'), 'utf8');
    expect(installer).toContain('--safe-profile-inspect is a standalone read-only action');
    expect(installer).toContain('Safe-profile migration is unsupported on this platform');
  });

  it('links to existing private preparation and recovery without another executable recipe', () => {
    const section = docSection(source, 'UNIFIED_AGENT', failureId);
    const article = document.createElement('article');
    article.innerHTML = renderDocMarkdown(source, 'UNIFIED_AGENT');
    for (const id of [
      'private-file-installation-linux-macos-and-nas',
      'agent-lookup-returns-http-401-or-403',
      'auto-update',
      'uninstall',
    ]) {
      expect(section.querySelector(`a[href="#${id}"]`)).not.toBeNull();
      expect(article.querySelector(`#${id}`)).not.toBeNull();
    }
    expect(section.querySelector('pre')).toBeNull();
    for (const boundary of [
      'Stop rather than bypass the private prompt',
      'not an instruction to repeat a failed recovery attempt',
      'name only the flag already used',
      'Do not post the full copied command',
      'do not repeat installation or removal to complete a report',
    ]) {
      expect(normalizedText(section)).toContain(boundary);
    }
  });
});

describe('Agent update failure help', () => {
  const section = () => docSection(source, 'UNIFIED_AGENT', 'agent-not-updating');

  it('separates update stages and manual recovery from future automatic updating', () => {
    const help = section();
    const text = help.textContent!.replace(/\s+/g, ' ');
    const steps = [...help.querySelectorAll(':scope > ol > li')];
    expect(steps.map((step) => step.querySelector('strong')?.textContent)).toEqual([
      'Before download:',
      'Download, validation or helper staging:',
      'Activation, restart or rollback:',
    ]);
    for (const fact of [
      "the server's update target, the agent's last reported version",
      'a stale report does not establish what is running now',
      'A successful checksum, signature or self-test does not establish activation',
      'signed artifact is not the requested pulse-agent version',
      'not a signature-verification failure',
      'A healthy Doctor view after a manual update does not prove future automatic updates work',
      'passing preflight is not installation',
    ]) {
      expect(text).toContain(fact);
    }
  });

  it('offers existing evidence and safe destinations instead of a diagnostic update recipe', () => {
    const help = section();
    const text = help.textContent!.replace(/\s+/g, ' ');
    const article = document.createElement('article');
    article.innerHTML = renderDocMarkdown(source, 'UNIFIED_AGENT');
    for (const id of [
      'collect-agent-logs-safely',
      'agent-lookup-returns-http-401-or-403',
      'copied-agent-command-fails-before-installation',
    ]) {
      expect(help.querySelector(`a[href="#${id}"]`)).not.toBeNull();
      expect(article.querySelector(`#${id}`)).not.toBeNull();
    }
    for (const id of ['find-the-running-server-and-agent-versions', '-getting-help']) {
      expect(help.querySelector(`a[href="/docs/TROUBLESHOOTING#${id}"]`)).not.toBeNull();
    }
    expect(help.querySelector('pre')).toBeNull();
    for (const boundary of [
      'do not restart or repeat an update to fill the gap',
      'Do not bypass signature checks, relax permissions or replace the helper manually',
      'Do not repeat the attempt while its result or installed state is uncertain',
      'Keep tokens, full copied commands, connection files and service environments private',
      'No new update, Diagnostics run or full configuration export is required',
    ]) {
      expect(text).toContain(boundary);
    }
  });

  it('routes failed automatic updates to investigation before a planned manual update', () => {
    const autoUpdate = docSection(source, 'UNIFIED_AGENT', 'auto-update');
    expect(autoUpdate.querySelector('a[href="#agent-not-updating"]')).not.toBeNull();
    expect(autoUpdate.textContent!.replace(/\s+/g, ' ')).toContain(
      'a manual command is not a diagnostic retry or a repair for every automatic-update failure',
    );
    expect(readFileSync(path.join(frontendRoot, 'public/docs/UNIFIED_AGENT.md'), 'utf8')).toBe(
      source,
    );
  });
});
