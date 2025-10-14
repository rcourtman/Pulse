# Pi-hole Nebula Sync Notes

- Primary Pi-hole: delly CT 114 (192.168.0.102)  
  Secondary: minipc CT 202 (192.168.0.101)  
  Virtual IP: 192.168.0.100
- Runs on the delly host (not inside containers).
- Binary: `/usr/local/bin/nebula-sync`; wrapper script: `/usr/local/bin/pihole-sync.sh`.
- Cron job (`root@delly`): `*/30 * * * *` → logs written to `/var/log/nebula-sync.log`.
- Credentials: `/root/.pihole-sync-credentials` on delly (chmod 600). Request the password from the user if needed.
- Both Pi-holes require `app_sudo = true` inside `/etc/pihole/pihole.toml`.
- When debugging, coordinate with the user before touching production Pi-hole instances.
