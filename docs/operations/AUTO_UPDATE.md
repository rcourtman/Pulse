# Automatic updates on installed servers

Use the [Pulse Server Automatic Updates guide](../AUTO_UPDATE.md) for the
current server update, timer, manual-update and recovery procedures. Server
updates and Pulse Agent updates are separate jobs.

Saving **Automatic Stable Updates** does not provision or start a missing
systemd update timer. Timer scheduling belongs to the installed deployment;
check its existing service and timer before changing automation. Source-built
servers do not use unattended release updates, and Docker/Kubernetes servers
are updated through their existing deployment manager.

Do not rewrite `system.json` through a shared temporary file to change update
preferences: that can expose configuration and replace its ownership or
permissions. Use the current guide and preserve the existing deployment's
configuration, credentials and data.

A failed update does not prove that the old version is still running or that
automatic rollback restored all state. Check the running version and service
health before retrying or restoring, keep the original failure evidence and
recovery snapshots, and follow the guide's qualified rollback procedure.
Do not start an update service just to test a reported failure.
