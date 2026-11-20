import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import { createGuideFixture } from './guideFixture';

const article = createGuideFixture((name) => readFileSync(`public/docs/${name}.md`, 'utf8'));

describe('HTTPS setup in Help', () => {
  it('renders a systemd drop-in rather than shell assignments', () => {
    const help = article('CONFIGURATION');
    const systemd = help.querySelector('#https-on-systemd');
    const code = systemd?.nextElementSibling?.nextElementSibling;
    expect(code?.tagName).toBe('PRE');
    expect(code?.textContent?.trim().split('\n')).toEqual([
      '[Service]',
      'Environment=HTTPS_ENABLED=true \\',
      '  "TLS_CERT_FILE=/etc/pulse/cert.pem" \\',
      '  "TLS_KEY_FILE=/etc/pulse/key.pem"',
    ]);
    expect(help.querySelector('a[href="#https-on-systemd"]')?.textContent).toBe('systemd');
    expect(help.querySelector('a[href="#https-in-docker"]')?.textContent).toBe('Docker');
    expect(help.querySelector('a[href="#redirect-http-to-https"]')?.textContent).toBe(
      'HTTP redirect',
    );
  });

  it('leads from the verified HTTPS check to the private-file agent setup', () => {
    const help = article('CONFIGURATION');
    const check = [...help.querySelectorAll('pre')].find((block) =>
      block.textContent?.includes('https://pulse.example:7655/api/health'),
    );
    expect(check?.textContent?.trim()).toMatch(/^curl --disable /);
    expect(check?.textContent).toContain('--cacert /path/to/trusted-cert.pem');
    expect(check?.textContent).not.toMatch(/--insecure|\s-k\s|[Aa]uthorization/);
    const target = 'private-file-installation-linux-macos-and-nas';
    expect(help.querySelector(`a[href="/docs/UNIFIED_AGENT#${target}"]`)).not.toBeNull();
    expect(article('UNIFIED_AGENT').querySelector(`#${target}`)).not.toBeNull();
  });
});
