# Pulse v6.2.2 Draft Release Notes

_Draft only. The full `v6.2.2` release packet will be completed at release
cutoff._

## Fixed

- Security: fixed proxy-auth role gating when `PROXY_AUTH_ROLE_HEADER` was set
  without `PROXY_AUTH_ADMIN_ROLE`; that combination treated every
  proxy-authenticated user as an administrator. Before upgrading, set
  `PROXY_AUTH_ADMIN_ROLE` to the IdP's administrator group name, matching case
  exactly. Role matching is now enforced, so the `admin` default will not match
  names such as `Admins` or `authentik Admins`. Pulse logs a startup warning
  when it falls back to the default.
