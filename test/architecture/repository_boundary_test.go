package architecture_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepositoryContainsNoAndroidOrJVMBackendArtifacts(t *testing.T) {
	root := filepath.Clean("../..")
	forbiddenSuffixes := []string{".java", ".kt", ".kts", "AndroidManifest.xml", "gradlew", "gradlew.bat"}
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
		for _, suffix := range forbiddenSuffixes {
			if strings.HasSuffix(entry.Name(), suffix) {
				t.Errorf("forbidden backend artifact: %s", rel)
			}
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(content), "com.android.application") || strings.Contains(string(content), "com.example.pda_app") {
			t.Errorf("forbidden Android reference: %s", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
