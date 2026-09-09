# Provider bundle delivery verification — 9 September 2026

The public v6.4.1 release delivered `pulse-provider-msp-v6.4.1.tar.gz` and its
`.sha256` and `.sshsig` sidecars. Retained verification:
- SHA256: a2f3f6df98385e7dbc47e5df9ca014046b8d81eba353420d8f22a668de621b88
- Checksum verification passed.
- SSH signature verification passed using identity `pulse-installer`,
  namespace `pulse-install`, fingerprint
  `SHA256:WjzDnbyb4fF3hPGRE1ZLtYcXzLimGpJq6Ou4opquTV0`.
- Extracted setup.sh lines 23–24 default email empty and source
  `provider_msp_setup`; lines 410–419 form the licence request.
- Offline execution of that exact jq expression passed with absent email and
  synthetic .invalid email, checking the complete field allowlist.

These are retained delivery and offline payload results, not a new installation
test. Setup, key generation, licence activation and customer lookup were not run.
The current documentation regression checks version selection, signature/checksum
commands, request-specific disclosure and byte-identical shipped documentation.
This does not qualify later provider archives or prove server-side acceptance.
