# Resilient DNS Resolution for Pulse

## Overview

Pulse now includes a resilient DNS resolution feature that helps handle DNS failures when connecting to Proxmox VE or PBS instances. This is particularly useful for environments with:

- Round-robin DNS configurations
- Multiple A records for a single hostname
- DNS servers that may occasionally fail
- Hosts with `.lan` domains that use local DNS servers

## How It Works

When resilient DNS is enabled for an endpoint, Pulse will:

1. **Resolve all IP addresses** for the hostname
2. **Cache DNS results** for 1 minute to reduce DNS queries
3. **Try each IP address** in sequence if one fails
4. **Mark failed IPs** and skip them for 30 seconds
5. **Use stale cache** if DNS resolution completely fails
6. **Remember working IPs** and prioritize them for future requests

## Configuration

### Automatic Enablement

Resilient DNS is automatically enabled for any endpoint with a hostname ending in `.lan`.

### Manual Configuration

You can manually enable resilient DNS for any endpoint by setting environment variables:

#### For Proxmox VE Endpoints

```bash
# Primary endpoint
PROXMOX_RESILIENT_DNS=true

# Additional endpoints
PROXMOX_RESILIENT_DNS_1=true
PROXMOX_RESILIENT_DNS_2=true
```

#### For PBS Endpoints

```bash
# Primary PBS
PBS_RESILIENT_DNS=true

# Additional PBS instances
PBS_RESILIENT_DNS_1=true
PBS_RESILIENT_DNS_2=true
```

## Example Configuration

```bash
# Example with round-robin DNS
PROXMOX_HOST=proxmox.lan
PROXMOX_PORT=8006
PROXMOX_TOKEN_ID=user@pam!token
PROXMOX_TOKEN_SECRET=secret-uuid
PROXMOX_RESILIENT_DNS=true  # Optional for .lan domains

# Example with custom domain
PROXMOX_HOST_1=cluster.example.com
PROXMOX_PORT_1=8006
PROXMOX_TOKEN_ID_1=user@pam!token
PROXMOX_TOKEN_SECRET_1=secret-uuid
PROXMOX_RESILIENT_DNS_1=true  # Required for non-.lan domains
```

## Troubleshooting

### Testing DNS Resolution

You can test DNS resolution manually using standard tools:

```bash
# Test DNS resolution with nslookup
nslookup proxmox.lan

# Test with dig for more details
dig proxmox.lan

# Test connectivity to resolved IPs
ping $(nslookup proxmox.lan | grep Address | tail -1 | cut -d' ' -f2)
```

For detailed DNS behavior testing, you can enable debug logging:
```bash
DEBUG=pulse:dns npm run dev
```

This will show:
- All resolved IP addresses
- Cache behavior
- Failed IP handling
- Hostname extraction from URLs

### Log Messages

When resilient DNS is enabled, you'll see messages like:

```
[ApiClients] Creating resilient client for hostname: proxmox.lan
[DnsResolver] Resolved proxmox.lan to: 192.168.1.10, 192.168.1.11, 192.168.1.12
[ResilientApiClient] Attempting request to proxmox.lan via IP 192.168.1.10
[ResilientApiClient] Request failed for proxmox.lan via IP 192.168.1.10: ECONNREFUSED
[DnsResolver] Marking host as failed: 192.168.1.10
[ResilientApiClient] Attempting request to proxmox.lan via IP 192.168.1.11
```

### Common Issues

1. **"DNS resolution failed" errors**
   - Verify the hostname is correct
   - Check your DNS server is responding
   - Try using `nslookup` or `dig` to test DNS resolution

2. **All IPs marked as failed**
   - Check if the Proxmox/PBS service is running
   - Verify firewall rules allow access
   - Wait 30 seconds for IPs to be retried

3. **SSL certificate errors with IP addresses**
   - The resilient client adds the proper `Host` header
   - Ensure your Proxmox certificates include the hostname
   - Consider using `PROXMOX_ALLOW_SELF_SIGNED_CERTS=true` for testing

## Performance Considerations

- DNS results are cached for 1 minute
- Failed IPs are skipped for 30 seconds
- Each IP attempt has its own timeout
- The last working IP is tried first on subsequent requests

## Disabling Resilient DNS

If you need to disable resilient DNS for a `.lan` domain:

```bash
PROXMOX_HOST=proxmox.lan
PROXMOX_RESILIENT_DNS=false  # Explicitly disable
```

## Security Notes

- The resilient DNS client maintains proper SSL/TLS validation
- The original hostname is used for certificate validation via the `Host` header
- Self-signed certificate settings are preserved