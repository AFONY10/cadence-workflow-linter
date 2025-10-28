package analyzer

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"go/types"

	"golang.org/x/tools/go/packages"

	"github.com/afony10/cadence-workflow-linter/adapters/go/analyzer/detectors"
	"github.com/afony10/cadence-workflow-linter/adapters/go/analyzer/modutils"
	"github.com/afony10/cadence-workflow-linter/adapters/go/analyzer/registry"
)

// PackageResolver handles package path resolution using hybrid approach
type PackageResolver struct {
	moduleInfo *modutils.ModuleInfo
	baseDir    string
}

// NewPackageResolver creates a resolver with go.mod parsing and fallback heuristics
func NewPackageResolver(baseDir string) *PackageResolver {
	resolver := &PackageResolver{baseDir: baseDir}

	if goModPath, err := modutils.FindGoMod(baseDir); err == nil {
		if moduleInfo, err := modutils.ParseGoMod(goModPath); err == nil {
			resolver.moduleInfo = moduleInfo
		}
	}

	return resolver

}

func (pr *PackageResolver) computePackagePath(filePath string, node *ast.File) string {
	pkgName := "local"
	if node.Name != nil {
		pkgName = node.Name.Name
	}

	if strings.Contains(filePath, "testdata") {
		if strings.Contains(filePath, string(filepath.Separator)+"mod"+string(filepath.Separator)) {
			rel, err := filepath.Rel(pr.baseDir, filepath.Dir(filePath))
			if err == nil {
				parts := strings.Split(filepath.ToSlash(rel), "/")
				if len(parts) >= 2 && parts[0] == "mod" {
					return "example.com/linttest/" + strings.Join(parts[1:], "/")
				}
			}
		}
		return "testdata/" + pkgName
	}

	if pr.moduleInfo != nil {
		modulePath := pr.moduleInfo.ModulePath
		if pkgName == "main" {
			return modulePath
		}
		rel, err := filepath.Rel(pr.moduleInfo.RootDir, filepath.Dir(filePath))
		if err == nil && rel != "." {
			subPath := strings.ReplaceAll(rel, string(filepath.Separator), "/")
			return modulePath + "/" + subPath
		}
		return modulePath
	}

	if pkgName == "main" {
		return "github.com/afony10/cadence-workflow-linter"
	}
	rel, err := filepath.Rel(pr.baseDir, filepath.Dir(filePath))
	if err == nil && rel != "." {
		return "github.com/afony10/cadence-workflow-linter/" + strings.ReplaceAll(rel, string(filepath.Separator), "/")
	}
	return pkgName
}

type parsedFile struct {
	filename  string
	fset      *token.FileSet
	node      *ast.File
	importMap map[string]string
	pkgPath   string
	typesInfo *types.Info
}

func buildImportMap(f *ast.File) map[string]string {
	m := make(map[string]string)
	for _, imp := range f.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		alias := ""
		if imp.Name != nil && imp.Name.Name != "" && imp.Name.Name != "_" && imp.Name.Name != "." {
			alias = imp.Name.Name
		} else {
			if i := strings.LastIndex(path, "/"); i >= 0 {
				alias = path[i+1:]
			} else {
				alias = path
			}
		}
		m[alias] = path
	}
	return m
}

func parseAllAndBuildRegistry(target string) ([]parsedFile, *registry.WorkflowRegistry, *modutils.ModuleInfo, error) {
	var files []parsedFile
	wr := registry.NewWorkflowRegistry()

	baseDir := target
	if info, err := os.Stat(target); err == nil && !info.IsDir() {
		baseDir = filepath.Dir(target)
	}

	resolver := NewPackageResolver(baseDir)

	addFile := func(path string) error {
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, path, src, parser.AllErrors)
		if err != nil {
			return err
		}

		importMap := buildImportMap(node)
		pkgPath := resolver.computePackagePath(path, node)

		files = append(files, parsedFile{
			filename:  path,
			fset:      fset,
			node:      node,
			importMap: importMap,
			pkgPath:   pkgPath,
		})

		wr.ProcessFile(node, pkgPath, importMap)
		return nil
	}

	info, err := os.Stat(target)
	if err != nil {
		return nil, nil, nil, err
	}
	if info.IsDir() {
		err = filepath.Walk(target, func(path string, fi os.FileInfo, _ error) error {
			if fi == nil || fi.IsDir() || filepath.Ext(path) != ".go" {
				return nil
			}
			return addFile(path)
		})
	} else {
		err = addFile(target)
	}
	if err != nil {
		return nil, nil, nil, err
	}

	cfg := &packages.Config{Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo, Dir: baseDir}
	pkgs, perr := packages.Load(cfg, "./...")
	if perr == nil {
		typesMap := make(map[string]*types.Info)
		for _, p := range pkgs {
			for i, f := range p.GoFiles {
				if i < len(p.Syntax) {
					typesMap[f] = p.TypesInfo
				}
			}
		}
		for i, pf := range files {
			if ti, ok := typesMap[pf.filename]; ok {
				files[i].typesInfo = ti
			}
		}
	}

	return files, wr, resolver.moduleInfo, nil
}

func runDetectors(files []parsedFile, wr *registry.WorkflowRegistry, moduleInfo *modutils.ModuleInfo, factory func(*modutils.ModuleInfo) []ast.Visitor) ([]detectors.Issue, error) {
	var all []detectors.Issue
	for _, pf := range files {
		visitors := factory(moduleInfo)
		ctx := detectors.FileContext{File: pf.filename, Fset: pf.fset, ImportMap: pf.importMap, Node: pf.node, TypesInfo: pf.typesInfo}
		for _, v := range visitors {
			if wa, ok := v.(detectors.WorkflowAware); ok {
				wa.SetWorkflowRegistry(wr)
			}
			if fca, ok := v.(detectors.FileContextAware); ok {
				fca.SetFileContext(ctx)
			}
			if pa, ok := v.(detectors.PackageAware); ok {
				pa.SetPackagePath(pf.pkgPath)
			}
			ast.Walk(v, pf.node)
			if ip, ok := v.(detectors.IssueProvider); ok {
				all = append(all, ip.Issues()...)
			}
		}
	}
	return all, nil
}

func ScanFile(path string, factory func(*modutils.ModuleInfo) []ast.Visitor) ([]detectors.Issue, error) {
	files, wr, moduleInfo, err := parseAllAndBuildRegistry(path)
	if err != nil {
		return nil, err
	}
	return runDetectors(files, wr, moduleInfo, factory)
}

func ScanDirectory(root string, factory func(*modutils.ModuleInfo) []ast.Visitor) ([]detectors.Issue, error) {
	files, wr, moduleInfo, err := parseAllAndBuildRegistry(root)
	if err != nil {
		return nil, err
	}
	return runDetectors(files, wr, moduleInfo, factory)
}
