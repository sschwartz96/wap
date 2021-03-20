package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// conains the necessary informatino to build a Svelte script
type SvelteBuild struct {
	Name          string
	SvelteLoc     string // location where the script lies
	EntryPointLoc string // this needs to be written ./tmp/*
	BuildLoc      string // location where svelte route will be built
}

var buildScriptTmpl = `const sveltePreprocess = require('svelte-preprocess');
const esbuildSvelte = require('esbuild-svelte');
const esbuild = require('esbuild');

esbuild
  .build({
    entryPoints: ['{{.EntryPoint}}'],
    bundle: true,
    outdir: './public/build',
    plugins: [
      esbuildSvelte({
        preprocess: sveltePreprocess(),
      }),
    ],
  })
  .catch(() => process.exit(1));`

var esBuildEntryTmpl = `import {{.Name}} from '{{.SvelteLoc}}';
const app = new App({
  target: document.body,
  props: {},
});
export default app;`

// outdir: ./public/build

func build() error {
	// list all the routes and build them into there own javascript
	routes := []string{}
	routePath := "frontend\\src\\routes\\"
	// check if path exists
	if _, err := os.Stat(routePath); os.IsNotExist(err) {
		return fmt.Errorf("could not find frontend/src/routes folder are you in the proper directory?")
	}
	// get the list of routes and append them to are string slice
	err := filepath.Walk(routePath, func(path string, info fs.FileInfo, err error) error {
		if !info.IsDir() {
			routes = append(routes, strings.Replace(path, routePath, "", 1))
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("received error while walking path: %v", err)
	}
	fmt.Println("routes: ", routes)
	return nil
}
