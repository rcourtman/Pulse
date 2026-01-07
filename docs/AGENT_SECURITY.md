# Agent Security

Pulse agents incorporate several security mechanisms to ensure that the code running on your infrastructure is authentic and untampered with.

## Self-Update Security

The agent's self-update mechanism is critical for security and stability. To prevent supply chain attacks or compromised update servers from distributing malicious or broken agents, Pulse employs a rigorous verification process.

### 1. Checksum Verification
The agent verifies a SHA-256 checksum of the downloaded binary. The server must provide
`X-Checksum-Sha256`; updates are rejected if the header is missing or mismatched.

### 2. Pre-Flight Checks
To prevent "brick-updates"—bad updates that crash immediately and require manual recovery—the agent performs a pre-flight check before replacing the running executable.
1. Download new binary.
2. Verify checksum (required).
3. Make executable.
4. **Execute with `--self-test`**: The agent attempts to run the new binary with a special flag that loads the configuration and verifies basic functionality.
5. If the self-test fails (exit code != 0), the update is aborted.

## API Security

- **Token Authentication**: All agent-to-server communication requires a valid API token.
- **TLS**: Encrypted by default (unless specifically disabled).
