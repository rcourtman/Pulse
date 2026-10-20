import type { DockerUpdateCommands } from '@/api/updates';

export const PRO_REGISTRY_PASSWORD_NOTE =
  'Run interactively. At Docker’s Password prompt, enter the key from Settings → License. Input is hidden. Never put it in a shell command. Docker may save it locally.';

const UNRECOGNISED_LOGIN_NOTE =
  'Registry sign-in instructions could not be checked safely. See https://pulserelay.pro/download.html. Never put your key in a shell command.';

// Never render or copy arbitrary broker login text. In particular, old brokers
// put a key placeholder in printf: substituting it records the key in shell
// history even with --password-stdin. Extract only the non-secret identity from
// that exact template or a plain Docker password-prompt command, then rebuild
// the latter. Unknown templates fail closed without echoing their contents.
export function getProRegistryLogin(
  docker: Pick<DockerUpdateCommands, 'image' | 'loginCommand'>,
): { command?: string; note: string } | null {
  if (!docker.loginCommand) return null;
  const legacy =
    /^printf '%s' '<activation-key>' \| docker login ([a-zA-Z0-9.:-]+) -u '([a-zA-Z0-9_.@-]+)' --password-stdin$/;
  const prompt = /^docker login ([a-zA-Z0-9.:-]+) (?:-u|--username) '([a-zA-Z0-9_.@-]+)'$/;
  const match = legacy.exec(docker.loginCommand) ?? prompt.exec(docker.loginCommand);
  if (!match) return { note: UNRECOGNISED_LOGIN_NOTE };

  const [, registry, username] = match;
  // Bind login to the image's registry, not an unrelated credential receiver.
  // Restrict host/port and username to inert shell tokens; a secret in place of
  // a username is not a supported login identity either.
  if (
    !/^[a-zA-Z0-9]+(?:[.-][a-zA-Z0-9]+)*(?::[0-9]{1,5})?$/.test(registry) ||
    registry !== docker.image.split('/')[0] ||
    username.length > 128 ||
    username.startsWith('-') ||
    username.includes('ppk_live_')
  ) {
    return { note: UNRECOGNISED_LOGIN_NOTE };
  }
  return {
    command: `docker login ${registry} --username '${username}'`,
    note: PRO_REGISTRY_PASSWORD_NOTE,
  };
}
