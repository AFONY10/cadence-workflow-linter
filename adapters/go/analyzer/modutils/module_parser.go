package modutils

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ModuleInfo contains parsed information from go.mod
type ModuleInfo struct {
	ModulePath string             // The module declaration path
	GoVersion  string             // Go version requirement
	Requires   []RequireDirective // Direct dependencies
	Replaces   []ReplaceDirective // Replace directives
	RootDir    string             // Directory containing go.mod
}

// RequireDirective represents a require statement
type RequireDirective struct {
	Path     string
	Version  string
	Indirect bool // // indirect comment
}

// ReplaceDirective represents a replace statement
type ReplaceDirective struct {
	OldPath    string
	OldVersion string // empty if no version specified
	NewPath    string
	NewVersion string // empty for local paths
}

// ParseGoMod parses a go.mod file and returns module information
func ParseGoMod(goModPath string) (*ModuleInfo, error) {
	file, err := os.Open(goModPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open go.mod: %w", err)
	}
	defer file.Close()

	info := &ModuleInfo{
		RootDir:  filepath.Dir(goModPath),
		Requires: make([]RequireDirective, 0),
		Replaces: make([]ReplaceDirective, 0),
	}

	scanner := bufio.NewScanner(file)
	var inRequireBlock, inReplaceBlock bool

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		if line == ")" {
			inRequireBlock = false
			inReplaceBlock = false
			continue
		}

		if strings.HasPrefix(line, "module ") {
			info.ModulePath = strings.TrimSpace(strings.TrimPrefix(line, "module"))
			continue
		}

		if strings.HasPrefix(line, "go ") {
			info.GoVersion = strings.TrimSpace(strings.TrimPrefix(line, "go"))
			continue
		}

		if strings.HasPrefix(line, "require (") {
			inRequireBlock = true
			continue
		}

		if strings.HasPrefix(line, "replace (") {
			inReplaceBlock = true
			continue
		}

		if strings.HasPrefix(line, "require ") {
			if req := parseRequireLine(strings.TrimPrefix(line, "require ")); req != nil {
				info.Requires = append(info.Requires, *req)
			}
			continue
		}

		if strings.HasPrefix(line, "replace ") {
			if rep := parseReplaceLine(strings.TrimPrefix(line, "replace ")); rep != nil {
				info.Replaces = append(info.Replaces, *rep)
			}
			continue
		}

		if inRequireBlock {
			if req := parseRequireLine(line); req != nil {
				info.Requires = append(info.Requires, *req)
			}
			continue
		}

		if inReplaceBlock {
			if rep := parseReplaceLine(line); rep != nil {
				info.Replaces = append(info.Replaces, *rep)
			}
			continue
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading go.mod: %w", err)
	}

	return info, nil
}

func parseRequireLine(line string) *RequireDirective {
	if idx := strings.Index(line, "//"); idx >= 0 {
		comment := strings.TrimSpace(line[idx+2:])
		line = strings.TrimSpace(line[:idx])

		fields := strings.Fields(line)
		if len(fields) >= 2 {
			return &RequireDirective{
				Path:     fields[0],
				Version:  fields[1],
				Indirect: strings.Contains(comment, "indirect"),
			}
		}
	} else {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			return &RequireDirective{
				Path:    fields[0],
				Version: fields[1],
			}
		}
	}
	return nil
}

func parseReplaceLine(line string) *ReplaceDirective {
	if idx := strings.Index(line, "//"); idx >= 0 {
		line = strings.TrimSpace(line[:idx])
	}

	parts := strings.Split(line, "=>")
	if len(parts) != 2 {
		return nil
	}

	oldPart := strings.TrimSpace(parts[0])
	newPart := strings.TrimSpace(parts[1])

	oldFields := strings.Fields(oldPart)
	newFields := strings.Fields(newPart)

	if len(oldFields) == 0 || len(newFields) == 0 {
		return nil
	}

	replace := &ReplaceDirective{
		OldPath: oldFields[0],
		NewPath: newFields[0],
	}

	if len(oldFields) > 1 {
		replace.OldVersion = oldFields[1]
	}

	if len(newFields) > 1 {
		replace.NewVersion = newFields[1]
	}

	return replace
}

func FindGoMod(startDir string) (string, error) {
	currentDir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}

	for {
		goModPath := filepath.Join(currentDir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return goModPath, nil
		}
		parent := filepath.Dir(currentDir)
		if parent == currentDir {
			break
		}
		currentDir = parent
	}
	return "", fmt.Errorf("go.mod not found")
}

func (m *ModuleInfo) IsInternalPackage(packagePath string) bool {
	if m.ModulePath == "" {
		return false
	}
	return packagePath == m.ModulePath || strings.HasPrefix(packagePath, m.ModulePath+"/")
}

func (m *ModuleInfo) IsReplacedPackage(packagePath string) (bool, string) {
	for _, replace := range m.Replaces {
		if packagePath == replace.OldPath || strings.HasPrefix(packagePath, replace.OldPath+"/") {
			if !strings.Contains(replace.NewPath, "/") || strings.HasPrefix(replace.NewPath, "./") || strings.HasPrefix(replace.NewPath, "../") {
				return true, replace.NewPath
			}
			return true, replace.NewPath
		}
	}
	return false, ""
}

func (m *ModuleInfo) GetDirectDependencies() []string {
	var deps []string
	for _, req := range m.Requires {
		if !req.Indirect {
			deps = append(deps, req.Path)
		}
	}
	return deps
}
