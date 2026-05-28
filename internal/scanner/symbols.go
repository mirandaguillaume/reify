package scanner

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

// ExtractSymbols walks Go source files under root and extracts exported symbols
// using go/parser (stdlib, zero external deps).
func ExtractSymbols(root string) ([]SymbolEntry, error) {
	var symbols []SymbolEntry
	fset := token.NewFileSet()

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if SkipDirs[name] || name == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}

		// Only parse .go files, skip tests.
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		f, parseErr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if parseErr != nil {
			return nil // skip unparseable files
		}

		rel, _ := filepath.Rel(root, path)
		pkgName := f.Name.Name

		for _, decl := range f.Decls {
			switch d := decl.(type) {
			case *ast.GenDecl:
				symbols = append(symbols, extractGenDecl(d, pkgName, rel)...)
			case *ast.FuncDecl:
				if sym, ok := extractFuncDecl(d, pkgName, rel); ok {
					symbols = append(symbols, sym)
				}
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Sort by package, then name for stable output.
	sort.Slice(symbols, func(i, j int) bool {
		if symbols[i].File != symbols[j].File {
			return symbols[i].File < symbols[j].File
		}
		return symbols[i].Name < symbols[j].Name
	})

	return symbols, nil
}

func extractGenDecl(decl *ast.GenDecl, pkg, file string) []SymbolEntry {
	var syms []SymbolEntry

	for _, spec := range decl.Specs {
		switch s := spec.(type) {
		case *ast.TypeSpec:
			name := s.Name.Name
			if !isExported(name) {
				continue
			}
			kind := "type"
			switch s.Type.(type) {
			case *ast.StructType:
				kind = "struct"
			case *ast.InterfaceType:
				kind = "interface"
			}
			syms = append(syms, SymbolEntry{
				Package:  pkg,
				Name:     name,
				Kind:     kind,
				File:     file,
				Exported: true,
			})

		case *ast.ValueSpec:
			for _, ident := range s.Names {
				if !isExported(ident.Name) {
					continue
				}
				kind := "var"
				if decl.Tok == token.CONST {
					kind = "const"
				}
				syms = append(syms, SymbolEntry{
					Package:  pkg,
					Name:     ident.Name,
					Kind:     kind,
					File:     file,
					Exported: true,
				})
			}
		}
	}

	return syms
}

func extractFuncDecl(decl *ast.FuncDecl, pkg, file string) (SymbolEntry, bool) {
	name := decl.Name.Name
	if !isExported(name) {
		return SymbolEntry{}, false
	}

	kind := "func"
	if decl.Recv != nil && len(decl.Recv.List) > 0 {
		kind = "method"
	}

	return SymbolEntry{
		Package:  pkg,
		Name:     name,
		Kind:     kind,
		File:     file,
		Exported: true,
	}, true
}

func isExported(name string) bool {
	if name == "" {
		return false
	}
	return unicode.IsUpper(rune(name[0]))
}
