// Decode exactly the single-quoted Bash -c argument, including POSIX apostrophe
// escapes. Consumers can assert its actual source without depending on the
// extra outer quoting layer. Executable PTY tests separately pin real argv.
export const unixBootstrapTestSource = (command: string): string => {
  const child = /bash -c ('(?:[^']|'"'"')*') pulse-bootstrap "\$install_script";/.exec(command);
  if (!child) throw new Error('Expected a single-quoted private Unix bootstrap child');
  return child[1].slice(1, -1).replaceAll(`'"'"'`, "'");
};
