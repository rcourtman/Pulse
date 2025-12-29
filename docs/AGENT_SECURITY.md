# Agent Security

Pulse agents incorporate several security mechanisms to ensure that the code running on your infrastructure is authentic and untampered with.

## Self-Update Security

The agent's self-update mechanism is critical for security and stability. To prevent supply chain attacks or compromised update servers from distributing malicious or broken agents, Pulse employs a rigorous verification process.

### 1. Cryptographic Signature Verification
All agent binaries are signed using **Ed25519** signatures. The agent contains a hardcoded list of trusted public keys. Before an update is applied, the agent verifies the digital signature of the downloaded binary against these trusted keys.

- **Key Rotation**: The agent supports multiple trusted keys, allowing for seamless key rotation. A new key can be introduced in one version, and the old key retired in a future version.
- **Fail-Safe**: If a binary is not signed or the signature is invalid, the update is strictly rejected.

### 2. Checksum Verification
In addition to the cryptographic signature, the agent verifies the SHA-256 checksum of the downloaded binary to ensure integrity and prevent transmission errors.

### 3. Pre-Flight Checks
To prevent "brick-updates"—bad updates that crash immediately and require manual recovery—the agent performs a pre-flight check before replacing the running executable.
1. Download new binary.
2. Verify signature and checksum.
3. Make executable.
4. **Execute with `--self-test`**: The agent attempts to run the new binary with a special flag that loads the configuration and verifies basic functionality.
5. If the self-test fails (exit code != 0), the update is aborted.

## API Security

- **Token Authentication**: All agent-to-server communication requires a valid API token.
- **TLS**: Encrypted by default (unless specifically disabled).
