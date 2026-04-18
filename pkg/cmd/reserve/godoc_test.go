package reserve_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// TestExportedFunctionsHaveGodoc verifies that all exported functions in the
// reserve package have godoc comments that begin with the function name
// (standard Go convention) and are at least 10 words long (not trivially short).
func TestExportedFunctionsHaveGodoc(t *testing.T) {
	// The four functions called out in grava-66ba, plus other key exports.
	requiredFuncs := map[string]bool{
		"DeclareReservation":    false,
		"ReleaseReservation":    false,
		"ListReservations":      false,
		"CheckStagedConflicts":  false,
		"GetReservation":        false,
		"WriteReservationArtifact": false,
		"AddCommands":           false,
	}

	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("failed to parse package: %v", err)
	}

	for _, pkg := range pkgs {
		if pkg.Name == "reserve_test" {
			continue // skip test files
		}
		for filePath, file := range pkg.Files {
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Recv != nil { // skip methods and non-function decls
					continue
				}
				name := fn.Name.Name
				if _, tracked := requiredFuncs[name]; !tracked {
					continue
				}
				requiredFuncs[name] = true // mark as found

				// Must have a doc comment
				if fn.Doc == nil || fn.Doc.Text() == "" {
					t.Errorf("%s:%d: exported function %s has no godoc comment",
						filePath, fset.Position(fn.Pos()).Line, name)
					continue
				}

				doc := strings.TrimSpace(fn.Doc.Text())

				// Godoc must start with the function name (Go convention)
				if !strings.HasPrefix(doc, name) {
					t.Errorf("%s:%d: godoc for %s should start with %q, got: %q",
						filePath, fset.Position(fn.Pos()).Line, name, name, firstLine(doc))
				}

				// Must be at least 10 words to be meaningfully descriptive
				words := strings.Fields(doc)
				if len(words) < 10 {
					t.Errorf("%s:%d: godoc for %s is too short (%d words, need >=10): %q",
						filePath, fset.Position(fn.Pos()).Line, name, len(words), doc)
				}
			}
		}
	}

	// Verify all required functions were found in the package
	for name, found := range requiredFuncs {
		if !found {
			t.Errorf("exported function %s not found in package — was it removed?", name)
		}
	}
}

// TestExportedTypesHaveGodoc verifies that exported types have godoc.
func TestExportedTypesHaveGodoc(t *testing.T) {
	requiredTypes := map[string]bool{
		"Reservation":    false,
		"DeclareParams":  false,
		"DeclareResult":  false,
		"Conflict":       false,
	}

	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("failed to parse package: %v", err)
	}

	for _, pkg := range pkgs {
		if pkg.Name == "reserve_test" {
			continue
		}
		for filePath, file := range pkg.Files {
			for _, decl := range file.Decls {
				gd, ok := decl.(*ast.GenDecl)
				if !ok || gd.Tok != token.TYPE {
					continue
				}
				for _, spec := range gd.Specs {
					ts, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					name := ts.Name.Name
					if _, tracked := requiredTypes[name]; !tracked {
						continue
					}
					requiredTypes[name] = true

					// Check for doc on GenDecl (Go attaches doc to the group, not the spec)
					if gd.Doc == nil || gd.Doc.Text() == "" {
						t.Errorf("%s:%d: exported type %s has no godoc comment",
							filePath, fset.Position(ts.Pos()).Line, name)
					}
				}
			}
		}
	}

	for name, found := range requiredTypes {
		if !found {
			t.Errorf("exported type %s not found in package", name)
		}
	}
}

func firstLine(s string) string {
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		return s[:idx]
	}
	return s
}
