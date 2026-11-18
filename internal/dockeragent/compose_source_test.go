package dockeragent

import (
	"bytes"
	"strings"
	"testing"
)

func TestComposeImageEditPreservesOtherServicesAndFormatting(t *testing.T) {
	input := []byte("# production\r\nservices:\r\n  api:\r\n    image: 'registry.example:5000/app:1' # update only this\r\n  worker:\r\n    image: 'registry.example:5000/app:1'\r\n")
	digest := "sha256:" + strings.Repeat("a", 64)
	before, after, updated, err := composeImageEdit(input, "api", digest)
	if err != nil || before != "registry.example:5000/app:1" || after != before+"@"+digest {
		t.Fatalf("edit: %s %s %v", before, after, err)
	}
	if !bytes.Equal(updated, bytes.Replace(input, []byte(before), []byte(after), 1)) {
		t.Fatal("unrelated source bytes changed")
	}
	for _, data := range []string{
		"services:\n  api:\n    image: ${IMAGE}\n",
		"services:\n  api:\n    image: web\n    image: other\n",
		"services:\n  api:\n    image: web\n    build: .\n",
		"base: &base {image: web}\nservices: {api: *base}\n",
		"services: {api: {image: web}}\n---\nservices: {}\n",
	} {
		if _, _, _, err := composeImageEdit([]byte(data), "api", digest); err == nil {
			t.Errorf("accepted indirect or ambiguous image in %q", data)
		}
	}
}

func TestComposeImageEditKeepsImplicitLatestTrackable(t *testing.T) {
	digest := "sha256:" + strings.Repeat("d", 64)
	_, after, _, err := composeImageEdit([]byte("services: {web: {image: 'registry.example:5000/web'}}"), "web", digest)
	if err != nil || after != "registry.example:5000/web:latest@"+digest {
		t.Fatalf("reference %q: %v", after, err)
	}
}
