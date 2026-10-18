package installtests

import (
	"math"
	"os"
	"regexp"
	"strconv"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// Check the guide's actual worked examples against the normaliser used by
// container ingestion, History and alert evaluation, not a second formula.
func TestContainerCPUGuidanceExamplesMatchNormalisation(t *testing.T) {
	guide, err := os.ReadFile(repoFile("docs", "DOCKER.md"))
	if err != nil {
		t.Fatal(err)
	}
	rows := regexp.MustCompile(`(?m)^\| (\d+) \| ([\d.]+)% \| (?:About )?([\d.]+)% \|$`).FindAllStringSubmatch(string(guide), -1)
	if len(rows) != 3 {
		t.Fatalf("expected three worked host-capacity CPU examples, got %d", len(rows))
	}
	for _, row := range rows {
		cpus, err := strconv.Atoi(row[1])
		if err != nil {
			t.Fatal(err)
		}
		raw, err := strconv.ParseFloat(row[2], 64)
		if err != nil {
			t.Fatal(err)
		}
		printed, err := strconv.ParseFloat(row[3], 64)
		if err != nil {
			t.Fatal(err)
		}
		actual := models.NormalizeDockerContainerCPUCapacityPercent(raw, cpus)
		if math.Abs(actual-printed) > 0.005 {
			t.Errorf("%s disagrees with the production normaliser: %g", row[0], actual)
		}
	}
	for _, name := range []string{"DOCKER.md", "TROUBLESHOOTING.md"} {
		doc, err := os.ReadFile(repoFile("docs", name))
		if err != nil {
			t.Fatal(err)
		}
		shipped, err := os.ReadFile(repoFile("frontend-modern", "public", "docs", name))
		if err != nil || string(doc) != string(shipped) {
			t.Fatalf("%s shipped help does not match the checked canonical guide", name)
		}
	}
}
