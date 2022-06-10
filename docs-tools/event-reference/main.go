package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/ast/astutil"
)

func main() {
	s := token.NewFileSet()
	filepath.Walk(path.Join("..", ".."), func(pth string, i fs.FileInfo, err error) error {
		if strings.HasSuffix(i.Name(), ".go") {
			f, err := parser.ParseFile(s, pth, nil, 0)
			for _, d := range f.Decls {
				astutil.Apply(d, func(c *astutil.Cursor) bool {
					if m, ok := c.Node().(*ast.CompositeLit); ok {
						if e, ok := m.Type.(*ast.SelectorExpr); ok && e.Sel.Name == "Metadata" {
							if v, ok := e.X.(*ast.Ident); ok && strings.HasSuffix(v.Name, "events") {
								fmt.Printf("this is a Metadata: %+v\n", m)
							}
						}
					}
					return true
				}, nil)
			}
			if err != nil {
				// TODO: Replace with proper logger call
				fmt.Fprintf(os.Stderr, "error parsing Go source files: %v", err)
				os.Exit(1)
			}
		}
		return nil
	})
}
