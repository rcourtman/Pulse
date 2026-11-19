import { PROXMOX_ICON_PATH } from '@/components/icons/ProxmoxIcon';
import { MAP_APP_ICONS, type MapAppIcon } from './infrastructureMapAppIcons';

/**
 * What the map knows about well-known self-hosted apps: which one a guest or
 * container is, the logo to draw for it, and the group it belongs in. The
 * match is made from the container image first and the name second, on whole
 * words only, so a thing the catalogue does not know stays unlabelled and
 * goes to the last group instead of being guessed at.
 */

export type MapCategoryId =
  | 'access'
  | 'security'
  | 'media'
  | 'home'
  | 'monitoring'
  | 'data'
  | 'apps'
  | 'ai'
  | 'dev'
  | 'network'
  | 'backup'
  | 'other';

/** The order groups are drawn in. `access` and `backup` have layers of their own. */
export const MAP_CATEGORY_ORDER: readonly MapCategoryId[] = [
  'access',
  'media',
  'home',
  'apps',
  'monitoring',
  'data',
  'ai',
  'dev',
  'security',
  'network',
  'backup',
  'other',
];

export const MAP_CATEGORY_LABEL: Record<MapCategoryId, string> = {
  access: 'Ways in',
  security: 'Security',
  media: 'Media',
  home: 'Home automation',
  monitoring: 'Monitoring',
  data: 'Databases',
  apps: 'Apps',
  ai: 'AI',
  dev: 'Development',
  network: 'Network tools',
  backup: 'Backups',
  other: 'Other',
};

/** How an app in the `access` group lets traffic in or out. */
export type MapAppRole = 'tunnel' | 'VPN' | 'reverse proxy' | 'DNS' | 'remote desktop';

export interface MapApp {
  id: string;
  label: string;
  category: MapCategoryId;
  role?: MapAppRole;
  /** Key into the logo table, when the app has a logo there. */
  icon?: string;
}

// Logos the generated table does not carry: Proxmox's, which the platform
// tabs already draw, and Pulse's own ring and dot.
const EXTRA_LOGOS: Record<string, MapAppIcon> = {
  proxmox: { hex: 'E57000', d: PROXMOX_ICON_PATH },
  pulse: {
    hex: '2563EB',
    d: 'M12 3.47a8.53 8.53 0 1 0 0 17.06 8.53 8.53 0 0 0 0-17.06zm0 1.31a7.22 7.22 0 1 1 0 14.44 7.22 7.22 0 0 1 0-14.44zM12 9.56a2.44 2.44 0 1 0 0 4.88 2.44 2.44 0 0 0 0-4.88z',
  },
};

/** The logo for a key the catalogue hands out, if there is one. */
export const resolveMapAppLogo = (key: string | undefined): MapAppIcon | undefined =>
  key ? (MAP_APP_ICONS[key] ?? EXTRA_LOGOS[key]) : undefined;

type Entry = [
  id: string,
  label: string,
  aliases: string,
  options?: {
    role?: MapAppRole;
    icon?: string;
    /**
     * The name is an everyday word, so it only counts when it is the whole
     * image name, or leads a guest's name with nothing but filler after it.
     */
    plainWord?: boolean;
  },
];

// Aliases are compared with every separator removed, so `home-assistant`,
// `home_assistant` and `homeassistant` are one spelling.
const CATALOGUE: Record<MapCategoryId, Entry[]> = {
  access: [
    [
      'cloudflared',
      'Cloudflare Tunnel',
      'cloudflared cloudflaretunnel',
      { role: 'tunnel', icon: 'cloudflare' },
    ],
    ['tailscale', 'Tailscale', 'tailscale tailscalerouter', { role: 'VPN' }],
    ['headscale', 'Headscale', 'headscale', { role: 'VPN' }],
    ['wireguard', 'WireGuard', 'wireguard wgeasy', { role: 'VPN' }],
    ['openvpn', 'OpenVPN', 'openvpn', { role: 'VPN' }],
    ['zerotier', 'ZeroTier', 'zerotier', { role: 'VPN' }],
    ['netbird', 'NetBird', 'netbird', { role: 'VPN' }],
    ['traefik', 'Traefik', 'traefik', { role: 'reverse proxy', icon: 'traefikproxy' }],
    ['caddy', 'Caddy', 'caddy', { role: 'reverse proxy' }],
    [
      'nginxproxymanager',
      'Nginx Proxy Manager',
      'nginxproxymanager npmplus',
      { role: 'reverse proxy' },
    ],
    ['nginx', 'Nginx', 'nginx swag', { role: 'reverse proxy' }],
    ['haproxy', 'HAProxy', 'haproxy', { role: 'reverse proxy' }],
    ['pihole', 'Pi-hole', 'pihole', { role: 'DNS' }],
    ['adguard', 'AdGuard Home', 'adguard adguardhome', { role: 'DNS' }],
    ['technitium', 'Technitium DNS', 'technitium', { role: 'DNS' }],
    ['unbound', 'Unbound', 'unbound', { role: 'DNS' }],
    ['rustdesk', 'RustDesk', 'rustdesk', { role: 'remote desktop' }],
    ['guacamole', 'Guacamole', 'guacamole', { role: 'remote desktop', icon: 'apacheguacamole' }],
  ],
  security: [
    ['authentik', 'Authentik', 'authentik'],
    ['authelia', 'Authelia', 'authelia'],
    ['keycloak', 'Keycloak', 'keycloak'],
    ['vaultwarden', 'Vaultwarden', 'vaultwarden'],
    ['bitwarden', 'Bitwarden', 'bitwarden'],
    ['crowdsec', 'CrowdSec', 'crowdsec'],
    ['wazuh', 'Wazuh', 'wazuh'],
  ],
  media: [
    ['jellyfin', 'Jellyfin', 'jellyfin'],
    ['plex', 'Plex', 'plex plexmediaserver pms'],
    ['emby', 'Emby', 'emby embyserver'],
    ['kodi', 'Kodi', 'kodi'],
    ['sonarr', 'Sonarr', 'sonarr'],
    ['radarr', 'Radarr', 'radarr'],
    ['lidarr', 'Lidarr', 'lidarr'],
    ['readarr', 'Readarr', 'readarr'],
    ['prowlarr', 'Prowlarr', 'prowlarr'],
    ['bazarr', 'Bazarr', 'bazarr'],
    ['overseerr', 'Overseerr', 'overseerr'],
    ['jellyseerr', 'Jellyseerr', 'jellyseerr'],
    ['tautulli', 'Tautulli', 'tautulli'],
    ['recyclarr', 'Recyclarr', 'recyclarr'],
    ['flaresolverr', 'FlareSolverr', 'flaresolverr'],
    ['byparr', 'Byparr', 'byparr'],
    ['unpackerr', 'Unpackerr', 'unpackerr'],
    ['tdarr', 'Tdarr', 'tdarr'],
    ['jackett', 'Jackett', 'jackett'],
    ['qbittorrent', 'qBittorrent', 'qbittorrent qbit'],
    ['transmission', 'Transmission', 'transmission'],
    ['deluge', 'Deluge', 'deluge'],
    ['sabnzbd', 'SABnzbd', 'sabnzbd'],
    ['nzbget', 'NZBGet', 'nzbget'],
    ['jdownloader', 'JDownloader', 'jdownloader jdownloader2'],
    ['immich', 'Immich', 'immich immichserver'],
    ['photoprism', 'PhotoPrism', 'photoprism'],
    ['navidrome', 'Navidrome', 'navidrome'],
    ['audiobookshelf', 'Audiobookshelf', 'audiobookshelf'],
    ['calibre', 'Calibre', 'calibre calibreweb', { icon: 'calibreweb' }],
    ['kavita', 'Kavita', 'kavita'],
    ['komga', 'Komga', 'komga'],
    ['isponsorblocktv', 'iSponsorBlockTV', 'isponsorblocktv'],
  ],
  home: [
    ['homeassistant', 'Home Assistant', 'homeassistant hass haos hassio'],
    ['esphome', 'ESPHome', 'esphome'],
    ['zigbee2mqtt', 'Zigbee2MQTT', 'zigbee2mqtt z2m'],
    ['mosquitto', 'Mosquitto', 'mosquitto eclipsemosquitto mqtt', { icon: 'eclipsemosquitto' }],
    ['nodered', 'Node-RED', 'nodered'],
    ['frigate', 'Frigate', 'frigate'],
    ['homebridge', 'Homebridge', 'homebridge'],
    ['openhab', 'openHAB', 'openhab'],
    ['zwavejs', 'Z-Wave JS', 'zwavejs zwavejsui zwavejs2mqtt'],
    ['scrypted', 'Scrypted', 'scrypted'],
  ],
  monitoring: [
    ['grafana', 'Grafana', 'grafana'],
    ['prometheus', 'Prometheus', 'prometheus'],
    ['influxdb', 'InfluxDB', 'influxdb influx'],
    ['telegraf', 'Telegraf', 'telegraf'],
    ['loki', 'Loki', 'loki', { plainWord: true }],
    ['alertmanager', 'Alertmanager', 'alertmanager'],
    ['uptimekuma', 'Uptime Kuma', 'uptimekuma'],
    ['netdata', 'Netdata', 'netdata'],
    ['zabbix', 'Zabbix', 'zabbix zabbixserver'],
    ['checkmk', 'Checkmk', 'checkmk'],
    ['librenms', 'LibreNMS', 'librenms'],
    ['beszel', 'Beszel', 'beszel'],
    ['glances', 'Glances', 'glances', { plainWord: true }],
    ['dozzle', 'Dozzle', 'dozzle'],
    ['portainer', 'Portainer', 'portainer portainerce portaineragent'],
    ['elasticsearch', 'Elasticsearch', 'elasticsearch'],
    ['kibana', 'Kibana', 'kibana'],
    ['ntfy', 'ntfy', 'ntfy'],
    ['gotify', 'Gotify', 'gotify'],
    ['pulse', 'Pulse', 'pulse', { plainWord: true }],
  ],
  data: [
    ['postgres', 'PostgreSQL', 'postgres postgresql pgvector timescaledb', { icon: 'postgresql' }],
    ['mariadb', 'MariaDB', 'mariadb'],
    ['mysql', 'MySQL', 'mysql'],
    ['redis', 'Redis', 'redis valkey keydb'],
    ['mongodb', 'MongoDB', 'mongodb mongo'],
    ['clickhouse', 'ClickHouse', 'clickhouse clickhouseserver'],
    ['minio', 'MinIO', 'minio'],
    ['rabbitmq', 'RabbitMQ', 'rabbitmq'],
    ['couchdb', 'CouchDB', 'couchdb'],
    ['memcached', 'Memcached', 'memcached'],
  ],
  apps: [
    ['nextcloud', 'Nextcloud', 'nextcloud nextcloudaio'],
    ['paperless', 'Paperless-ngx', 'paperless paperlessngx', { icon: 'paperlessngx' }],
    ['homepage', 'Homepage', 'homepage gethomepage', { plainWord: true }],
    ['homarr', 'Homarr', 'homarr'],
    ['heimdall', 'Heimdall', 'heimdall'],
    ['dashy', 'Dashy', 'dashy'],
    ['mealie', 'Mealie', 'mealie'],
    ['vikunja', 'Vikunja', 'vikunja'],
    ['outline', 'Outline', 'outline', { plainWord: true }],
    ['bookstack', 'BookStack', 'bookstack'],
    ['wikijs', 'Wiki.js', 'wikijs', { icon: 'wikidotjs' }],
    ['ghost', 'Ghost', 'ghost', { plainWord: true }],
    ['wordpress', 'WordPress', 'wordpress'],
    ['freshrss', 'FreshRSS', 'freshrss'],
    ['miniflux', 'Miniflux', 'miniflux'],
    ['linkding', 'linkding', 'linkding'],
    ['actualbudget', 'Actual Budget', 'actualbudget actualserver'],
    ['firefly', 'Firefly III', 'fireflyiii firefly', { icon: 'fireflyiii' }],
    ['syncthing', 'Syncthing', 'syncthing'],
    ['seafile', 'Seafile', 'seafile'],
    ['n8n', 'n8n', 'n8n'],
    ['onlyoffice', 'ONLYOFFICE', 'onlyoffice'],
    ['joplin', 'Joplin', 'joplin joplinserver'],
    ['trilium', 'Trilium', 'trilium triliumnotes'],
    ['searxng', 'SearXNG', 'searxng searx'],
    ['stirlingpdf', 'Stirling-PDF', 'stirlingpdf'],
    ['grocy', 'Grocy', 'grocy'],
    ['tandoor', 'Tandoor', 'tandoor'],
    ['changedetection', 'changedetection.io', 'changedetection'],
  ],
  ai: [
    ['ollama', 'Ollama', 'ollama'],
    ['openwebui', 'Open WebUI', 'openwebui'],
    ['localai', 'LocalAI', 'localai'],
    ['comfyui', 'ComfyUI', 'comfyui'],
    ['litellm', 'LiteLLM', 'litellm'],
  ],
  dev: [
    ['gitea', 'Gitea', 'gitea'],
    ['forgejo', 'Forgejo', 'forgejo'],
    ['gitlab', 'GitLab', 'gitlab gitlabce gitlabee'],
    ['jenkins', 'Jenkins', 'jenkins'],
    ['woodpecker', 'Woodpecker', 'woodpecker woodpeckerserver'],
    ['codeserver', 'code-server', 'codeserver'],
    ['harbor', 'Harbor', 'harbor', { plainWord: true }],
    ['sonarqube', 'SonarQube', 'sonarqube'],
  ],
  network: [
    ['unifi', 'UniFi', 'unifi unificontroller unifinetworkapplication', { icon: 'ubiquiti' }],
    ['omada', 'Omada', 'omada omadacontroller'],
    ['opnsense', 'OPNsense', 'opnsense'],
    ['pfsense', 'pfSense', 'pfsense'],
    ['mikrotik', 'MikroTik', 'mikrotik routeros'],
    ['openwrt', 'OpenWrt', 'openwrt'],
    ['gluetun', 'Gluetun', 'gluetun'],
    ['speedtest', 'Speedtest Tracker', 'speedtest speedtesttracker'],
    ['netbootxyz', 'netboot.xyz', 'netbootxyz'],
    ['iventoy', 'iVentoy', 'iventoy'],
    ['smokeping', 'SmokePing', 'smokeping'],
  ],
  backup: [
    ['pbs', 'Proxmox Backup Server', 'pbs proxmoxbackupserver proxmoxbackup', { icon: 'proxmox' }],
    ['duplicati', 'Duplicati', 'duplicati'],
    ['borg', 'Borg', 'borg borgbackup borgmatic', { icon: 'borgbackup' }],
    ['restic', 'Restic', 'restic'],
    ['kopia', 'Kopia', 'kopia'],
    ['urbackup', 'UrBackup', 'urbackup'],
    ['duplicacy', 'Duplicacy', 'duplicacy'],
  ],
  other: [],
};

// Tags an owner already puts on a guest that say what it is for. They place a
// thing the catalogue does not know, and never override an app it does know.
const TAG_CATEGORY: Record<string, MapCategoryId> = {
  media: 'media',
  arr: 'media',
  streaming: 'media',
  homeautomation: 'home',
  smarthome: 'home',
  iot: 'home',
  monitoring: 'monitoring',
  observability: 'monitoring',
  metrics: 'monitoring',
  logging: 'monitoring',
  database: 'data',
  databases: 'data',
  db: 'data',
  ai: 'ai',
  llm: 'ai',
  dev: 'dev',
  development: 'dev',
  ci: 'dev',
  cicd: 'dev',
  security: 'security',
  auth: 'security',
  sso: 'security',
  network: 'network',
  networking: 'network',
  backup: 'backup',
  backups: 'backup',
};

const strip = (value: string): string => value.toLowerCase().replace(/[^a-z0-9]/g, '');
const words = (value: string): string[] =>
  value
    .toLowerCase()
    .split(/[^a-z0-9]+/)
    .filter(Boolean);

// Words that say where or how a thing runs, not what it is.
const FILLER = new Set(
  'docker lxc vm ct server app web ui prod production main primary new old test dev staging'.split(
    ' ',
  ),
);
const PLAIN_WORD_ALIASES = new Set<string>();

const APPS_BY_ALIAS: ReadonlyMap<string, MapApp> = (() => {
  const byAlias = new Map<string, MapApp>();
  for (const category of MAP_CATEGORY_ORDER) {
    for (const [id, label, aliases, options] of CATALOGUE[category]) {
      const icon = options?.icon ?? id;
      const app: MapApp = {
        id,
        label,
        category,
        ...(options?.role ? { role: options.role } : {}),
        ...(resolveMapAppLogo(icon) ? { icon } : {}),
      };
      for (const alias of aliases.split(' ')) {
        byAlias.set(alias, app);
        if (options?.plainWord) PLAIN_WORD_ALIASES.add(alias);
      }
    }
  }
  return byAlias;
})();

/** Every app the catalogue names, for tests and for the count on the page. */
export const MAP_CATALOGUE_APPS: readonly MapApp[] = [...new Set(APPS_BY_ALIAS.values())];

// The spellings a name can match under: the whole thing, each word, and each
// run of two or three neighbouring words (`nginx proxy manager`).
const spellings = (value: string): string[] => {
  const parts = words(value);
  const out = [strip(value), ...parts];
  for (let size = 2; size <= 3; size += 1) {
    for (let start = 0; start + size <= parts.length; start += 1) {
      out.push(parts.slice(start, start + size).join(''));
    }
  }
  return out;
};

// The longest alias wins, so `nginx-proxy-manager` is not read as Nginx. On a
// tie the spelling that comes first in the name wins.
const bestMatch = (value: string): MapApp | undefined => {
  const parts = words(value);
  const leadsWithFillerAfter = (alias: string): boolean =>
    parts[0] === alias && parts.slice(1).every((part) => FILLER.has(part) || /^\d+$/.test(part));
  let best: { app: MapApp; length: number } | undefined;
  for (const spelling of spellings(value)) {
    const app = APPS_BY_ALIAS.get(spelling);
    if (!app || (best && spelling.length <= best.length)) continue;
    if (PLAIN_WORD_ALIASES.has(spelling) && !leadsWithFillerAfter(spelling)) continue;
    best = { app, length: spelling.length };
  }
  return best?.app;
};

// `lscr.io/linuxserver/sonarr:latest` names its app in the last path part;
// `vaultwarden/server` names it in the part before.
const imageNames = (image: string): string[] => {
  const path = image.split('@')[0].replace(/:[^/:]*$/, '');
  const parts = path.split('/').filter(Boolean);
  return parts.slice(-2).reverse();
};

export interface MapAppSubject {
  name: string;
  image?: string;
  tags?: readonly string[];
}

/** The catalogue app a guest or container is, if it is one the map knows. */
export const resolveMapApp = (subject: MapAppSubject): MapApp | undefined => {
  for (const name of subject.image ? imageNames(subject.image) : []) {
    const app = bestMatch(name);
    if (app) return app;
  }
  return bestMatch(subject.name);
};

/** The group a thing is drawn in: its app's, else the one its tags name. */
export const resolveMapCategory = (
  app: MapApp | undefined,
  tags: readonly string[] | undefined,
): MapCategoryId => {
  if (app) return app.category;
  for (const tag of tags ?? []) {
    const category = TAG_CATEGORY[strip(tag)];
    if (category) return category;
  }
  return 'other';
};

/** The network gear an availability check can be recognised as. */
export type MapDeviceRole = 'router' | 'switch' | 'access-point';

export const MAP_DEVICE_ROLE_LABEL: Record<MapDeviceRole, string> = {
  router: 'Router',
  switch: 'Switch',
  'access-point': 'Access point',
};

// Words an owner gives the thing, and the systems a router runs. `gateway`
// is left out: an API gateway and a mail gateway are checked too, and a
// check on a real default gateway is known by its address.
const ROUTER_WORDS = new Set([
  'router',
  'firewall',
  'modem',
  'opnsense',
  'pfsense',
  'mikrotik',
  'routeros',
  'edgerouter',
  'openwrt',
  'fritzbox',
  'udm',
  'usg',
]);
const SWITCH_WORDS = new Set(['switch', 'switches']);
const ACCESS_POINT_WORDS = new Set(['ap', 'aps', 'wifi', 'wlan', 'accesspoint']);
// A console is not network gear, whatever it is called.
const NOT_GEAR_WORDS = new Set(['nintendo', 'kvm']);

/**
 * The network gear a check is on, from its name, the way an app is known
 * from its container's. It decides the tile's glyph and box and nothing
 * else, and a name that says none of these is left a plain check.
 */
export const resolveMapDeviceRole = (name: string): MapDeviceRole | undefined => {
  const words = name
    .toLowerCase()
    .replace(/wi-fi/g, 'wifi')
    .replace(/access[\s_-]*points?/g, 'accesspoint')
    .split(/[^a-z0-9]+/)
    .filter(Boolean);
  if (words.some((word) => NOT_GEAR_WORDS.has(word))) return undefined;
  if (words.some((word) => ROUTER_WORDS.has(word))) return 'router';
  if (words.some((word) => SWITCH_WORDS.has(word))) return 'switch';
  if (words.some((word) => ACCESS_POINT_WORDS.has(word))) return 'access-point';
  return undefined;
};
