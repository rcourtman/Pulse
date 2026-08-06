package vmware

import (
	"context"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	// vmwareTagObjectBatchSize caps how many inventory objects ride in one
	// list-attached-tags-on-objects request. vCenter accepts large object
	// lists, but a bounded batch keeps a big estate from building one
	// unbounded request body and one unbounded response.
	vmwareTagObjectBatchSize = 400

	// vmwareTagCatalogTTL bounds how long resolved tag and category names are
	// reused. Associations are re-read every refresh because they follow the
	// estate; names change rarely, so a renamed tag converges within one TTL
	// instead of needing a Pulse restart.
	vmwareTagCatalogTTL = 10 * time.Minute

	// vmwareTagMetadataConcurrency bounds the cold-start metadata fan-out. The
	// catalog makes this a once-per-TTL cost, but an estate with many distinct
	// tags should not serialize that first refresh.
	vmwareTagMetadataConcurrency = 8
)

// vCenter managed object type names used by the CIS tagging association API.
const (
	vmwareTagObjectTypeHost = "HostSystem"
	vmwareTagObjectTypeVM   = "VirtualMachine"
)

// cisObjectID identifies one inventory object to the CIS tagging service.
type cisObjectID struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// cisTagAssociation is one entry of the batched association response.
type cisTagAssociation struct {
	ObjectID cisObjectID `json:"object_id"`
	TagIDs   []string    `json:"tag_ids"`
}

type cisTagInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CategoryID  string `json:"category_id"`
}

type cisCategoryInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// vmwareTagCatalog caches resolved vCenter tag and category names for one
// client. Tag identifiers are opaque URNs, so every tagged object would
// otherwise cost two extra GETs per refresh just to learn names that the whole
// estate shares. The catalog reduces the steady state to the association reads
// alone.
type vmwareTagCatalog struct {
	mu         sync.Mutex
	loadedAt   time.Time
	tags       map[string]InventoryTag
	categories map[string]string
}

// beginRefresh drops the cached catalog once its TTL lapses so renames and
// deletions in vCenter converge on the next refresh.
func (t *vmwareTagCatalog) beginRefresh(now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.tags == nil || t.categories == nil {
		t.tags = make(map[string]InventoryTag)
		t.categories = make(map[string]string)
		t.loadedAt = now
		return
	}
	if now.Sub(t.loadedAt) < vmwareTagCatalogTTL {
		return
	}
	t.tags = make(map[string]InventoryTag)
	t.categories = make(map[string]string)
	t.loadedAt = now
}

func (t *vmwareTagCatalog) tag(id string) (InventoryTag, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	tag, ok := t.tags[id]
	return tag, ok
}

func (t *vmwareTagCatalog) putTag(id string, tag InventoryTag) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.tags == nil {
		t.tags = make(map[string]InventoryTag)
	}
	t.tags[id] = tag
}

func (t *vmwareTagCatalog) category(id string) (string, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	name, ok := t.categories[id]
	return name, ok
}

func (t *vmwareTagCatalog) putCategory(id string, name string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.categories == nil {
		t.categories = make(map[string]string)
	}
	t.categories[id] = name
}

// enrichInventoryTags attaches operator-authored vCenter tags to the hosts and
// VMs already present in snapshot.
//
// Tags live in the CIS tagging service (`/api/cis/tagging/...`), not in the
// `/api/vcenter/...` inventory endpoints the rest of this client reads. Both
// are the vSphere Automation API and share the `/api/session` token, so this
// reuses the caller's Automation session rather than opening a second one.
//
// The read is optional: an account without the tagging read privilege, or a
// vCenter with the service unavailable, degrades into an enrichment issue and
// leaves the inventory untagged rather than failing the refresh.
func (c *Client) enrichInventoryTags(
	ctx context.Context,
	automationSessionID string,
	snapshot *InventorySnapshot,
) ([]InventoryEnrichmentIssue, error) {
	if snapshot == nil {
		return nil, nil
	}

	hostIndexByID := make(map[string]int, len(snapshot.Hosts))
	vmIndexByID := make(map[string]int, len(snapshot.VMs))
	refs := make([]cisObjectID, 0, len(snapshot.Hosts)+len(snapshot.VMs))
	for i := range snapshot.Hosts {
		id := strings.TrimSpace(snapshot.Hosts[i].Host)
		if id == "" {
			continue
		}
		hostIndexByID[id] = i
		refs = append(refs, cisObjectID{ID: id, Type: vmwareTagObjectTypeHost})
	}
	for i := range snapshot.VMs {
		id := strings.TrimSpace(snapshot.VMs[i].VM)
		if id == "" {
			continue
		}
		vmIndexByID[id] = i
		refs = append(refs, cisObjectID{ID: id, Type: vmwareTagObjectTypeVM})
	}
	if len(refs) == 0 {
		return nil, nil
	}

	associations, err := c.listAttachedTagsOnObjects(ctx, automationSessionID, refs)
	if issue, ok := classifyInventoryEnrichmentIssue("tags", "tag", "", err); ok {
		return []InventoryEnrichmentIssue{*issue}, nil
	} else if err != nil {
		return nil, err
	}

	resolved, issues, err := c.resolveTagCatalog(ctx, automationSessionID, distinctTagIDs(associations))
	if err != nil {
		return nil, err
	}

	for _, association := range associations {
		objectID := strings.TrimSpace(association.ObjectID.ID)
		if objectID == "" {
			continue
		}
		tags := make([]InventoryTag, 0, len(association.TagIDs))
		for _, tagID := range association.TagIDs {
			if tag, ok := resolved[strings.TrimSpace(tagID)]; ok {
				tags = append(tags, tag)
			}
		}
		if len(tags) == 0 {
			continue
		}
		sortInventoryTags(tags)
		switch {
		case strings.EqualFold(strings.TrimSpace(association.ObjectID.Type), vmwareTagObjectTypeHost):
			if index, ok := hostIndexByID[objectID]; ok {
				snapshot.Hosts[index].Tags = tags
			}
		case strings.EqualFold(strings.TrimSpace(association.ObjectID.Type), vmwareTagObjectTypeVM):
			if index, ok := vmIndexByID[objectID]; ok {
				snapshot.VMs[index].Tags = tags
			}
		}
	}

	sort.Slice(issues, func(i, j int) bool {
		return inventoryEnrichmentIssueSortKey(issues[i]) < inventoryEnrichmentIssueSortKey(issues[j])
	})
	return issues, nil
}

// listAttachedTagsOnObjects reads tag associations for every supplied object in
// bounded batches. This is the only per-refresh tagging cost for an estate
// whose tag names are already cached.
func (c *Client) listAttachedTagsOnObjects(
	ctx context.Context,
	sessionID string,
	refs []cisObjectID,
) ([]cisTagAssociation, error) {
	out := make([]cisTagAssociation, 0, len(refs))
	for start := 0; start < len(refs); start += vmwareTagObjectBatchSize {
		end := start + vmwareTagObjectBatchSize
		if end > len(refs) {
			end = len(refs)
		}
		payload := struct {
			ObjectIDs []cisObjectID `json:"object_ids"`
		}{ObjectIDs: refs[start:end]}
		var batch []cisTagAssociation
		if err := c.postSessionScopedJSON(
			ctx,
			sessionID,
			"/api/cis/tagging/tag-association?action=list-attached-tags-on-objects",
			"vcenter tag associations",
			payload,
			&batch,
		); err != nil {
			return nil, err
		}
		out = append(out, batch...)
	}
	return out, nil
}

// resolveTagCatalog turns the opaque tag URNs carried by associations into
// named tags, reading only the identifiers the catalog has not already cached.
// A tag whose metadata cannot be read is reported and skipped: one unreadable
// tag must not blank out the tags Pulse can resolve.
func (c *Client) resolveTagCatalog(
	ctx context.Context,
	sessionID string,
	tagIDs []string,
) (map[string]InventoryTag, []InventoryEnrichmentIssue, error) {
	resolved := make(map[string]InventoryTag, len(tagIDs))
	if len(tagIDs) == 0 {
		return resolved, nil, nil
	}

	c.tagCatalog.beginRefresh(time.Now())

	// Cached hits are drained first so every write to `resolved` from this
	// goroutine happens before any worker starts writing to it.
	missing := make([]string, 0, len(tagIDs))
	for _, tagID := range tagIDs {
		if cached, ok := c.tagCatalog.tag(tagID); ok {
			resolved[tagID] = cached
			continue
		}
		missing = append(missing, tagID)
	}
	if len(missing) == 0 {
		return resolved, nil, nil
	}

	var (
		mu       sync.Mutex
		issues   []InventoryEnrichmentIssue
		firstErr error
		wg       sync.WaitGroup
	)
	sem := make(chan struct{}, vmwareTagMetadataConcurrency)

	for _, tagID := range missing {
		wg.Add(1)
		go func(tagID string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			tag, err := c.fetchTag(ctx, sessionID, tagID)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if issue, ok := classifyInventoryEnrichmentIssue("tags", "tag", tagID, err); ok {
					issues = append(issues, *issue)
					return
				}
				if firstErr == nil {
					firstErr = err
				}
				return
			}
			// A tag whose category could not be resolved is usable but
			// incomplete, so it is not cached: caching it would pin the
			// degraded label for the rest of the TTL instead of retrying on
			// the next refresh.
			if tag.CategoryID == "" || tag.Category != "" {
				c.tagCatalog.putTag(tagID, tag)
			}
			resolved[tagID] = tag
		}(tagID)
	}
	wg.Wait()

	if firstErr != nil {
		return nil, nil, firstErr
	}
	return resolved, issues, nil
}

// fetchTag reads one tag and resolves its category name, reusing the cached
// category when another tag in the same category already resolved it.
func (c *Client) fetchTag(ctx context.Context, sessionID string, tagID string) (InventoryTag, error) {
	var info cisTagInfo
	if err := c.getSessionScopedJSON(
		ctx,
		sessionID,
		"/api/cis/tagging/tag/"+url.PathEscape(tagID),
		"vcenter tag",
		&info,
	); err != nil {
		return InventoryTag{}, err
	}

	tag := InventoryTag{
		TagID:       strings.TrimSpace(firstNonEmptyTrimmed(info.ID, tagID)),
		Name:        strings.TrimSpace(info.Name),
		CategoryID:  strings.TrimSpace(info.CategoryID),
		Description: strings.TrimSpace(info.Description),
	}
	if tag.Name == "" {
		return InventoryTag{}, &ConnectionError{
			Category: "endpoint",
			Message:  "VMware vcenter tag response did not include a tag name",
		}
	}
	if tag.CategoryID == "" {
		return tag, nil
	}

	if name, ok := c.tagCatalog.category(tag.CategoryID); ok {
		tag.Category = name
		return tag, nil
	}
	var category cisCategoryInfo
	if err := c.getSessionScopedJSON(
		ctx,
		sessionID,
		"/api/cis/tagging/category/"+url.PathEscape(tag.CategoryID),
		"vcenter tag category",
		&category,
	); err != nil {
		// A readable tag whose category is not readable is still a usable
		// label; report nothing and fall back to the bare tag name.
		return tag, nil
	}
	tag.Category = strings.TrimSpace(category.Name)
	if tag.Category != "" {
		c.tagCatalog.putCategory(tag.CategoryID, tag.Category)
	}
	return tag, nil
}

func distinctTagIDs(associations []cisTagAssociation) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, len(associations))
	for _, association := range associations {
		for _, tagID := range association.TagIDs {
			tagID = strings.TrimSpace(tagID)
			if tagID == "" {
				continue
			}
			if _, ok := seen[tagID]; ok {
				continue
			}
			seen[tagID] = struct{}{}
			out = append(out, tagID)
		}
	}
	sort.Strings(out)
	return out
}

func sortInventoryTags(tags []InventoryTag) {
	sort.Slice(tags, func(i, j int) bool {
		return inventoryTagSortKey(tags[i]) < inventoryTagSortKey(tags[j])
	})
}

func inventoryTagSortKey(tag InventoryTag) string {
	return strings.ToLower(strings.TrimSpace(tag.Category)) + "\x00" +
		strings.ToLower(strings.TrimSpace(tag.Name)) + "\x00" +
		strings.TrimSpace(tag.TagID)
}

// InventoryTagLabel renders one vCenter tag as the flat label Pulse uses in
// tag search, tag filters, and the workload Tags column. vCenter tag names are
// only unique inside their category, so the category is part of the label
// whenever vCenter reports one.
func InventoryTagLabel(tag InventoryTag) string {
	name := strings.TrimSpace(tag.Name)
	if name == "" {
		return ""
	}
	category := strings.TrimSpace(tag.Category)
	if category == "" {
		return name
	}
	return category + ":" + name
}
