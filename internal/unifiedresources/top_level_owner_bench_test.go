package unifiedresources

import "testing"

var benchmarkTopLevelOwnerResolver TopLevelSystemResolver
var benchmarkTopLevelOwnerAttachments int

// Fixed synthetic topologies. Both variants include all index construction;
// resolver comparisons also include the matcher, all passes and full output.
// These are not installed fleet CPU/RSS or release-performance thresholds.
func BenchmarkTopLevelOwnerIndexResolver(b *testing.B) {
	for _, fixture := range topLevelOwnerIndexFixtures() {
		b.Run(fixture.name, func(b *testing.B) {
			for _, method := range []struct {
				name    string
				resolve func([]Resource) TopLevelSystemResolver
			}{{"before", resolveTopLevelSystemsBeforeOwnerIndex}, {"indexed", ResolveTopLevelSystems}} {
				b.Run(method.name, func(b *testing.B) {
					b.ReportAllocs()
					for i := 0; i < b.N; i++ {
						benchmarkTopLevelOwnerResolver = method.resolve(fixture.resources)
					}
				})
			}
		})
	}
}

func BenchmarkTopLevelOwnerIndexPass(b *testing.B) {
	for _, fixture := range topLevelOwnerIndexFixtures() {
		nodes := make([]topLevelSystemNode, 0, len(fixture.resources))
		parents := make([]int, len(fixture.resources))
		for i := range fixture.resources {
			nodes = append(nodes, buildTopLevelSystemNode(&fixture.resources[i]))
			parents[i] = i
		}
		groups := buildTopLevelSystemResolvedGroups(nodes, parents, nil)
		best := int(^uint(0) >> 1)
		for _, group := range groups {
			if group.priority < best {
				best = group.priority
			}
		}
		b.Run(fixture.name, func(b *testing.B) {
			for _, method := range []string{"before", "indexed"} {
				b.Run(method, func(b *testing.B) {
					b.ReportAllocs()
					for i := 0; i < b.N; i++ {
						attached := 0
						if method == "before" {
							hosts, ips := buildTopLevelSystemFallbackOwners(groups)
							forms := buildTopLevelSystemHostFormOwners(groups)
							for root, group := range groups {
								if !group.attachByHost || group.priority <= best {
									continue
								}
								if _, ok := uniqueBetterTopLevelSystemTargetBeforeOwnerIndex(root, group, hosts, ips, forms, groups); ok {
									attached++
								}
							}
						} else {
							index := buildTopLevelSystemFallbackIndex(groups)
							for root, group := range groups {
								if !group.attachByHost || group.priority <= best {
									continue
								}
								if _, ok := uniqueBetterTopLevelSystemTarget(root, group, index, groups); ok {
									attached++
								}
							}
						}
						benchmarkTopLevelOwnerAttachments = attached
					}
				})
			}
		})
	}
}
