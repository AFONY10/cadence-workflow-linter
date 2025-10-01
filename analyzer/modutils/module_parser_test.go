package modutils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseGoMod(t *testing.T) {
	// Create a temporary go.mod file for testing
	tempDir := t.TempDir()
	goModPath := filepath.Join(tempDir, "go.mod")

	goModContent := `module github.com/example/test-project

go 1.21

require (
	github.com/google/uuid v1.6.0
	go.uber.org/cadence v1.0.0
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	github.com/old/pkg v1.0.0 => ./local/pkg
	github.com/another/pkg => github.com/forked/pkg v2.0.0
)
`

	err := os.WriteFile(goModPath, []byte(goModContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test go.mod: %v", err)
	}

	// Test parsing
	moduleInfo, err := ParseGoMod(goModPath)
	if err != nil {
		t.Fatalf("Failed to parse go.mod: %v", err)
	}

	// Verify module path
	if moduleInfo.ModulePath != "github.com/example/test-project" {
		t.Errorf("Expected module path 'github.com/example/test-project', got '%s'", moduleInfo.ModulePath)
	}

	// Verify go version
	if moduleInfo.GoVersion != "1.21" {
		t.Errorf("Expected go version '1.21', got '%s'", moduleInfo.GoVersion)
	}

	// Verify requires
	expectedRequires := []RequireDirective{
		{"github.com/google/uuid", "v1.6.0", false},
		{"go.uber.org/cadence", "v1.0.0", false},
		{"gopkg.in/yaml.v3", "v3.0.1", true},
	}

	if len(moduleInfo.Requires) != len(expectedRequires) {
		t.Errorf("Expected %d requires, got %d", len(expectedRequires), len(moduleInfo.Requires))
	}

	for i, expected := range expectedRequires {
		if i >= len(moduleInfo.Requires) {
			break
		}
		actual := moduleInfo.Requires[i]
		if actual.Path != expected.Path || actual.Version != expected.Version || actual.Indirect != expected.Indirect {
			t.Errorf("Require %d: expected %+v, got %+v", i, expected, actual)
		}
	}

	// Verify replaces
	expectedReplaces := []ReplaceDirective{
		{"github.com/old/pkg", "v1.0.0", "./local/pkg", ""},
		{"github.com/another/pkg", "", "github.com/forked/pkg", "v2.0.0"},
	}

	if len(moduleInfo.Replaces) != len(expectedReplaces) {
		t.Errorf("Expected %d replaces, got %d", len(expectedReplaces), len(moduleInfo.Replaces))
	}

	for i, expected := range expectedReplaces {
		if i >= len(moduleInfo.Replaces) {
			break
		}
		actual := moduleInfo.Replaces[i]
		if actual.OldPath != expected.OldPath || actual.OldVersion != expected.OldVersion ||
			actual.NewPath != expected.NewPath || actual.NewVersion != expected.NewVersion {
			t.Errorf("Replace %d: expected %+v, got %+v", i, expected, actual)
		}
	}
}

func TestIsInternalPackage(t *testing.T) {
	moduleInfo := &ModuleInfo{
		ModulePath: "github.com/example/test-project",
	}

	testCases := []struct {
		packagePath string
		expected    bool
		name        string
	}{
		{"github.com/example/test-project", true, "exact module match"},
		{"github.com/example/test-project/internal", true, "subpackage"},
		{"github.com/example/test-project/pkg/utils", true, "nested subpackage"},
		{"github.com/example/other-project", false, "different module"},
		{"github.com/google/uuid", false, "external package"},
		{"go.uber.org/cadence", false, "external framework"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := moduleInfo.IsInternalPackage(tc.packagePath)
			if result != tc.expected {
				t.Errorf("IsInternalPackage(%s): expected %v, got %v", tc.packagePath, tc.expected, result)
			}
		})
	}
}

func TestIsReplacedPackage(t *testing.T) {
	moduleInfo := &ModuleInfo{
		Replaces: []ReplaceDirective{
			{"github.com/old/pkg", "v1.0.0", "./local/pkg", ""},
			{"github.com/another/pkg", "", "github.com/forked/pkg", "v2.0.0"},
			{"github.com/local/test", "", "../testpkg", ""},
		},
	}

	testCases := []struct {
		packagePath      string
		expectedReplaced bool
		expectedNewPath  string
		name             string
	}{
		{"github.com/old/pkg", true, "./local/pkg", "exact replace match"},
		{"github.com/old/pkg/subpkg", true, "./local/pkg", "subpackage of replaced"},
		{"github.com/another/pkg", true, "github.com/forked/pkg", "replace with external"},
		{"github.com/local/test", true, "../testpkg", "replace with relative path"},
		{"github.com/unrelated/pkg", false, "", "no replacement"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			isReplaced, newPath := moduleInfo.IsReplacedPackage(tc.packagePath)
			if isReplaced != tc.expectedReplaced {
				t.Errorf("IsReplacedPackage(%s): expected replaced=%v, got %v", tc.packagePath, tc.expectedReplaced, isReplaced)
			}
			if newPath != tc.expectedNewPath {
				t.Errorf("IsReplacedPackage(%s): expected newPath='%s', got '%s'", tc.packagePath, tc.expectedNewPath, newPath)
			}
		})
	}
}

func TestFindGoMod(t *testing.T) {
	// Create temporary directory structure
	tempDir := t.TempDir()
	subDir := filepath.Join(tempDir, "subdir", "nested")
	err := os.MkdirAll(subDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create go.mod in root
	goModPath := filepath.Join(tempDir, "go.mod")
	err = os.WriteFile(goModPath, []byte("module test"), 0644)
	if err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	// Test finding from subdirectory
	foundPath, err := FindGoMod(subDir)
	if err != nil {
		t.Fatalf("Failed to find go.mod: %v", err)
	}

	if foundPath != goModPath {
		t.Errorf("Expected to find go.mod at %s, got %s", goModPath, foundPath)
	}

	// Test when go.mod doesn't exist
	nonExistentDir := t.TempDir()
	_, err = FindGoMod(nonExistentDir)
	if err == nil {
		t.Error("Expected error when go.mod doesn't exist, got nil")
	}
}
