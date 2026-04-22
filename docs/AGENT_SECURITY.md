# Agent Security

Pulse agents incorporate several security mechanisms to ensure that the code running on your infrastructure is authentic and untampered with.

## Self-Update Security

The agent's self-update mechanism is critical for security and stability. To prevent supply chain attacks or compromised update servers from distributing malicious or broken agents, Pulse employs a rigorous verification process.

### 1. Checksum Verification
The agent verifies a SHA-256 checksum of the downloaded binary. The server must provide
`X-Checksum-Sha256`; updates are rejected if the header is missing or mismatched.

### 2. Signature Verification
Release builds embed trusted Ed25519 update public keys and require
`X-Signature-Ed25519` in addition to the checksum header. Updates are rejected
when the signature is missing or does not verify against the embedded trust
root.

### 3. Pre-Flight Checks
To prevent "brick-updates"—bad updates that crash immediately and require manual recovery—agents perform pre-flight validation before replacing the running executable.

Unified agent (`pulse-agent`):
1. Download new binary.
2. Verify checksum (required).
3. Verify the Ed25519 release signature when trusted update keys are embedded.
4. Validate binary magic (ELF/Mach-O/PE) and size limits (100MB max).
5. Make executable and swap atomically.

## API Security

- **Token Authentication**: All agent-to-server communication requires a valid API token.
- **TLS**: Encrypted by default (unless specifically disabled).
