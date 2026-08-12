# Pulse v6.2.2 Draft Release Notes

_Draft only. The full `v6.2.2` release packet will be completed at release
cutoff._

## Fixed

- Security: fixed proxy-auth role gating when `PROXY_AUTH_ROLE_HEADER` is set
  without `PROXY_AUTH_ADMIN_ROLE`. Deployments using role-based proxy access
  should upgrade or explicitly set `PROXY_AUTH_ADMIN_ROLE`.
