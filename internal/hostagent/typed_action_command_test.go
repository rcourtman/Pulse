package hostagent

import "testing"

func TestTypedActionCatalogRejectsArgumentInjection(t *testing.T) {
	tests := []struct {
		catalog typedActionCatalog
		name    string
		args    []string
		valid   bool
	}{
		{typedActionCatalogPackage, "apt-get", []string{"-s", "-o", "Debug::NoLocking=1", "upgrade"}, true},
		{typedActionCatalogPackage, "apt-get", []string{"update"}, true},
		{typedActionCatalogPackage, "apt-get", []string{"clean"}, true},
		{typedActionCatalogPackage, "apt-get", []string{"-y", "--no-remove", "-o", "Dpkg::Options::=--force-confold", "upgrade"}, true},
		{typedActionCatalogPackage, "dpkg", []string{"--audit"}, true},
		{typedActionCatalogPackage, "apt-get", []string{"update", ";", "sh"}, false},
		{typedActionCatalogPackage, "apt-get", []string{"autoclean"}, false},
		{typedActionCatalogPackage, "apt-get", []string{"clean", "--simulate"}, false},
		{typedActionCatalogPackage, "/tmp/apt-get", []string{"update"}, false},
		{typedActionCatalogProxmox, "qm", []string{"status", "100"}, true},
		{typedActionCatalogProxmox, "pct", []string{"stop", "2147483647"}, true},
		{typedActionCatalogProxmox, "qm", []string{"shutdown", "100"}, true},
		{typedActionCatalogProxmox, "qm", []string{"reboot", "100"}, true},
		{typedActionCatalogProxmox, "pct", []string{"reboot", "2147483647"}, false},
		{typedActionCatalogProxmoxHandoff, "qm", []string{"start", "100"}, true},
		{typedActionCatalogProxmoxHandoff, "pct", []string{"reboot", "2147483647"}, true},
		{typedActionCatalogProxmoxHandoff, "qm", []string{"reboot", "100"}, false},
		{typedActionCatalogProxmoxHandoff, "qm", []string{"status", "100"}, false},
		{typedActionCatalogProxmox, "qm", []string{"start", "01"}, false},
		{typedActionCatalogProxmox, "qm", []string{"start", "100", "--skiplock"}, false},
		{typedActionCatalogProxmox, "sh", []string{"-c", "id"}, false},
		{typedActionCatalogProbe, "true", nil, true},
		{typedActionCatalogProbe, "true", []string{"--help"}, false},
	}
	for _, test := range tests {
		err := validateTypedActionInvocation(test.catalog, test.name, test.args)
		if (err == nil) != test.valid {
			t.Fatalf("validate %s %s %v: err=%v valid=%v", test.catalog, test.name, test.args, err, test.valid)
		}
	}
}

func TestTypedActionCatalogForProxmoxInvocationSeparatesProviderHandoff(t *testing.T) {
	for _, test := range []struct {
		tool string
		verb string
		want typedActionCatalog
	}{
		{tool: "qm", verb: "status", want: typedActionCatalogProxmox},
		{tool: "pct", verb: "stop", want: typedActionCatalogProxmox},
		{tool: "qm", verb: "shutdown", want: typedActionCatalogProxmox},
		{tool: "qm", verb: "start", want: typedActionCatalogProxmoxHandoff},
		{tool: "pct", verb: "start", want: typedActionCatalogProxmoxHandoff},
		{tool: "qm", verb: "reboot", want: typedActionCatalogProxmox},
		{tool: "pct", verb: "reboot", want: typedActionCatalogProxmoxHandoff},
	} {
		if got := typedActionCatalogForProxmoxInvocation(test.tool, test.verb); got != test.want {
			t.Fatalf("%s %s catalog=%q want=%q", test.tool, test.verb, got, test.want)
		}
	}
}
