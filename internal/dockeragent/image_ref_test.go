package dockeragent

import "testing"

func TestMatchesImageReference(t *testing.T) {
	tests := []struct {
		name      string
		imageName string
		repoRef   string
		want      bool
	}{
		{
			name:      "exact match",
			imageName: "nginx",
			repoRef:   "nginx",
			want:      true,
		},
		{
			name:      "docker hub library image",
			imageName: "nginx",
			repoRef:   "docker.io/library/nginx",
			want:      true,
		},
		{
			name:      "docker hub library image with tag",
			imageName: "nginx:latest",
			repoRef:   "docker.io/library/nginx",
			want:      true,
		},
		{
			name:      "docker hub library image with specific tag",
			imageName: "nginx:1.25",
			repoRef:   "docker.io/library/nginx",
			want:      true,
		},
		{
			name:      "docker hub with namespace",
			imageName: "myuser/myapp",
			repoRef:   "docker.io/myuser/myapp",
			want:      true,
		},
		{
			name:      "docker hub with namespace and tag",
			imageName: "myuser/myapp:v1.0",
			repoRef:   "docker.io/myuser/myapp",
			want:      true,
		},
		{
			name:      "ghcr.io registry",
			imageName: "ghcr.io/user/repo",
			repoRef:   "ghcr.io/user/repo",
			want:      true,
		},
		{
			name:      "ghcr.io registry with tag",
			imageName: "ghcr.io/user/repo:latest",
			repoRef:   "ghcr.io/user/repo",
			want:      true,
		},
		{
			name:      "quay.io registry",
			imageName: "quay.io/org/image",
			repoRef:   "quay.io/org/image",
			want:      true,
		},
		{
			name:      "different images should not match",
			imageName: "nginx",
			repoRef:   "docker.io/library/redis",
			want:      false,
		},
		{
			name:      "different namespaces should not match",
			imageName: "user1/app",
			repoRef:   "docker.io/user2/app",
			want:      false,
		},
		{
			name:      "image with port in registry should preserve port",
			imageName: "localhost:5000/myimage:tag",
			repoRef:   "localhost:5000/myimage",
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesImageReference(tt.imageName, tt.repoRef)
			if got != tt.want {
				t.Errorf("matchesImageReference(%q, %q) = %v, want %v", tt.imageName, tt.repoRef, got, tt.want)
			}
		})
	}
}
