package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/format"
	"os"
	"sort"
)

type entry struct {
	ID                string   `json:"id"`
	Origin            string   `json:"origin"`
	ResourceClass     string   `json:"resource_class"`
	ResourceKind      string   `json:"resource_kind"`
	Capability        string   `json:"capability"`
	Entrypoint        string   `json:"entrypoint"`
	Disposition       string   `json:"disposition"`
	LifecycleExecutor string   `json:"lifecycle_executor"`
	Approval          string   `json:"approval_floor"`
	Delivery          string   `json:"delivery"`
	Verification      string   `json:"verification"`
	Rollback          string   `json:"rollback"`
	ResidualOwners    []string `json:"residual_owners"`
}

func main() {
	raw, err := os.ReadFile("manifest.json")
	must(err)
	var entries []entry
	must(json.Unmarshal(raw, &entries))
	sort.Slice(entries, func(i, j int) bool { return entries[i].ID < entries[j].ID })
	var out bytes.Buffer
	out.WriteString("// Generated code; DO NOT EDIT.\n\npackage mutationregistry\n\n")
	out.WriteString("var generatedEntries = []Entry{\n")
	for _, e := range entries {
		fmt.Fprintf(&out, "\t{ID:%q, Origin:Origin(%q), ResourceClass:ResourceClass(%q), ResourceKind:%q, Capability:%q, Entrypoint:%q, Disposition:Disposition(%q), LifecycleExecutor:%q, Approval:ApprovalFloor(%q), Delivery:DeliveryClass(%q), Verification:VerificationClass(%q), Rollback:RollbackClass(%q), ResidualOwners:%#v},\n", e.ID, e.Origin, e.ResourceClass, e.ResourceKind, e.Capability, e.Entrypoint, e.Disposition, e.LifecycleExecutor, e.Approval, e.Delivery, e.Verification, e.Rollback, e.ResidualOwners)
	}
	out.WriteString("}\n\nvar generatedByID = func() map[string]Entry {\n\tout := make(map[string]Entry, len(generatedEntries))\n\tfor _, entry := range generatedEntries { out[entry.ID] = entry }\n\treturn out\n}()\n")
	formatted, err := format.Source(out.Bytes())
	must(err)
	must(os.WriteFile("registry_generated.go", formatted, 0o644))
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
