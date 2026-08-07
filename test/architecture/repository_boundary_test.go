package architecture_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func isAndroidArtifact(rel string, entry fs.DirEntry) bool {
	name := strings.ToLower(entry.Name())
	for _, suffix := range []string{".java", ".kt", ".kts"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	switch name {
	case "androidmanifest.xml", "settings.gradle", "settings.gradle.kts", "build.gradle", "build.gradle.kts", "gradlew", "gradlew.bat":
		return true
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	for index := 0; index+2 < len(parts); index++ {
		if parts[index] == "src" && parts[index+1] == "main" && (parts[index+2] == "java" || parts[index+2] == "kotlin") {
			return true
		}
	}
	return false
}

func containsAndroidBuildReference(path string, content []byte) bool {
	extension := strings.ToLower(filepath.Ext(path))
	if extension == ".md" || extension == ".txt" || extension == ".yaml" || extension == ".yml" {
		return false
	}
	text := string(content)
	return strings.Contains(text, "com.android.application") || strings.Contains(text, "com.example.pda_app")
}

func TestRepositoryContainsNoAndroidOrJVMBackendArtifacts(t *testing.T) {
	root := filepath.Clean("../..")
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if entry.IsDir() && (strings.HasPrefix(rel, "PDA_Backend_Enterprise_Strategy") || rel == ".git") {
			return filepath.SkipDir
		}
		if entry.IsDir() {
			return nil
		}
		if rel == "test/architecture/repository_boundary_test.go" {
			return nil
		}
		if isAndroidArtifact(rel, entry) {
			t.Errorf("forbidden backend artifact: %s", rel)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if containsAndroidBuildReference(rel, content) {
			t.Errorf("forbidden Android reference: %s", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryBoundaryClassification(t *testing.T) {
	tests := []struct {
		name      string
		rel       string
		file      string
		content   string
		forbidden bool
	}{
		{name: "approved markdown reference", rel: "docs/integration-pda-app/contract.md", file: "contract.md", content: "com.android.application", forbidden: false},
		{name: "java source", rel: "internal/src/main/java/com/example/App.java", file: "App.java", forbidden: true},
		{name: "kotlin source", rel: "internal/src/main/kotlin/App.kt", file: "App.kt", forbidden: true},
		{name: "android manifest", rel: "app/AndroidManifest.xml", file: "AndroidManifest.xml", forbidden: true},
		{name: "gradle build", rel: "app/build.gradle.kts", file: "build.gradle.kts", forbidden: true},
		{name: "plugin in executable config", rel: "config/app.conf", file: "app.conf", content: "com.android.application", forbidden: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			entry := boundaryFileEntry{name: test.file}
			if got := isAndroidArtifact(test.rel, entry); got != test.forbidden && test.content == "" {
				t.Fatalf("isAndroidArtifact(%q) = %v, want %v", test.rel, got, test.forbidden)
			}
			if test.content != "" {
				got := containsAndroidBuildReference(test.rel, []byte(test.content))
				if got != test.forbidden {
					t.Fatalf("containsAndroidBuildReference(%q) = %v, want %v", test.rel, got, test.forbidden)
				}
			}
		})
	}
}

type boundaryFileEntry struct {
	name string
}

func (entry boundaryFileEntry) Name() string               { return entry.name }
func (entry boundaryFileEntry) IsDir() bool                { return false }
func (entry boundaryFileEntry) Type() fs.FileMode          { return 0 }
func (entry boundaryFileEntry) Info() (fs.FileInfo, error) { return nil, nil }
