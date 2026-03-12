export async function resolveComposeInvocation(canRun) {
  if (await canRun('docker', ['compose', 'version'])) {
    return {
      command: 'docker',
      args: ['compose', '-f', 'docker-compose.test.yml', 'up', '-d'],
      label: 'modern docker compose',
    };
  }

  if (await canRun('docker-compose', ['version'])) {
    return {
      command: 'docker-compose',
      args: ['-f', 'docker-compose.test.yml', 'up', '-d'],
      label: 'legacy docker-compose',
    };
  }

  throw new Error(
    'Neither `docker compose` nor `docker-compose` is available. Install Docker, or set PULSE_E2E_SKIP_DOCKER=1 when the test environment is already provisioned.',
  );
}
