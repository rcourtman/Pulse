# Primeros pasos con Pulse en español

Esta página es la entrada en español para instalación, primer inicio de sesión y
elección de plan. La documentación completa y canónica sigue estando en la
documentación en inglés en [docs/README.md](../../README.md). Los comandos,
nombres de imágenes, claves de configuración, claves de activación y rutas de
la interfaz se conservan sin traducir de forma intencional.

## ¿Qué es Pulse?

Pulse es un workspace de monitoreo autohospedado para Proxmox, Docker,
Kubernetes, TrueNAS e infraestructura relacionada. Community incluye el
monitoreo principal gratis. Relay añade acceso remoto seguro a la interfaz web
de Pulse, emparejamiento con Pulse Mobile para handoff, notificaciones push e
historial de 14 días. Pro añade análisis de causa raíz, flujos de remediación
seguros, herramientas operativas, funciones de gobernanza e historial de 90
días.

## Clientes de pago Relay, Pro y legacy

Los assets de GitHub Releases y la imagen pública de Docker `rcourtman/pulse`
son builds Community. Activa tu licencia en
**Settings → Plans → Existing purchases** para desbloquear funciones Pro.

Estos builds Community no incluyen los hooks privados de runtime de Pulse Pro
para Audit Log, Audit Webhooks, RBAC y governed remediation. Para esa runtime,
usa <https://pulserelay.pro/download.html> con una **v6 activation key**. Una
v6 activation key empieza con `ppk_live_`. Una clave v5 o legacy no es una
activation key `ppk_live_` y no funcionará en esa página de descarga.

## Inicio rápido: Proxmox LXC

Si usas Proxmox VE, el instalador LXC oficial es el camino más nativo de Pulse.
Instala el servidor Pulse. Sustituye `vX.Y.Z` por la etiqueta exacta del
release que quieres instalar, verifica el instalador firmado y ejecútalo en tu
host Proxmox:

```bash
export PULSE_VERSION=vX.Y.Z
curl -fsSLO "https://github.com/rcourtman/Pulse/releases/download/${PULSE_VERSION}/install.sh"
curl -fsSLO "https://github.com/rcourtman/Pulse/releases/download/${PULSE_VERSION}/install.sh.sshsig"
ssh-keygen -Y verify \
  -f <(printf '%s\n' 'pulse-installer namespaces="pulse-install" ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIMZd/DaH+BldzOkq1A8KVTcFk73nAyrE8aJOyf7i00jm pulse-installer') \
  -I pulse-installer \
  -n pulse-install \
  -s install.sh.sshsig < install.sh
bash install.sh --version "${PULSE_VERSION}"
rm -f install.sh install.sh.sshsig
```

Las instalaciones de agentes y las actualizaciones de agentes de v5 a v6 usan
el comando que Pulse genera en **Settings → Infrastructure → Install on a
host**. Ese comando se sirve desde tu servidor Pulse en `/install.sh`.

## Inicio rápido: Docker

Para entornos en contenedores o pruebas, puedes iniciar Pulse directamente con
Docker:

```bash
docker run -d \
  --name pulse \
  -p 7655:7655 \
  -v pulse_data:/data \
  --restart unless-stopped \
  rcourtman/pulse:vX.Y.Z
```

Después abre Pulse en `http://<your-ip>:7655`.

Con Docker Compose, el mismo archivo puede servir para imágenes Community y
Pro privadas si usa la variable `PULSE_IMAGE`:

```yaml
services:
  pulse:
    image: ${PULSE_IMAGE:-rcourtman/pulse:vX.Y.Z}
    container_name: pulse
    restart: unless-stopped
    ports:
      - "7655:7655"
    volumes:
      - pulse_data:/data
    environment:
      - PULSE_AUTH_USER=admin
      - PULSE_AUTH_PASS=secret123

volumes:
  pulse_data:
```

## Primer inicio de sesión

En el primer arranque debes recuperar un **Bootstrap Token** y usarlo para
crear la cuenta administradora.

| Plataforma | Comando |
|---|---|
| Docker | `docker exec pulse /app/pulse bootstrap-token` |
| Kubernetes | `kubectl exec -it <pod> -- /app/pulse bootstrap-token` |
| Systemd | `sudo pulse bootstrap-token` |

Pega la cadena de token que imprime el comando. No pegues directamente el
contenido del archivo `.bootstrap_token`. En v6 ese archivo puede contener un
snapshot JSON cifrado en vez del token utilizable para la configuración.

1. Abre `http://<your-ip>:7655`.
2. Pega el **Bootstrap Token**.
3. Completa el asistente **Quick Security Setup**.
4. Abre **Settings → Infrastructure → Install on a host** si quieres instalar
   el unified agent en hosts.

Para Proxmox, empieza con monitoreo API-only si el inventario, estado de nodo,
estado de VMs/contenedores y métricas de storage son suficientes. Usa agentes
para visibilidad de Docker/Podman dentro de invitados, datos SMART/temperatura
del host, detalle local de ZFS/Ceph/mdadm u otra telemetría que requiera acceso
local al host.

## Siguientes pasos

- [Installation Guide](../../INSTALL.md) para rutas completas de instalación.
- [Configuration](../../CONFIGURATION.md) para autenticación, notificaciones y
  ajustes del sistema.
- [Troubleshooting](../../TROUBLESHOOTING.md) para logs y problemas comunes.
- [Agent Security](../../AGENT_SECURITY.md) para privilegios de agentes,
  opciones Proxmox API-only y verificación de firmas.
- [Plans and entitlements](../../PULSE_PRO.md) para Community, Relay, Pro y
  Cloud.
