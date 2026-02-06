// Package servicediscovery provides infrastructure discovery capabilities.
package servicediscovery

import (
	"fmt"
	"strings"
)

// webServiceDefault defines the default web interface configuration for a service.
type webServiceDefault struct {
	Port     int    // Default port
	Protocol string // http or https
	Path     string // Path suffix (e.g., "/web" for Plex)
}

// webServiceDefaults maps service types to their default web interface configurations.
// These are used to construct suggested URLs when a service is discovered.
var webServiceDefaults = map[string]webServiceDefault{
	// Media
	"frigate":        {5000, "http", ""},
	"jellyfin":       {8096, "http", ""},
	"plex":           {32400, "http", "/web"},
	"emby":           {8096, "http", ""},
	"kodi":           {8080, "http", ""},
	"sonarr":         {8989, "http", ""},
	"radarr":         {7878, "http", ""},
	"lidarr":         {8686, "http", ""},
	"prowlarr":       {9696, "http", ""},
	"bazarr":         {6767, "http", ""},
	"overseerr":      {5055, "http", ""},
	"tautulli":       {8181, "http", ""},
	"ombi":           {3579, "http", ""},
	"navidrome":      {4533, "http", ""},
	"audiobookshelf": {13378, "http", ""},

	// Home Automation
	"home-assistant": {8123, "http", ""},
	"homeassistant":  {8123, "http", ""},
	"openhab":        {8080, "http", ""},
	"domoticz":       {8080, "http", ""},
	"node-red":       {1880, "http", ""},
	"nodered":        {1880, "http", ""},

	// Monitoring
	"grafana":     {3000, "http", ""},
	"prometheus":  {9090, "http", ""},
	"influxdb":    {8086, "http", ""},
	"uptime-kuma": {3001, "http", ""},
	"uptimekuma":  {3001, "http", ""},
	"netdata":     {19999, "http", ""},
	"zabbix":      {80, "http", ""},

	// Web Servers / Reverse Proxies
	"nginx":               {80, "http", ""},
	"apache":              {80, "http", ""},
	"caddy":               {80, "http", ""},
	"traefik":             {8080, "http", "/dashboard/"},
	"haproxy":             {8404, "http", "/stats"},
	"nginx-proxy-manager": {81, "http", ""},

	// Backup
	"proxmox-backup-server": {8007, "https", ""},
	"pbs":                   {8007, "https", ""},
	"duplicati":             {8200, "http", ""},
	"urbackup":              {55414, "http", ""},
	"veeam":                 {9443, "https", ""},

	// Virtualization
	"proxmox":        {8006, "https", ""},
	"proxmox-ve":     {8006, "https", ""},
	"portainer":      {9000, "http", ""},
	"cockpit":        {9090, "https", ""},
	"unraid":         {80, "http", ""},
	"truenas":        {80, "http", ""},
	"openmediavault": {80, "http", ""},

	// NVR / Security
	"blue-iris":  {81, "http", ""},
	"shinobi":    {8080, "http", ""},
	"zoneminder": {80, "http", "/zm"},
	"scrypted":   {10443, "https", ""},
	"motioneye":  {8765, "http", ""},

	// Storage / File Sharing
	"nextcloud":   {80, "http", ""},
	"owncloud":    {80, "http", ""},
	"syncthing":   {8384, "http", ""},
	"filebrowser": {80, "http", ""},
	"seafile":     {80, "http", ""},

	// Download / Torrents
	"qbittorrent":  {8080, "http", ""},
	"transmission": {9091, "http", ""},
	"deluge":       {8112, "http", ""},
	"rtorrent":     {8080, "http", ""},
	"sabnzbd":      {8080, "http", ""},
	"nzbget":       {6789, "http", ""},
	"jackett":      {9117, "http", ""},

	// Network / DNS
	"pihole":           {80, "http", "/admin"},
	"adguard-home":     {3000, "http", ""},
	"adguardhome":      {3000, "http", ""},
	"unifi":            {8443, "https", ""},
	"unifi-controller": {8443, "https", ""},
	"opnsense":         {443, "https", ""},
	"pfsense":          {443, "https", ""},

	// Database Admin
	"phpmyadmin": {80, "http", ""},
	"pgadmin":    {80, "http", ""},
	"adminer":    {8080, "http", ""},

	// Other
	"gitea":         {3000, "http", ""},
	"gitlab":        {80, "http", ""},
	"jenkins":       {8080, "http", ""},
	"vaultwarden":   {80, "http", ""},
	"bitwarden":     {80, "http", ""},
	"paperless":     {8000, "http", ""},
	"paperless-ngx": {8000, "http", ""},
	"mealie":        {9000, "http", ""},
	"photoprism":    {2342, "http", ""},
	"immich":        {2283, "http", ""},
	"homepage":      {3000, "http", ""},
	"dashy":         {80, "http", ""},
	"homarr":        {7575, "http", ""},
	"organizr":      {80, "http", ""},
}

// webEnabledCategories defines which service categories typically have web interfaces.
var webEnabledCategories = map[ServiceCategory]bool{
	CategoryWebServer:   true,
	CategoryMedia:       true,
	CategoryHomeAuto:    true,
	CategoryMonitoring:  true,
	CategoryNVR:         true,
	CategoryBackup:      true,
	CategoryVirtualizer: true,
	CategoryStorage:     true,
	CategoryNetwork:     true,
}

// commonWebPorts defines ports commonly used for web interfaces.
// Used as fallback when service type is not in the defaults table.
var commonWebPorts = []int{80, 443, 8080, 8443, 3000, 5000, 8000, 8888, 9000}

const (
	urlReasonServiceDefaultMatch = "service_default_match"
	urlReasonServiceVariation    = "service_default_variation_match"
	urlReasonWebPortInference    = "web_port_inference"
	urlReasonNoDiscovery         = "no_discovery"
	urlReasonNoHost              = "no_host"
	urlReasonCategoryNotWeb      = "category_not_web_enabled"
	urlReasonNoPortsDetected     = "no_ports_detected"
	urlReasonNoCommonWebPort     = "no_common_web_ports"
)

// SuggestWebURL generates a suggested web interface URL for a discovered resource.
// It uses service defaults when available, falling back to discovered ports.
// Returns empty string if no suitable URL can be constructed.
//
// Note: For Docker containers with non-standard port mappings, the fallback logic
// uses internal container ports. However, this only affects unknown services not
// in the defaults table - known services use their standard ports.
func SuggestWebURL(discovery *ResourceDiscovery, hostIP string) string {
	url, _, _ := suggestWebURLWithReason(discovery, hostIP)
	return url
}

func suggestWebURLWithReason(discovery *ResourceDiscovery, hostIP string) (string, string, string) {
	if discovery == nil || hostIP == "" {
		switch {
		case discovery == nil:
			return "", urlReasonNoDiscovery, "missing discovery payload"
		default:
			return "", urlReasonNoHost, "no host or IP candidate available"
		}
	}

	// Skip if category doesn't typically have web UI
	if !webEnabledCategories[discovery.Category] && discovery.Category != CategoryUnknown {
		return "", urlReasonCategoryNotWeb, fmt.Sprintf("service category %q is not typically web-facing", discovery.Category)
	}

	// Normalize service type for lookup
	serviceType := strings.ToLower(discovery.ServiceType)
	serviceType = strings.ReplaceAll(serviceType, "_", "-")
	serviceType = strings.ReplaceAll(serviceType, " ", "-")

	// Try service defaults first
	if defaults, ok := webServiceDefaults[serviceType]; ok {
		return buildURL(defaults.Protocol, hostIP, defaults.Port, defaults.Path), urlReasonServiceDefaultMatch, fmt.Sprintf("service default: %s", serviceType)
	}

	// Try variations of service type
	variations := []string{
		serviceType,
		strings.ReplaceAll(serviceType, "-", ""),
		strings.TrimSuffix(serviceType, "-server"),
		strings.TrimSuffix(serviceType, "server"),
	}
	for _, variation := range variations {
		if defaults, ok := webServiceDefaults[variation]; ok {
			return buildURL(defaults.Protocol, hostIP, defaults.Port, defaults.Path), urlReasonServiceVariation, fmt.Sprintf("normalized match: %s", variation)
		}
	}

	// Fallback: use discovered ports
	if len(discovery.Ports) > 0 {
		// First, check if any discovered port is a known web port
		for _, port := range discovery.Ports {
			if port.Protocol == "tcp" && isCommonWebPort(port.Port) {
				protocol := "http"
				if port.Port == 443 || port.Port == 8443 {
					protocol = "https"
				}
				return buildURL(protocol, hostIP, port.Port, ""), urlReasonWebPortInference, fmt.Sprintf("detected web port %d/%s", port.Port, port.Protocol)
			}
		}
		return "", urlReasonNoCommonWebPort, "no common web UI ports detected"
	}

	// No suitable URL could be determined
	return "", urlReasonNoPortsDetected, "no detected ports for web UI inference"
}

// buildURL constructs a URL from components.
func buildURL(protocol, host string, port int, path string) string {
	// Default ports don't need to be included
	if (protocol == "http" && port == 80) || (protocol == "https" && port == 443) {
		return fmt.Sprintf("%s://%s%s", protocol, host, path)
	}
	return fmt.Sprintf("%s://%s:%d%s", protocol, host, port, path)
}

// isCommonWebPort checks if a port is commonly used for web interfaces.
func isCommonWebPort(port int) bool {
	for _, p := range commonWebPorts {
		if port == p {
			return true
		}
	}
	// Also check common ranges
	return (port >= 80 && port <= 90) ||
		(port >= 443 && port <= 450) ||
		(port >= 8000 && port <= 8100) ||
		(port >= 3000 && port <= 3100) ||
		(port >= 5000 && port <= 5100) ||
		(port >= 9000 && port <= 9100)
}
