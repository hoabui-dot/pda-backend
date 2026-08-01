package architecture_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestHexagonalImportBoundaries(t *testing.T) {
	root := filepath.Clean("../../internal")
	forEachGoFile(t, root, func(path string, file *ast.File) {
		normalized := filepath.ToSlash(path)
		for _, imported := range file.Imports {
			name, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(normalized, "/domain/") && (name == "net/http" || strings.Contains(name, "/adapters/") || isInfrastructureClient(name)) {
				t.Errorf("domain package %s imports forbidden dependency %s", normalized, name)
			}
			if strings.Contains(normalized, "/adapters/http/") && strings.Contains(name, "/adapters/postgres") {
				t.Errorf("HTTP adapter %s imports PostgreSQL adapter %s", normalized, name)
			}
		}
	})
}

func TestDomainAPIsDoNotExposeTransportOrPersistenceTypes(t *testing.T) {
	forEachGoFile(t, filepath.Clean("../../internal"), func(path string, file *ast.File) {
		if !strings.Contains(filepath.ToSlash(path), "/domain/") {
			return
		}
		ast.Inspect(file, func(node ast.Node) bool {
			identifier, ok := node.(*ast.Ident)
			if ok && identifier.IsExported() && (strings.HasSuffix(identifier.Name, "DTO") || strings.HasSuffix(identifier.Name, "Entity") || strings.HasSuffix(identifier.Name, "Record")) {
				t.Errorf("domain API %s exposes forbidden type name %s", path, identifier.Name)
			}
			return true
		})
	})
}

func TestKafkaAndRedisClientsStayInAdapters(t *testing.T) {
	forEachGoFile(t, filepath.Clean("../../internal"), func(path string, file *ast.File) {
		for _, imported := range file.Imports {
			name, _ := strconv.Unquote(imported.Path.Value)
			if isInfrastructureClient(name) && !strings.Contains(filepath.ToSlash(path), "/adapters/") {
				t.Errorf("infrastructure client %s imported outside adapter: %s", name, path)
			}
		}
	})
}

func forEachGoFile(t *testing.T, root string, inspect func(string, *ast.File)) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(root, "**", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	// filepath.Glob does not recurse through an arbitrary number of levels, so walk known Go files through the parser directory traversal.
	_ = matches
	fset := token.NewFileSet()
	packages, err := parser.ParseDir(fset, root, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, pkg := range packages {
		for path, file := range pkg.Files {
			inspect(path, file)
		}
	}
	entries, err := filepath.Glob(filepath.Join(root, "*", "*"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		info, err := filepath.Glob(filepath.Join(entry, "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, path := range info {
			if strings.HasSuffix(path, "_test.go") {
				continue
			}
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			inspect(path, file)
		}
		deeper, err := filepath.Glob(filepath.Join(entry, "*", "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, path := range deeper {
			if strings.HasSuffix(path, "_test.go") {
				continue
			}
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			inspect(path, file)
		}
	}
}

func isInfrastructureClient(importPath string) bool {
	return strings.Contains(importPath, "segmentio/kafka") || strings.Contains(importPath, "go-redis") || strings.Contains(importPath, "jackc/pgx")
}
