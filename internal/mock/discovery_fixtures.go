package mock

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

const (
	maxMockVMDiscoveryFixtures        = 48
	maxMockContainerDiscoveryFixtures = 64
	maxMockDockerDiscoveryFixtures    = 96
	maxMockHostDiscoveryFixtures      = 6
	maxMockK8sDiscoveryFixtures       = 80

	discoveryResourceTypeVM              = "vm"
	discoveryResourceTypeSystemContainer = "system-container"
	discoveryResourceTypeDocker          = "docker"
	discoveryResourceTypeK8s             = "k8s"
	discoveryResourceTypeAgent           = "agent"

	discoveryFactCategoryVersion  = "version"
	discoveryFactCategoryService  = "service"
	discoveryFactCategoryHardware = "hardware"

	discoveryCategoryDatabase    = "database"
	discoveryCategoryWebServer   = "web_server"
	discoveryCategoryCache       = "cache"
	discoveryCategoryMonitoring  = "monitoring"
	discoveryCategoryBackup      = "backup"
	discoveryCategoryStorage     = "storage"
	discoveryCategoryVirtualizer = "virtualizer"
	discoveryCategoryNetwork     = "network"
	discoveryCategorySecurity    = "security"
	discoveryCategoryMedia       = "media"
	discoveryCategoryHomeAuto    = "home_automation"

	mockFingerprintSchemaVersion = 3
	mockCLIAccessVersion         = 2
)

type DiscoveryFixture struct {
	ID                       string
	ResourceType             string
	ResourceID               string
	TargetID                 string
	AgentID                  string
	Hostname                 string
	ServiceType              string
	ServiceName              string
	ServiceVersion           string
	Category                 string
	CLIAccess                string
	Facts                    []DiscoveryFact
	ConfigPaths              []string
	DataPaths                []string
	LogPaths                 []string
	Ports                    []DiscoveryPort
	DockerMounts             []DiscoveryDockerBindMount
	UserNotes                string
	UserSecrets              map[string]string
	Confidence               float64
	AIReasoning              string
	DiscoveredAt             time.Time
	UpdatedAt                time.Time
	ScanDuration             int64
	Fingerprint              string
	FingerprintedAt          time.Time
	FingerprintSchemaVersion int
	CLIAccessVersion         int
	RawCommandOutput         map[string]string
	SuggestedURL             string
	SuggestedURLSourceCode   string
	SuggestedURLSourceDetail string
	SuggestedURLDiagnostic   string
}

type DiscoveryFact struct {
	Category     string
	Key          string
	Value        string
	Source       string
	Confidence   float64
	DiscoveredAt time.Time
}

type DiscoveryPort struct {
	Port     int
	Protocol string
	Process  string
	Address  string
}

type DiscoveryDockerBindMount struct {
	ContainerName string
	Source        string
	Destination   string
	Type          string
	ReadOnly      bool
}

type mockDiscoveryProfile struct {
	ServiceType     string
	ServiceName     string
	ServiceVersion  string
	Category        string
	ConfigPaths     []string
	DataPaths       []string
	LogPaths        []string
	Ports           []DiscoveryPort
	WebScheme       string
	WebPort         int
	WebPath         string
	SourceCode      string
	SourceDetail    string
	Diagnostic      string
	AdditionalFacts []DiscoveryFact
}

func normalizeDiscoveryResourceType(resourceType string) string {
	return strings.TrimSpace(resourceType)
}

func makeDiscoveryResourceID(resourceType, targetID, resourceID string) string {
	return fmt.Sprintf("%s:%s:%s", resourceType, targetID, resourceID)
}

func getMockCLIAccessTemplate(resourceType string) string {
	switch normalizeDiscoveryResourceType(resourceType) {
	case discoveryResourceTypeSystemContainer:
		return "Use pulse_control with target_host matching this container's hostname. Commands run directly inside the container."
	case discoveryResourceTypeVM:
		return "Use pulse_control with target_host matching this VM's hostname. Commands run directly inside the VM."
	case discoveryResourceTypeDocker:
		return "Use pulse_control targeting the Docker host with command: docker exec {container} <your-command>"
	case discoveryResourceTypeK8s:
		return "Use kubectl exec -n {namespace} {pod} -- <your-command>"
	case discoveryResourceTypeAgent:
		return "Use pulse_control with target_host matching this host. Commands run directly."
	default:
		return "Use pulse_control with target_host matching the resource hostname."
	}
}

// CurrentDiscoveryFixtures returns defensive copies of the mock-mode discovery
// records exposed through the normal Discovery API.
func CurrentDiscoveryFixtures() []*DiscoveryFixture {
	if !IsMockEnabled() {
		return nil
	}

	dataMu.RLock()
	defer dataMu.RUnlock()

	return cloneDiscoveryFixtures(mockGraph.DiscoveryFixtures)
}

func CurrentDiscoveryFixtureByResource(resourceType string, targetID, resourceID string) *DiscoveryFixture {
	for _, discovery := range CurrentDiscoveryFixtures() {
		if discoveryFixtureMatchesResource(discovery, resourceType, targetID, resourceID) {
			return cloneDiscoveryFixture(discovery)
		}
	}
	return nil
}

func CurrentDiscoveryFixturesByType(resourceType string) []*DiscoveryFixture {
	matches := make([]*DiscoveryFixture, 0)
	for _, discovery := range CurrentDiscoveryFixtures() {
		if discovery == nil {
			continue
		}
		if normalizeDiscoveryResourceType(discovery.ResourceType) == normalizeDiscoveryResourceType(resourceType) {
			matches = append(matches, cloneDiscoveryFixture(discovery))
		}
	}
	return matches
}

func CurrentDiscoveryFixturesByTarget(targetID string) []*DiscoveryFixture {
	matches := make([]*DiscoveryFixture, 0)
	for _, discovery := range CurrentDiscoveryFixtures() {
		if discoveryFixtureMatchesTarget(discovery, targetID) {
			matches = append(matches, cloneDiscoveryFixture(discovery))
		}
	}
	return matches
}

func discoveryFixtureMatchesResource(discovery *DiscoveryFixture, resourceType string, targetID, resourceID string) bool {
	if discovery == nil {
		return false
	}
	if normalizeDiscoveryResourceType(discovery.ResourceType) != normalizeDiscoveryResourceType(resourceType) {
		return false
	}
	if !discoveryFixtureMatchesTarget(discovery, targetID) {
		return false
	}

	resourceID = strings.TrimSpace(resourceID)
	return resourceID != "" && strings.EqualFold(strings.TrimSpace(discovery.ResourceID), resourceID)
}

func discoveryFixtureMatchesTarget(discovery *DiscoveryFixture, targetID string) bool {
	if discovery == nil {
		return false
	}

	targetID = strings.TrimSpace(targetID)
	if targetID == "" {
		return false
	}

	candidates := []string{
		discovery.TargetID,
		discovery.AgentID,
		discovery.Hostname,
	}
	if discovery.ResourceType == discoveryResourceTypeAgent {
		candidates = append(candidates, discovery.ResourceID)
	}
	for _, candidate := range candidates {
		if strings.EqualFold(strings.TrimSpace(candidate), targetID) {
			return true
		}
	}
	return false
}

func buildDiscoveryFixtures(state models.StateSnapshot, now time.Time) []*DiscoveryFixture {
	if now.IsZero() {
		now = time.Now()
	}
	now = now.UTC()

	discoveries := make([]*DiscoveryFixture, 0, 24)
	discoveries = append(discoveries, buildHostDiscoveryFixtures(state, now)...)
	discoveries = append(discoveries, buildVMDiscoveryFixtures(state.VMs, now)...)
	discoveries = append(discoveries, buildContainerDiscoveryFixtures(state.Containers, now)...)
	discoveries = append(discoveries, buildDockerDiscoveryFixtures(state.DockerHosts, now)...)
	discoveries = append(discoveries, buildK8sDiscoveryFixtures(state.KubernetesClusters, now)...)

	sort.Slice(discoveries, func(i, j int) bool {
		return discoveries[i].ID < discoveries[j].ID
	})
	return discoveries
}

func buildHostDiscoveryFixtures(state models.StateSnapshot, now time.Time) []*DiscoveryFixture {
	discoveries := make([]*DiscoveryFixture, 0, maxMockHostDiscoveryFixtures)

	hosts := append([]models.Host(nil), state.Hosts...)
	sort.Slice(hosts, func(i, j int) bool {
		return firstNonEmpty(hosts[i].DisplayName, hosts[i].Hostname, hosts[i].ID) < firstNonEmpty(hosts[j].DisplayName, hosts[j].Hostname, hosts[j].ID)
	})
	for _, host := range hosts {
		if len(discoveries) >= maxMockHostDiscoveryFixtures/2 {
			break
		}
		targetID := firstNonEmpty(host.ID, host.Hostname)
		if targetID == "" {
			continue
		}
		profile := mockHostDiscoveryProfile(host.OSName, host.OSVersion, host.AgentVersion)
		discoveries = append(discoveries, newMockDiscovery(discoveryResourceTypeAgent, targetID, targetID, host.Hostname, profile, now, len(discoveries)))
	}

	dockerHosts := append([]models.DockerHost(nil), state.DockerHosts...)
	sort.Slice(dockerHosts, func(i, j int) bool {
		return firstNonEmpty(dockerHosts[i].DisplayName, dockerHosts[i].Hostname, dockerHosts[i].ID) < firstNonEmpty(dockerHosts[j].DisplayName, dockerHosts[j].Hostname, dockerHosts[j].ID)
	})
	for _, host := range dockerHosts {
		if len(discoveries) >= maxMockHostDiscoveryFixtures {
			break
		}
		targetID := firstNonEmpty(host.AgentID, host.Hostname, host.ID)
		if targetID == "" {
			continue
		}
		profile := mockDockerHostDiscoveryProfile(host.Runtime, firstNonEmpty(host.RuntimeVersion, host.DockerVersion), host.AgentVersion)
		discoveries = append(discoveries, newMockDiscovery(discoveryResourceTypeAgent, targetID, targetID, host.Hostname, profile, now, len(discoveries)))
	}

	return discoveries
}

func buildVMDiscoveryFixtures(vms []models.VM, now time.Time) []*DiscoveryFixture {
	vms = append([]models.VM(nil), vms...)
	sort.Slice(vms, func(i, j int) bool {
		return vms[i].VMID < vms[j].VMID
	})

	discoveries := make([]*DiscoveryFixture, 0, maxMockVMDiscoveryFixtures)
	for _, vm := range vms {
		if len(discoveries) >= maxMockVMDiscoveryFixtures {
			break
		}
		if !strings.EqualFold(strings.TrimSpace(vm.Status), "running") || vm.Node == "" || vm.VMID <= 0 {
			continue
		}
		profile := mockServiceDiscoveryProfile(vm.Name, "", "vm")
		discovery := newMockDiscovery(discoveryResourceTypeVM, vm.Node, strconv.Itoa(vm.VMID), vm.Name, profile, now, len(discoveries))
		discovery.SuggestedURL = mockSuggestedURL(profile, firstIPOrHost(vm.IPAddresses, vm.Name))
		discoveries = append(discoveries, discovery)
	}
	return discoveries
}

func buildContainerDiscoveryFixtures(containers []models.Container, now time.Time) []*DiscoveryFixture {
	containers = append([]models.Container(nil), containers...)
	sort.Slice(containers, func(i, j int) bool {
		return containers[i].VMID < containers[j].VMID
	})

	discoveries := make([]*DiscoveryFixture, 0, maxMockContainerDiscoveryFixtures)
	for _, ct := range containers {
		if len(discoveries) >= maxMockContainerDiscoveryFixtures {
			break
		}
		if !strings.EqualFold(strings.TrimSpace(ct.Status), "running") || ct.Node == "" || ct.VMID <= 0 {
			continue
		}
		profile := mockServiceDiscoveryProfile(ct.Name, "", "system-container")
		discovery := newMockDiscovery(discoveryResourceTypeSystemContainer, ct.Node, strconv.Itoa(ct.VMID), ct.Name, profile, now, len(discoveries))
		discovery.SuggestedURL = mockSuggestedURL(profile, firstIPOrHost(ct.IPAddresses, ct.Name))
		discoveries = append(discoveries, discovery)
	}
	return discoveries
}

func buildDockerDiscoveryFixtures(hosts []models.DockerHost, now time.Time) []*DiscoveryFixture {
	hosts = append([]models.DockerHost(nil), hosts...)
	sort.Slice(hosts, func(i, j int) bool {
		return hosts[i].Hostname < hosts[j].Hostname
	})

	discoveries := make([]*DiscoveryFixture, 0, maxMockDockerDiscoveryFixtures)
	for _, host := range hosts {
		targetID := firstNonEmpty(host.AgentID, host.Hostname, host.ID)
		if targetID == "" || !strings.EqualFold(strings.TrimSpace(host.Status), "online") {
			continue
		}

		containers := append([]models.DockerContainer(nil), host.Containers...)
		sort.Slice(containers, func(i, j int) bool {
			return containers[i].Name < containers[j].Name
		})
		for _, container := range containers {
			if len(discoveries) >= maxMockDockerDiscoveryFixtures {
				return discoveries
			}
			if !strings.EqualFold(strings.TrimSpace(container.State), "running") || container.Name == "" {
				continue
			}
			profile := mockServiceDiscoveryProfile(container.Name, container.Image, "docker")
			discovery := newMockDiscovery(discoveryResourceTypeDocker, targetID, container.Name, host.Hostname, profile, now, len(discoveries))
			discovery.AgentID = targetID
			discovery.SuggestedURL = mockSuggestedURL(profile, host.Hostname)
			discovery.Facts = append(discovery.Facts, mockDiscoveryFact(discoveryFactCategoryService, "container_image", container.Image, "docker inspect", 0.95, discovery.DiscoveredAt))
			discovery.DockerMounts = mockDockerMounts(host, container, profile)
			discoveries = append(discoveries, discovery)
		}
	}
	return discoveries
}

func buildK8sDiscoveryFixtures(clusters []models.KubernetesCluster, now time.Time) []*DiscoveryFixture {
	clusters = append([]models.KubernetesCluster(nil), clusters...)
	sort.Slice(clusters, func(i, j int) bool {
		return clusters[i].Name < clusters[j].Name
	})

	discoveries := make([]*DiscoveryFixture, 0, maxMockK8sDiscoveryFixtures)
	for _, cluster := range clusters {
		targetID := firstNonEmpty(cluster.AgentID, cluster.ID, cluster.Name)
		if targetID == "" {
			continue
		}
		pods := append([]models.KubernetesPod(nil), cluster.Pods...)
		sort.Slice(pods, func(i, j int) bool {
			return pods[i].Name < pods[j].Name
		})
		for _, pod := range pods {
			if len(discoveries) >= maxMockK8sDiscoveryFixtures {
				return discoveries
			}
			if !strings.EqualFold(strings.TrimSpace(pod.Phase), "running") {
				continue
			}
			resourceID := firstNonEmpty(pod.UID, namespacedName(pod.Namespace, pod.Name), pod.Name)
			if resourceID == "" {
				continue
			}
			image := ""
			if len(pod.Containers) > 0 {
				image = pod.Containers[0].Image
			}
			profile := mockServiceDiscoveryProfile(firstNonEmpty(pod.OwnerName, pod.Name), image, "k8s")
			discovery := newMockDiscovery(discoveryResourceTypeK8s, targetID, resourceID, firstNonEmpty(cluster.Name, pod.NodeName, pod.Name), profile, now, len(discoveries))
			discovery.AgentID = targetID
			discovery.SuggestedURL = mockSuggestedURL(profile, firstNonEmpty(cluster.Name, pod.Name))
			discovery.Facts = append(discovery.Facts,
				mockDiscoveryFact(discoveryFactCategoryService, "namespace", pod.Namespace, "kubectl get pod -o json", 0.98, discovery.DiscoveredAt),
				mockDiscoveryFact(discoveryFactCategoryService, "pod", pod.Name, "kubectl get pod -o json", 0.98, discovery.DiscoveredAt),
			)
			discoveries = append(discoveries, discovery)
		}
	}
	return discoveries
}

func newMockDiscovery(resourceType string, targetID, resourceID, hostname string, profile mockDiscoveryProfile, now time.Time, ordinal int) *DiscoveryFixture {
	discoveredAt := now.Add(-time.Duration(ordinal%9+1) * time.Hour)
	updatedAt := now.Add(-time.Duration(ordinal%17+3) * time.Minute)
	id := makeDiscoveryResourceID(resourceType, targetID, resourceID)
	if profile.ServiceVersion == "" {
		profile.ServiceVersion = "detected"
	}

	discovery := &DiscoveryFixture{
		ID:                       id,
		ResourceType:             resourceType,
		ResourceID:               resourceID,
		TargetID:                 targetID,
		AgentID:                  targetID,
		Hostname:                 firstNonEmpty(hostname, targetID),
		ServiceType:              profile.ServiceType,
		ServiceName:              profile.ServiceName,
		ServiceVersion:           profile.ServiceVersion,
		Category:                 profile.Category,
		CLIAccess:                getMockCLIAccessTemplate(resourceType),
		Facts:                    mockBaseFacts(profile, discoveredAt),
		ConfigPaths:              append([]string(nil), profile.ConfigPaths...),
		DataPaths:                append([]string(nil), profile.DataPaths...),
		LogPaths:                 append([]string(nil), profile.LogPaths...),
		Ports:                    append([]DiscoveryPort(nil), profile.Ports...),
		UserSecrets:              map[string]string{},
		Confidence:               0.86 + float64(ordinal%8)/100,
		AIReasoning:              "Discovery identified the service from runtime metadata, exposed ports, and well-known configuration paths in the demo fixture.",
		DiscoveredAt:             discoveredAt,
		UpdatedAt:                updatedAt,
		ScanDuration:             int64(1200 + ordinal*140),
		Fingerprint:              mockStableHexString(16, id, "discovery-fingerprint"),
		FingerprintedAt:          discoveredAt.Add(-2 * time.Minute),
		FingerprintSchemaVersion: mockFingerprintSchemaVersion,
		CLIAccessVersion:         mockCLIAccessVersion,
		SuggestedURLSourceCode:   profile.SourceCode,
		SuggestedURLSourceDetail: profile.SourceDetail,
		SuggestedURLDiagnostic:   profile.Diagnostic,
	}
	for _, fact := range profile.AdditionalFacts {
		if fact.DiscoveredAt.IsZero() {
			fact.DiscoveredAt = discoveredAt
		}
		discovery.Facts = append(discovery.Facts, fact)
	}
	return discovery
}

func mockBaseFacts(profile mockDiscoveryProfile, discoveredAt time.Time) []DiscoveryFact {
	facts := []DiscoveryFact{
		mockDiscoveryFact(discoveryFactCategoryService, "service", profile.ServiceName, "service inventory", 0.94, discoveredAt),
		mockDiscoveryFact(discoveryFactCategoryVersion, "version", profile.ServiceVersion, "version probe", 0.88, discoveredAt),
	}
	if len(profile.ConfigPaths) > 0 {
		facts = append(facts, mockDiscoveryFact(discoveryFactCategoryService, "primary_config", profile.ConfigPaths[0], "filesystem probe", 0.89, discoveredAt))
	}
	return facts
}

func mockDiscoveryFact(category string, key, value, source string, confidence float64, discoveredAt time.Time) DiscoveryFact {
	return DiscoveryFact{
		Category:     category,
		Key:          key,
		Value:        value,
		Source:       source,
		Confidence:   confidence,
		DiscoveredAt: discoveredAt,
	}
}

func mockServiceDiscoveryProfile(name, image, runtime string) mockDiscoveryProfile {
	token := strings.ToLower(strings.TrimSpace(name + " " + image))
	version := imageTag(image)
	if version == "" || version == "latest" {
		version = "detected"
	}

	switch {
	case strings.Contains(token, "postgres"):
		return mockProfile("postgres", "PostgreSQL", firstNonEmpty(version, "16.4"), discoveryCategoryDatabase, 5432, "postgres", "", "", "No web interface was suggested because PostgreSQL exposes a database port, not an HTTP UI.", []string{"/var/lib/postgresql/data/postgresql.conf", "/etc/postgresql/postgresql.conf"}, []string{"/var/lib/postgresql/data"}, []string{"/var/log/postgresql/postgresql.log"})
	case strings.Contains(token, "redis"):
		return mockProfile("redis", "Redis", firstNonEmpty(version, "7.2"), discoveryCategoryCache, 6379, "redis-server", "", "", "No web interface was suggested because Redis exposes a cache port, not an HTTP UI.", []string{"/usr/local/etc/redis/redis.conf", "/etc/redis/redis.conf"}, []string{"/data"}, []string{"/var/log/redis/redis-server.log"})
	case strings.Contains(token, "traefik") || strings.Contains(token, "edge-proxy"):
		return mockProfile("traefik", "Traefik", firstNonEmpty(version, "3.1"), discoveryCategoryWebServer, 443, "traefik", "https", "/", "", []string{"/etc/traefik/traefik.yml", "/etc/traefik/dynamic"}, []string{"/letsencrypt"}, []string{"/var/log/traefik/traefik.log"})
	case strings.Contains(token, "smtp") || strings.Contains(token, "postfix") || strings.Contains(token, "mail-relay"):
		return mockProfile("postfix", "Postfix SMTP Relay", firstNonEmpty(version, "3.8"), discoveryCategoryNetwork, 25, "master", "", "", "No web interface was suggested because the detected service exposes SMTP, not an HTTP UI.", []string{"/etc/postfix/main.cf", "/etc/postfix/master.cf"}, []string{"/var/spool/postfix"}, []string{"/var/log/mail.log"})
	case strings.Contains(token, "auth-service"):
		return mockProfile("auth-service", "Auth Service", firstNonEmpty(version, "2026.04"), discoveryCategorySecurity, 8080, "auth-service", "http", "/", "", []string{"/etc/auth-service/config.yaml"}, []string{"/var/lib/auth-service"}, []string{"/var/log/auth-service.log"})
	case strings.Contains(token, "billing-worker") || strings.Contains(token, "payments-worker"):
		return mockProfile("queue-worker", "Queue Worker", firstNonEmpty(version, "2026.04"), discoveryCategoryBackup, 0, "queue-worker", "", "", "No web interface was suggested because this workload is a background worker.", []string{"/etc/pulse-demo/worker.yaml"}, []string{"/var/lib/pulse-demo/worker"}, []string{"/var/log/pulse-demo/worker.log"})
	case strings.Contains(token, "reporting-api") || strings.Contains(token, "inventory-api") || strings.Contains(token, "checkout-api"):
		return mockProfile("api-service", "API Service", firstNonEmpty(version, "2026.04"), discoveryCategoryWebServer, 8080, "api-service", "http", "/", "", []string{"/etc/pulse-demo/api.yaml"}, []string{"/var/lib/pulse-demo/api"}, []string{"/var/log/pulse-demo/api.log"})
	case strings.Contains(token, "docs-wiki") || strings.Contains(token, "docs-portal") || strings.Contains(token, "dev-portal") || strings.Contains(token, "customer-portal"):
		return mockProfile("web-portal", "Web Portal", firstNonEmpty(version, "2026.04"), discoveryCategoryWebServer, 8080, "web-portal", "http", "/", "", []string{"/etc/pulse-demo/portal.yaml"}, []string{"/var/lib/pulse-demo/portal"}, []string{"/var/log/pulse-demo/portal.log"})
	case strings.Contains(token, "vaultwarden"):
		return mockProfile("vaultwarden", "Vaultwarden", firstNonEmpty(version, "1.32.7"), discoveryCategorySecurity, 80, "vaultwarden", "http", "/", "", []string{"/data/config.json"}, []string{"/data"}, []string{"/data/vaultwarden.log"})
	case strings.Contains(token, "uptime-kuma"):
		return mockProfile("uptime-kuma", "Uptime Kuma", firstNonEmpty(version, "1.23.16"), discoveryCategoryMonitoring, 3001, "node", "http", "/", "", []string{"/app/data/kuma.db"}, []string{"/app/data"}, []string{"/app/data/error.log"})
	case strings.Contains(token, "prometheus"):
		return mockProfile("prometheus", "Prometheus", firstNonEmpty(version, "2.54.1"), discoveryCategoryMonitoring, 9090, "prometheus", "http", "/", "", []string{"/etc/prometheus/prometheus.yml"}, []string{"/prometheus"}, []string{"/var/log/prometheus/prometheus.log"})
	case strings.Contains(token, "grafana"):
		return mockProfile("grafana-agent", "Grafana Agent", firstNonEmpty(version, "0.42.0"), discoveryCategoryMonitoring, 12345, "grafana-agent", "http", "/", "", []string{"/etc/grafana-agent/config.river"}, []string{"/var/lib/grafana-agent"}, []string{"/var/log/grafana-agent.log"})
	case strings.Contains(token, "backup"):
		return mockProfile("backup-coordinator", "Backup Coordinator", firstNonEmpty(version, "2026.04"), discoveryCategoryBackup, 8080, "backup-coordinator", "http", "/health", "", []string{"/etc/pulse-demo/backup-coordinator.yaml"}, []string{"/var/lib/pulse-demo/backups"}, []string{"/var/log/pulse-demo/backup-coordinator.log"})
	case strings.Contains(token, "sftp"):
		return mockProfile("sftp", "SFTP Ingest", version, discoveryCategoryStorage, 22, "sshd", "", "", "No web interface was suggested because the detected service exposes SSH/SFTP only.", []string{"/etc/ssh/sshd_config", "/etc/sftp/users.conf"}, []string{"/home"}, []string{"/var/log/auth.log"})
	case strings.Contains(token, "minio"):
		return mockProfile("minio", "MinIO", firstNonEmpty(version, "RELEASE.2026-04"), discoveryCategoryStorage, 9000, "minio", "http", "/", "", []string{"/etc/minio/config.env"}, []string{"/data"}, []string{"/var/log/minio/minio.log"})
	case strings.Contains(token, "homeassistant"):
		return mockProfile("home-assistant", "Home Assistant", firstNonEmpty(version, "2026.4"), discoveryCategoryHomeAuto, 8123, "python3", "http", "/", "", []string{"/config/configuration.yaml"}, []string{"/config"}, []string{"/config/home-assistant.log"})
	case strings.Contains(token, "pihole"):
		return mockProfile("pihole", "Pi-hole", firstNonEmpty(version, "2026.03"), discoveryCategoryNetwork, 80, "lighttpd", "http", "/admin", "", []string{"/etc/pihole/setupVars.conf", "/etc/dnsmasq.d"}, []string{"/etc/pihole"}, []string{"/var/log/pihole/pihole.log"})
	case strings.Contains(token, "jellyfin"):
		return mockProfile("jellyfin", "Jellyfin", firstNonEmpty(version, "10.10"), discoveryCategoryMedia, 8096, "jellyfin", "http", "/", "", []string{"/config/system.xml"}, []string{"/config", "/media"}, []string{"/config/log"})
	case strings.Contains(token, "nextcloud"):
		return mockProfile("nextcloud", "Nextcloud", firstNonEmpty(version, "30.0"), discoveryCategoryStorage, 80, "apache2", "http", "/", "", []string{"/var/www/html/config/config.php"}, []string{"/var/www/html/data"}, []string{"/var/www/html/data/nextcloud.log"})
	default:
		serviceType := strings.Trim(strings.ToLower(strings.ReplaceAll(name, "_", "-")), "-")
		if serviceType == "" {
			serviceType = strings.TrimSpace(runtime)
		}
		if serviceType == "" {
			serviceType = "application"
		}
		return mockProfile(serviceType, humanizeServiceName(serviceType), firstNonEmpty(version, "2026.04"), discoveryCategoryWebServer, 8080, serviceType, "http", "/", "", []string{fmt.Sprintf("/etc/%s/config.yaml", serviceType)}, []string{fmt.Sprintf("/var/lib/%s", serviceType)}, []string{fmt.Sprintf("/var/log/%s.log", serviceType)})
	}
}

func mockProfile(serviceType, serviceName, version string, category string, port int, process, scheme, path, diagnostic string, configPaths, dataPaths, logPaths []string) mockDiscoveryProfile {
	sourceCode := ""
	sourceDetail := ""
	ports := []DiscoveryPort{}
	if scheme != "" && port > 0 {
		sourceCode = "listening-port"
		sourceDetail = fmt.Sprintf("Detected %s listener on %d/tcp", serviceName, port)
	}
	if port > 0 {
		ports = append(ports, DiscoveryPort{Port: port, Protocol: "tcp", Process: process, Address: "0.0.0.0"})
	}
	return mockDiscoveryProfile{
		ServiceType:     serviceType,
		ServiceName:     serviceName,
		ServiceVersion:  version,
		Category:        category,
		ConfigPaths:     configPaths,
		DataPaths:       dataPaths,
		LogPaths:        logPaths,
		Ports:           ports,
		WebScheme:       scheme,
		WebPort:         port,
		WebPath:         path,
		SourceCode:      sourceCode,
		SourceDetail:    sourceDetail,
		Diagnostic:      diagnostic,
		AdditionalFacts: []DiscoveryFact{},
	}
}

func mockHostDiscoveryProfile(osName, osVersion, agentVersion string) mockDiscoveryProfile {
	serviceName := "Pulse Unified Agent"
	version := firstNonEmpty(agentVersion, osVersion, "detected")
	profile := mockProfile("pulse-agent", serviceName, version, discoveryCategoryMonitoring, 0, "pulse-agent", "", "", "No web interface was suggested for this host-level agent discovery.", []string{"/etc/pulse-agent/config.yaml"}, []string{"/var/lib/pulse-agent"}, []string{"/var/log/pulse-agent.log"})
	if osName != "" {
		profile.AdditionalFacts = append(profile.AdditionalFacts, DiscoveryFact{
			Category:   discoveryFactCategoryHardware,
			Key:        "os",
			Value:      strings.TrimSpace(osName + " " + osVersion),
			Source:     "agent host facts",
			Confidence: 0.98,
		})
	}
	return profile
}

func mockDockerHostDiscoveryProfile(runtime, runtimeVersion, agentVersion string) mockDiscoveryProfile {
	serviceName := "Docker Host"
	if strings.EqualFold(strings.TrimSpace(runtime), "podman") {
		serviceName = "Podman Host"
	}
	profile := mockProfile(strings.ToLower(strings.ReplaceAll(serviceName, " ", "-")), serviceName, firstNonEmpty(runtimeVersion, agentVersion, "detected"), discoveryCategoryVirtualizer, 0, runtime, "", "", "No workload web interface was suggested for the host runtime itself.", []string{"/etc/docker/daemon.json", "/etc/containers/containers.conf"}, []string{"/var/lib/docker", "/var/lib/containers"}, []string{"/var/log/docker.log", "/var/log/podman.log"})
	profile.AdditionalFacts = append(profile.AdditionalFacts, DiscoveryFact{
		Category:   discoveryFactCategoryService,
		Key:        "runtime",
		Value:      firstNonEmpty(runtime, "container runtime"),
		Source:     "agent runtime facts",
		Confidence: 0.98,
	})
	return profile
}

func mockDockerMounts(host models.DockerHost, container models.DockerContainer, profile mockDiscoveryProfile) []DiscoveryDockerBindMount {
	base := fmt.Sprintf("/srv/%s/%s", sanitizePathToken(firstNonEmpty(host.Hostname, host.ID, "docker-host")), sanitizePathToken(container.Name))
	mounts := make([]DiscoveryDockerBindMount, 0, 2)
	if len(profile.ConfigPaths) > 0 {
		mounts = append(mounts, DiscoveryDockerBindMount{
			ContainerName: container.Name,
			Source:        base + "/config",
			Destination:   containerPathRoot(profile.ConfigPaths[0]),
			Type:          "bind",
		})
	}
	if len(profile.DataPaths) > 0 {
		mounts = append(mounts, DiscoveryDockerBindMount{
			ContainerName: container.Name,
			Source:        base + "/data",
			Destination:   profile.DataPaths[0],
			Type:          "bind",
		})
	}
	return mounts
}

func mockSuggestedURL(profile mockDiscoveryProfile, host string) string {
	if profile.WebScheme == "" || profile.WebPort <= 0 || host == "" {
		return ""
	}
	path := profile.WebPath
	if path == "" {
		path = "/"
	}
	return fmt.Sprintf("%s://%s:%d%s", profile.WebScheme, strings.Trim(host, "[]"), profile.WebPort, path)
}

func imageTag(image string) string {
	image = strings.TrimSpace(image)
	if image == "" {
		return ""
	}
	if idx := strings.LastIndex(image, ":"); idx >= 0 && idx < len(image)-1 && !strings.Contains(image[idx+1:], "/") {
		return strings.TrimSpace(image[idx+1:])
	}
	return ""
}

func firstIPOrHost(ips []string, fallback string) string {
	for _, ip := range ips {
		ip = strings.TrimSpace(ip)
		if ip == "" {
			continue
		}
		if before, _, found := strings.Cut(ip, "/"); found {
			ip = before
		}
		return ip
	}
	return strings.TrimSpace(fallback)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func namespacedName(namespace, name string) string {
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	if namespace == "" {
		return name
	}
	if name == "" {
		return namespace
	}
	return namespace + "/" + name
}

func humanizeServiceName(serviceType string) string {
	parts := strings.FieldsFunc(serviceType, func(r rune) bool {
		return r == '-' || r == '_' || r == '.'
	})
	for i := range parts {
		if parts[i] == "" {
			continue
		}
		parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
	}
	return strings.Join(parts, " ")
}

func sanitizePathToken(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer(" ", "-", "/", "-", ":", "-", "_", "-")
	value = replacer.Replace(value)
	value = strings.Trim(value, "-")
	if value == "" {
		return "service"
	}
	return value
}

func containerPathRoot(path string) string {
	path = strings.TrimSpace(path)
	if path == "" || path == "/" {
		return "/config"
	}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		return "/config"
	}
	return "/" + parts[0]
}

func cloneDiscoveryFixtures(in []*DiscoveryFixture) []*DiscoveryFixture {
	if in == nil {
		return nil
	}
	out := make([]*DiscoveryFixture, 0, len(in))
	for _, discovery := range in {
		out = append(out, cloneDiscoveryFixture(discovery))
	}
	return out
}

func cloneDiscoveryFixture(in *DiscoveryFixture) *DiscoveryFixture {
	if in == nil {
		return nil
	}
	out := *in
	out.Facts = append([]DiscoveryFact(nil), in.Facts...)
	out.ConfigPaths = append([]string(nil), in.ConfigPaths...)
	out.DataPaths = append([]string(nil), in.DataPaths...)
	out.LogPaths = append([]string(nil), in.LogPaths...)
	out.Ports = append([]DiscoveryPort(nil), in.Ports...)
	out.DockerMounts = append([]DiscoveryDockerBindMount(nil), in.DockerMounts...)
	if in.UserSecrets != nil {
		out.UserSecrets = make(map[string]string, len(in.UserSecrets))
		for key, value := range in.UserSecrets {
			out.UserSecrets[key] = value
		}
	}
	if in.RawCommandOutput != nil {
		out.RawCommandOutput = make(map[string]string, len(in.RawCommandOutput))
		for key, value := range in.RawCommandOutput {
			out.RawCommandOutput[key] = value
		}
	}
	return &out
}
