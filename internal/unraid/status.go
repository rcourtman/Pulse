package unraid

import "strings"

// NormalizeNativeIdentity removes transport-only sentinel values emitted by
// Unraid for slots that have never had a device assigned. These values are not
// disk identities and must not turn DISK_NP placeholders into missing members.
func NormalizeNativeIdentity(value string) string {
	value = strings.TrimSpace(value)
	if IsPlaceholderIdentity(value) {
		return ""
	}
	return value
}

// IsPlaceholderIdentity recognizes the empty identity forms exposed by mdcmd
// and the model-shaped value produced when an older agent parsed that identity.
func IsPlaceholderIdentity(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "_", "ata-_", "scsi-_", "sas-_", "usb-_", "nvme-_",
		"ata -", "scsi -", "sas -", "usb -", "nvme -":
		return true
	default:
		return false
	}
}

// HasMeaningfulIdentity reports whether either native identity field names a
// device rather than an unassigned-slot sentinel.
func HasMeaningfulIdentity(model, serial string) bool {
	return NormalizeNativeIdentity(model) != "" || NormalizeNativeIdentity(serial) != ""
}

// HasMeaningfulFilesystem reports whether a native filesystem field is
// evidence that a slot has a disk assigned. Unraid emits "auto" for configured
// but empty DISK_NP slots, so presence of that value alone cannot establish
// membership.
func HasMeaningfulFilesystem(filesystem string) bool {
	filesystem = strings.TrimSpace(filesystem)
	return filesystem != "" && !strings.EqualFold(filesystem, "auto")
}

// IsExplicitMissingMember reports Unraid's provider-owned status for a slot
// that was assigned but whose device is no longer present. Plain DISK_NP means
// no device is assigned and must not be treated as equivalent.
func IsExplicitMissingMember(rawStatus string) bool {
	return strings.EqualFold(strings.TrimSpace(rawStatus), "DISK_NP_MISSING")
}
