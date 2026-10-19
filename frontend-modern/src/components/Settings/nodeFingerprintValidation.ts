// Match pkg/tlsutil/fingerprint.go: Proxmox removes colons and ignores hex
// case, but does not strip spaces or a SHA256: label. Never repair or clear a
// pin silently: that would change the requested trust policy.
export const getNodeFingerprintError = (fingerprint: string): string | null => {
  if (fingerprint === '' || /^[0-9a-f]{64}$/i.test(fingerprint.replace(/:/g, ''))) return null;
  return 'Enter a SHA-256 certificate fingerprint with exactly 64 hexadecimal characters, optionally separated by colons. Do not include spaces or a SHA256: label. Verify it through an already-trusted administration channel. Valid format does not prove trust.';
};
