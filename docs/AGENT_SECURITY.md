# Agent Security

Pulse agents incorporate several security mechanisms to ensure that the code running on your infrastructure is authentic and untampered with.

## Self-Update Security

The agent's self-update mechanism is critical for security and stability. To prevent supply chain attacks or compromised update servers from distributing malicious or broken agents, Pulse employs a rigorous verification process.

### 1. Checksum Verification
The agent verifies a SHA-256 checksum of the downloaded binary. The server must provide
`X-Checksum-Sha256`; updates are rejected if the header is missing or mismatched.

### 2. Signature Verification (Optional)
The legacy Docker agent supports optional Ed25519 signature verification when the server provides `X-Signature-Ed25519`. The unified agent relies on checksum verification only. Missing signatures are logged as a warning where supported.

### 3. Pre-Flight Checks
To prevent "brick-updates"—bad updates that crash immediately and require manual recovery—agents perform pre-flight validation before replacing the running executable.

Unified agent (`pulse-agent`):
1. Download new binary.
2. Verify checksum (required).
3. Validate binary magic (ELF/Mach-O/PE) and size limits (100MB max).
4. Make executable and swap atomically.

Legacy Docker agent (`pulse-docker-agent`):
1. Download new binary.
2. Verify checksum (required).
3. Make executable.
4. **Execute with `--self-test`** to validate startup.
5. If the self-test fails, the update is aborted.

## API Security

- **Token Authentication**: All agent-to-server communication requires a valid API token.
- **TLS**: Encrypted by default (unless specifically disabled).
