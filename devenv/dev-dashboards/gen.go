//go:build ignore
// +build ignore

package main

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"
	"text/template"

	dev_dashboards "github.com/grafana/grafana/devenv/dev-dashboards"
	"github.com/grafana/grafana/pkg/codegen"
)

var (
	OUTPUT_PATH = "gen.libsonnet"
	EXCLUDE     = map[string]struct{}{
		"jsonnetfile.json":      {},
		"jsonnetfile.lock.json": {},
		"panel-library.json":    {}, // TODO: remove panel-library once importing issue is fixed
	}
)

//go:embed tmpl/*.tmpl
var tmplFS embed.FS

func main() {
	g := newGen()
	out, err := g.generate()
	if err != nil {
		panic(err)
	}
	wd := codegen.NewWriteDiffer()
	wd[OUTPUT_PATH] = []byte(out)

	if _, set := os.LookupEnv("CODEGEN_VERIFY"); set {
		err = wd.Verify()
		if err != nil {
			fmt.Fprintf(os.Stderr, "generated code is not up to date:\n%s\nrun `make gen-jsonnet` to regenerate\n\n", err)
			os.Exit(1)
		}
	} else {
		err = wd.Write()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error while writing generated code to disk:\n%s\n", err)
			os.Exit(1)
		}
	}
}

type devDashboard struct {
	Name string
	Path string
}

type libjsonnetGen struct {
	templates  *template.Template
	dashboards []devDashboard
}

func newGen() *libjsonnetGen {
	tmpls := template.New("codegen")
	tmpls = template.Must(tmpls.ParseFS(tmplFS, "tmpl/*.tmpl"))
	return &libjsonnetGen{templates: tmpls}
}

func (g *libjsonnetGen) generate() (string, error) {
	buf := new(bytes.Buffer)

	if err := g.readDir("."); err != nil {
		return "", err
	}

	sort.Slice(g.dashboards, func(i, j int) bool {
		return g.dashboards[i].Name < g.dashboards[j].Name
	})

	vars := struct {
		Dashboards []devDashboard
	}{g.dashboards}

	if err := g.templates.Lookup("gen.libsonnet.tmpl").Execute(buf, vars); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func (g *libjsonnetGen) readDir(dir string) error {
	files, err := dev_dashboards.DevDashboardFS.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, f := range files {
		if _, skip := EXCLUDE[f.Name()]; skip {
			continue
		}

		if f.IsDir() {
			if err := g.readDir(path.Join(dir, f.Name())); err != nil {
				return err
			}
			continue
		}

		name := strings.TrimSuffix(f.Name(), ".json")
		if len(name) > 40 {
			name = name[:40]
		}

		g.dashboards = append(g.dashboards, devDashboard{
			Path: path.Join(dir, f.Name()),
			Name: name,
		})
	}
	return nil
}
