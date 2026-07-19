package recovery

import "testing"

// TestProxmoxPBSGuestLooseContinuityKey exercises every branch of
// ProxmoxPBSGuestLooseContinuityKey in keys.go: the itemType guard (default
// arm), the empty entityIDLabel guard, the empty/whitespace subjectLabel guard
// (via normalizeContinuityLabel), the numeric-only subjectLabel guard, and the
// success path for both "vm" and "system-container" (including normalization of
// raw item-type aliases through NormalizeRecoveryItemType).
func TestProxmoxPBSGuestLooseContinuityKey(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		subjectLabel  string
		itemType      string
		entityIDLabel string
		want          string
	}{
		// itemType switch: default arm returns "" for any item type that does
		// not normalize to "vm" or "system-container".
		{
			name:          "itemType app-container rejected",
			subjectLabel:  "web-server",
			itemType:      "app-container",
			entityIDLabel: "100",
			want:          "",
		},
		{
			name:          "itemType pvc rejected",
			subjectLabel:  "web-server",
			itemType:      "pvc",
			entityIDLabel: "100",
			want:          "",
		},
		{
			name:          "itemType empty rejected",
			subjectLabel:  "web-server",
			itemType:      "",
			entityIDLabel: "100",
			want:          "",
		},
		{
			name:          "itemType all normalizes to empty and is rejected",
			subjectLabel:  "web-server",
			itemType:      "all",
			entityIDLabel: "100",
			want:          "",
		},
		// entityIDLabel empty guard (both literal empty and whitespace-only,
		// since the implementation strings.TrimSpace's the value before checking).
		{
			name:          "vm empty entityIDLabel returns empty",
			subjectLabel:  "web-server",
			itemType:      "vm",
			entityIDLabel: "",
			want:          "",
		},
		{
			name:          "vm whitespace entityIDLabel returns empty",
			subjectLabel:  "web-server",
			itemType:      "vm",
			entityIDLabel: "   ",
			want:          "",
		},
		// subjectLabel that normalizes to empty (normalizeContinuityLabel
		// returns "" for empty/whitespace-only input).
		{
			name:          "vm empty subjectLabel returns empty",
			subjectLabel:  "",
			itemType:      "vm",
			entityIDLabel: "100",
			want:          "",
		},
		{
			name:          "vm whitespace subjectLabel returns empty",
			subjectLabel:  "   \t\n",
			itemType:      "vm",
			entityIDLabel: "100",
			want:          "",
		},
		// numeric-only subjectLabel guard (isNumericOnlyLabel matches digits only).
		{
			name:          "vm numeric-only subjectLabel returns empty",
			subjectLabel:  "100",
			itemType:      "vm",
			entityIDLabel: "100",
			want:          "",
		},
		{
			name:          "system-container numeric-only subjectLabel returns empty",
			subjectLabel:  "00140",
			itemType:      "system-container",
			entityIDLabel: "140",
			want:          "",
		},
		// Success path: vm.
		{
			name:          "vm success builds loose key without namespace",
			subjectLabel:  "web-server",
			itemType:      "vm",
			entityIDLabel: "100",
			want:          "proxmox-pbs-guest-loose:vm:100:web-server",
		},
		// Success path: system-container.
		{
			name:          "system-container success builds loose key without namespace",
			subjectLabel:  "pulse-v4-prod",
			itemType:      "system-container",
			entityIDLabel: "140",
			want:          "proxmox-pbs-guest-loose:system-container:140:pulse-v4-prod",
		},
		// NormalizeRecoveryItemType maps alias item types onto "vm"/"system-container"
		// and the loose key uses the normalized value.
		{
			name:          "proxmox-vm alias normalizes to vm in loose key",
			subjectLabel:  "web-server",
			itemType:      "proxmox-vm",
			entityIDLabel: "100",
			want:          "proxmox-pbs-guest-loose:vm:100:web-server",
		},
		{
			name:          "lxc alias normalizes to system-container in loose key",
			subjectLabel:  "pulse-v4-prod",
			itemType:      "lxc",
			entityIDLabel: "140",
			want:          "proxmox-pbs-guest-loose:system-container:140:pulse-v4-prod",
		},
		// normalizeContinuityLabel lowercases input and collapses internal
		// whitespace; the loose key embeds the normalized label.
		{
			name:          "subjectLabel normalized (case folded, whitespace collapsed)",
			subjectLabel:  "  Web   Server  ",
			itemType:      "vm",
			entityIDLabel: "100",
			want:          "proxmox-pbs-guest-loose:vm:100:web server",
		},
		// entityIDLabel is only TrimSpace'd (not lowercased); the loose key
		// preserves its case and surrounding whitespace is stripped.
		{
			name:          "entityIDLabel trimmed but case preserved",
			subjectLabel:  "web-server",
			itemType:      "vm",
			entityIDLabel: "  VM-100  ",
			want:          "proxmox-pbs-guest-loose:vm:VM-100:web-server",
		},
		// Sanity check that the loose key is structurally distinct from the
		// conservative key (no namespace segment between itemType and entityID).
		{
			name:          "loose key omits namespace segment present in conservative key",
			subjectLabel:  "pulse-v4-prod",
			itemType:      "system-container",
			entityIDLabel: "140",
			want:          "proxmox-pbs-guest-loose:system-container:140:pulse-v4-prod",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ProxmoxPBSGuestLooseContinuityKey(tc.subjectLabel, tc.itemType, tc.entityIDLabel)
			if got != tc.want {
				t.Fatalf("ProxmoxPBSGuestLooseContinuityKey(%q, %q, %q) = %q, want %q",
					tc.subjectLabel, tc.itemType, tc.entityIDLabel, got, tc.want)
			}
		})
	}
}

// TestProxmoxPBSGuestLooseContinuityKey_OmitsNamespace guarantees directly that
// for the same (subjectLabel, itemType, entityIDLabel) tuple, the loose key
// never embeds a namespace segment and differs from a hypothetical key that did
// include one. This pins the doc-comment promise at keys.go:228-229.
func TestProxmoxPBSGuestLooseContinuityKey_OmitsNamespace(t *testing.T) {
	t.Parallel()

	got := ProxmoxPBSGuestLooseContinuityKey("pulse-v4-prod", "system-container", "140")
	want := "proxmox-pbs-guest-loose:system-container:140:pulse-v4-prod"
	if got != want {
		t.Fatalf("loose key = %q, want %q", got, want)
	}

	// The conservative key for the same identity includes a namespace segment;
	// the loose key must be a strict prefix-without-namespace and must not equal
	// any key that contains a namespace value.
	conservative := ProxmoxPBSGuestContinuityKey("pulse-v4-prod", "system-container", "pimox", "140")
	if conservative == "" {
		t.Fatalf("conservative key unexpectedly empty; test setup invalid")
	}
	if got == conservative {
		t.Fatalf("loose key (%q) must not equal conservative key with namespace (%q)", got, conservative)
	}
}
