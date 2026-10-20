import { describe, expect, it } from 'vitest';
import { getProRegistryLogin, PRO_REGISTRY_PASSWORD_NOTE } from '../proRegistryLogin';

const image = 'license.pulserelay.pro/pulse-pro';
const legacy =
  "printf '%s' '<activation-key>' | docker login license.pulserelay.pro -u 'pulse-pro' --password-stdin";
const prompt = "docker login license.pulserelay.pro --username 'pulse-pro'";

describe('protected registry sign-in presentation', () => {
  it.each([undefined, ''])('omits an absent login (%s)', (loginCommand) => {
    expect(getProRegistryLogin({ image, loginCommand })).toBeNull();
  });

  it.each([legacy, prompt, prompt.replace('--username', '-u')])(
    'rebuilds a recognised login as Docker’s interactive password prompt: %s',
    (loginCommand) => {
      const result = getProRegistryLogin({ image, loginCommand });
      expect(result).toEqual({ command: prompt, note: PRO_REGISTRY_PASSWORD_NOTE });
      expect(result?.command).not.toMatch(/activation-key|password-stdin|printf|ppk_live_/);
    },
  );

  it('preserves a matching custom registry port and inert username', () => {
    expect(
      getProRegistryLogin({
        image: 'registry.example.invalid:5443/pulse/pulse-pro',
        loginCommand: "docker login registry.example.invalid:5443 -u 'operator_1@example.invalid'",
      })?.command,
    ).toBe("docker login registry.example.invalid:5443 --username 'operator_1@example.invalid'");
  });

  it.each([
    legacy.replace('<activation-key>', 'ppk_live_SYNTHETIC_NOT_A_CREDENTIAL'),
    legacy.replace("'%s'", '"%s"'),
    prompt + '; echo injected',
    prompt + '\necho injected',
    prompt + ' --password-stdin',
    prompt.replace('license.pulserelay.pro', 'unrelated.example.invalid'),
    prompt.replace('license.pulserelay.pro', 'https://license.pulserelay.pro'),
    prompt.replace('license.pulserelay.pro', '-receiver'),
    prompt.replace('license.pulserelay.pro', 'license..pulserelay.pro'),
    prompt.replace('pulse-pro', 'ppk_live_SYNTHETIC_NOT_A_CREDENTIAL'),
    prompt.replace('pulse-pro', '-option'),
    prompt.replace('pulse-pro', 'x'.repeat(129)),
    prompt.replace('pulse-pro', "operator'$(echo injected)'"),
    'echo synthetic-secret | docker login license.pulserelay.pro -u pulse-pro --password-stdin',
  ])('withholds unknown, secret-bearing or mismatched input: %s', (loginCommand) => {
    const result = getProRegistryLogin({ image, loginCommand });
    expect(result?.command).toBeUndefined();
    expect(result?.note).toContain('could not be checked safely');
    expect(result?.note).toContain('https://pulserelay.pro/download.html');
    expect(JSON.stringify(result)).not.toContain(loginCommand);
    expect(JSON.stringify(result)).not.toMatch(/SYNTHETIC|synthetic-secret|injected/);
  });
});
