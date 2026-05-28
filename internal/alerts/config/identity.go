package config

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func CanonicalResourceTypeKeys(resourceType string) []string {
	typeKey := CanonicalAlertResourceType(resourceType)
	if typeKey == "" || isUnsupportedLegacyAlertResourceType(typeKey) {
		return nil
	}

	addUnique := func(slice []string, value string) []string {
		if value == "" {
			return slice
		}
		for _, existing := range slice {
			if existing == value {
				return slice
			}
		}
		return append(slice, value)
	}

	var keys []string
	switch typeKey {
	case "guest":
		keys = addUnique(keys, "guest")
	case "vm":
		keys = addUnique(keys, "vm")
		keys = addUnique(keys, "guest")
	case "system-container":
		keys = addUnique(keys, "system-container")
		keys = addUnique(keys, "guest")
	case "oci-container":
		keys = addUnique(keys, "oci-container")
		keys = addUnique(keys, "system-container")
		keys = addUnique(keys, "guest")
	case "app-container":
		keys = addUnique(keys, "app-container")
		keys = addUnique(keys, "guest")
	case "docker-host":
		keys = addUnique(keys, "docker-host")
		keys = addUnique(keys, "node")
	case "docker-service":
		keys = addUnique(keys, "docker-service")
		keys = addUnique(keys, "app-container")
		keys = addUnique(keys, "guest")
	case "node":
		keys = addUnique(keys, "node")
	case "agent":
		keys = addUnique(keys, "agent")
		keys = addUnique(keys, "node")
	case "agent-disk":
		keys = addUnique(keys, "agent-disk")
		keys = addUnique(keys, "agent")
		keys = addUnique(keys, "storage")
	case "pbs":
		keys = addUnique(keys, "pbs")
		keys = addUnique(keys, "node")
	case "pmg":
		keys = addUnique(keys, "pmg")
		keys = addUnique(keys, "node")
	case "k8s-cluster":
		keys = addUnique(keys, "k8s-cluster")
		keys = addUnique(keys, "node")
	case "kubernetes cluster":
		keys = addUnique(keys, "k8s-cluster")
		keys = addUnique(keys, "node")
	case "k8s-node":
		keys = addUnique(keys, "k8s-node")
		keys = addUnique(keys, "node")
	case "kubernetes node":
		keys = addUnique(keys, "k8s-node")
		keys = addUnique(keys, "node")
	case "k8s-deployment":
		keys = addUnique(keys, "k8s-deployment")
		keys = addUnique(keys, "guest")
	case "kubernetes deployment":
		keys = addUnique(keys, "k8s-deployment")
		keys = addUnique(keys, "guest")
	case "k8s-namespace":
		keys = addUnique(keys, "k8s-namespace")
	case "kubernetes namespace":
		keys = addUnique(keys, "k8s-namespace")
	case "pod":
		keys = addUnique(keys, "pod")
		keys = addUnique(keys, "guest")
	case "kubernetes pod":
		keys = addUnique(keys, "pod")
		keys = addUnique(keys, "guest")
	case "truenas-system":
		keys = addUnique(keys, "truenas-system")
		keys = addUnique(keys, "agent")
		keys = addUnique(keys, "node")
	case "truenas system":
		keys = addUnique(keys, "truenas-system")
		keys = addUnique(keys, "agent")
		keys = addUnique(keys, "node")
	case "truenas-pool":
		keys = addUnique(keys, "truenas-pool")
		keys = addUnique(keys, "storage")
	case "truenas pool":
		keys = addUnique(keys, "truenas-pool")
		keys = addUnique(keys, "storage")
	case "truenas-dataset":
		keys = addUnique(keys, "truenas-dataset")
		keys = addUnique(keys, "storage")
	case "truenas dataset":
		keys = addUnique(keys, "truenas-dataset")
		keys = addUnique(keys, "storage")
	case "truenas-disk":
		keys = addUnique(keys, "truenas-disk")
		keys = addUnique(keys, "physical_disk")
		keys = addUnique(keys, "disk")
		keys = addUnique(keys, "storage")
	case "truenas disk":
		keys = addUnique(keys, "truenas-disk")
		keys = addUnique(keys, "physical_disk")
		keys = addUnique(keys, "disk")
		keys = addUnique(keys, "storage")
	case "vmware-host":
		keys = addUnique(keys, "vmware-host")
		keys = addUnique(keys, "agent")
		keys = addUnique(keys, "node")
	case "vmware-vm":
		keys = addUnique(keys, "vmware-vm")
		keys = addUnique(keys, "vm")
		keys = addUnique(keys, "guest")
	case "vmware-datastore":
		keys = addUnique(keys, "vmware-datastore")
		keys = addUnique(keys, "storage")
	case "vmware-network":
		keys = addUnique(keys, "vmware-network")
		keys = addUnique(keys, "network")
	case "storage":
		keys = addUnique(keys, "storage")
	case "disk":
		keys = addUnique(keys, "disk")
		keys = addUnique(keys, "storage")
	case "datastore":
		keys = addUnique(keys, "datastore")
		keys = addUnique(keys, "storage")
		keys = addUnique(keys, "pbs")
	case "pool", "dataset":
		keys = addUnique(keys, typeKey)
		keys = addUnique(keys, "storage")
	case "ceph":
		keys = addUnique(keys, "ceph")
		keys = addUnique(keys, "storage")
	case "physical_disk":
		keys = addUnique(keys, "physical_disk")
		keys = addUnique(keys, "disk")
		keys = addUnique(keys, "storage")
	default:
		keys = addUnique(keys, typeKey)
	}

	return keys
}

func isUnsupportedLegacyAlertResourceType(typeKey string) bool {
	if unifiedresources.IsUnsupportedLegacyResourceTypeAlias(typeKey) {
		return true
	}

	switch typeKey {
	case "host", "qemu", "container", "lxc", "docker", "docker container", "dockercontainer", "docker host", "dockerhost", "docker service", "dockerservice", "k8s", "k8s pod", "kubernetes", "kubernetes-cluster", "agent disk", "agentdisk", "pbs server", "pbsserver", "pmg server", "proxmox mail gateway":
		return true
	default:
		return false
	}
}

func CanonicalAlertResourceType(resourceType string) string {
	typeKey := strings.ToLower(strings.TrimSpace(resourceType))
	switch typeKey {
	case "kubernetes cluster":
		return "k8s-cluster"
	case "kubernetes node":
		return "k8s-node"
	case "kubernetes deployment":
		return "k8s-deployment"
	case "kubernetes namespace":
		return "k8s-namespace"
	case "kubernetes pod":
		return "pod"
	case "truenas system":
		return "truenas-system"
	case "truenas pool":
		return "truenas-pool"
	case "truenas dataset":
		return "truenas-dataset"
	case "truenas disk":
		return "truenas-disk"
	case "vmware host", "vsphere host":
		return "vmware-host"
	case "vmware vm", "vsphere vm", "vmware virtual machine", "vsphere virtual machine":
		return "vmware-vm"
	case "vmware datastore", "vsphere datastore":
		return "vmware-datastore"
	case "vmware network", "vsphere network":
		return "vmware-network"
	default:
		return typeKey
	}
}
