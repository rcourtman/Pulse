# Pulse Security Guide

## Overview

Pulse is designed as an internal monitoring tool for Proxmox environments with enterprise-grade security built in. Unlike traditional monitoring tools that use plaintext configuration files, Pulse automatically encrypts all sensitive data.

## How Pulse Security Works

### Automatic Encryption

- **All credentials are encrypted** using AES-256-GCM encryption
- **No plaintext passwords** are ever stored on disk
- **Encryption is automatic** - you don't need to configure anything
- **Keys are derived from machine ID** - unique per installation

### Secure by Default

When you add credentials through the web UI:
1. Data is sent over your network (use HTTPS in production)
2. Pulse encrypts the credentials immediately
3. Encrypted data is stored in `/etc/pulse/*.enc` files
4. Files are automatically secured with 0600 permissions
5. Only the Pulse service can decrypt the data

### What Gets Encrypted

- Proxmox node credentials (passwords and API tokens)
- Email server passwords  
- Webhook URLs with embedded tokens
- Any other sensitive configuration data

## Best Practices

### For Home Labs

Even in trusted environments, Pulse's encryption provides peace of mind:
- Credentials are never exposed in backups
- Config files can't be accidentally shared
- Protection against disk access by other users

### For Production

1. **Use HTTPS** - Put Pulse behind a reverse proxy with SSL
2. **Use API Tokens** - More secure than passwords for Proxmox
3. **Network Isolation** - Run Pulse in a management network
4. **Access Control** - Use reverse proxy authentication
5. **Regular Updates** - Keep Pulse updated for security patches

## Security Architecture

```
┌─────────────┐     HTTPS      ┌─────────────┐
│   Browser   │ ─────────────> │  Pulse UI   │
└─────────────┘                └─────────────┘
                                      │
                               ┌──────┴──────┐
                               │   Encrypt   │
                               │  AES-256    │
                               └──────┬──────┘
                                      │
                               ┌──────┴──────┐
                               │   Storage   │
                               │   *.enc     │
                               │  (0600)     │
                               └─────────────┘
```

## File Permissions

Pulse automatically manages file permissions:

```bash
/etc/pulse/
├── nodes.enc      (0600) # Encrypted node credentials
├── email.enc      (0600) # Encrypted email settings
├── webhooks.json  (0600) # Webhook configurations
├── alerts.json    (0600) # Alert thresholds
└── system.json    (0600) # System settings
```

## Comparison with Other Tools

| Feature | Traditional Tools | Pulse |
|---------|------------------|-------|
| Password Storage | Plaintext YAML/ENV | Encrypted AES-256 |
| Configuration | Manual file editing | Web UI only |
| File Permissions | User responsibility | Automatic 0600 |
| Secrets Management | Complex setup | Built-in |
| Credential Rotation | Manual process | Simple UI update |

## Advanced Security Options

### Using External Secrets (Optional)

While Pulse's built-in encryption is sufficient for most users, you can reference external secrets if required by your security policies:

1. **Environment Variables**: In the UI, use `${VAR_NAME}` as a value
2. **File References**: Use `file:///path/to/secret` as a value

These are resolved at runtime, but the UI-based encrypted storage is recommended for simplicity.

### Proxy Authentication

For additional security, place Pulse behind an authenticating reverse proxy:

```nginx
location / {
    auth_basic "Pulse Monitoring";
    auth_basic_user_file /etc/nginx/.htpasswd;
    proxy_pass http://localhost:7655;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
}
```

## Security FAQ

**Q: Where is the encryption key stored?**
A: The key is derived from your machine ID and system characteristics. It's never stored directly.

**Q: Can I backup the encrypted files?**
A: Yes, encrypted files are safe to backup. They can only be decrypted on the original system.

**Q: What if I need to migrate to new hardware?**
A: You'll need to reconfigure through the UI. This is by design for security.

**Q: Is the encryption FIPS compliant?**
A: Pulse uses Go's standard crypto libraries with AES-256-GCM, which meets FIPS requirements.

**Q: Can I audit the encryption implementation?**
A: Yes, the source code is open. See `/internal/crypto/crypto.go` in the repository.

## Reporting Security Issues

If you discover a security vulnerability:

1. **Do NOT** create a public GitHub issue
2. Email security concerns to the maintainer
3. Allow reasonable time for a fix before disclosure
4. Help us improve security for all users

## Summary

Pulse provides enterprise-grade security out of the box:
- ✅ Automatic encryption of all credentials
- ✅ Secure file permissions
- ✅ No plaintext secrets
- ✅ Simple and secure by default
- ✅ No complex configuration needed

Just install Pulse, configure through the UI, and your credentials are automatically protected.