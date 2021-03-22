package main

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"text/template"

	"github.com/davecgh/go-spew/spew"
)

// conains the necessary informatino to build a Svelte script
type SvelteBuild struct {
	Name      string
	Path      string // route path as in form of url
	SvelteLoc string // location where the script lies
	BuildLoc  string // location where svelte route will be built

	BuildScriptLoc string // location with build script lies
	EntryPointLoc  string // location where esbuild script lies
}

var buildScriptTmpl = `const sveltePreprocess = require('svelte-preprocess');
const esbuildSvelte = require('esbuild-svelte');
const esbuild = require('esbuild');

esbuild
  .build({
    entryPoints: ['{{.EntryPointLoc}}'],
    bundle: true,
    outdir: 'backend/public/build',
    plugins: [
      esbuildSvelte({
        preprocess: sveltePreprocess(),
      }),
    ],
  })
  .catch(() => process.exit(1));`

var esBuildEntryTmpl = `import {{.Name}} from '../{{.SvelteLoc}}';
const app = new App({
  target: document.body,
  props: {},
});
export default app;`

// build looks for svelte files in frontend/src/routes and compiles them
func build() error {
	// list all the routes and build them into there own javascript
	buildObjs := []SvelteBuild{}
	routePath := filepath.Clean("frontend/src/routes")

	// check if path exists
	if _, err := os.Stat(routePath); os.IsNotExist(err) {
		return fmt.Errorf("Could not find frontend/src/routes folder are you in the proper directory?")
	}
	// get the list of routes and append them to are string slice
	err := filepath.Walk(routePath, func(path string, info fs.FileInfo, err error) error {
		if !info.IsDir() {
			fmt.Println("path: ", path)
			pName := getNameOfPath(path, routePath)
			underscoreName := strings.ToLower(strings.ReplaceAll(pName, "/", "_")) + ".js"
			buildObjs = append(buildObjs, SvelteBuild{
				Name:           getNameFromPath(path),
				SvelteLoc:      path,
				BuildScriptLoc: filepath.Clean("tmp/build_" + underscoreName + ".js"),
				EntryPointLoc:  filepath.Clean("tmp/" + underscoreName),
				BuildLoc:       "backend/public/build/" + underscoreName + ".js",
			})
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("received error while walking path: %v", err)
	}

	err = os.Mkdir("tmp", 0770)
	if err != nil && !strings.Contains(err.Error(), "file exists") {
		return fmt.Errorf("Error creating tmp build directory: %v", err)
	}
	wg := sync.WaitGroup{}
	// compile each route and place in respective directory in public/js
	for i := range buildObjs {
		wg.Add(1)
		go func(b SvelteBuild) {
			fmt.Println("Building Svelte Object:")
			spew.Dump(b)
			err := compileSvelte(b)
			if err != nil {
				fmt.Printf("error compiling svelte build object:\n\tname: %s\n\tmessage: %v", b.Name, err)
			}
			wg.Done()
		}(buildObjs[i])
	}
	wg.Wait()
	err = os.RemoveAll("tmp")
	if err != nil {
		return fmt.Errorf("Error removing tmp build directory: %v", err)
	}
	return nil
}

// compileSvelte compiles the svelte build object
func compileSvelte(sb SvelteBuild) error {
	// write build scripts to tmp/ folder
	err := createScriptFile(sb.BuildScriptLoc, buildScriptTmpl, sb)
	if err != nil {
		return err
	}
	err = createScriptFile(sb.EntryPointLoc, esBuildEntryTmpl, sb)
	if err != nil {
		return err
	}

	// exec build script
	cmd := exec.Command("node", sb.BuildScriptLoc)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Error running build script, output:\n%s\nerror:\n%v", string(output), err)
	}

	if len(output) > 0 {
		fmt.Printf("Build Output: %s\n", string(output))
	}
	return nil
}

func createScriptFile(fileName, templateString string, sb SvelteBuild) error {
	tempFile, err := os.OpenFile(fileName, os.O_CREATE|os.O_RDWR, 0770)
	defer tempFile.Close()
	if err != nil {
		return fmt.Errorf("Error creating script file name: %s\nerror: %v", fileName, err)
	}
	scriptTemplate, err := template.New(fileName).Parse(templateString)
	if err != nil {
		return fmt.Errorf("Error parsing template name: %s\nerror: %v", fileName, err)
	}
	err = scriptTemplate.Execute(tempFile, sb)
	if err != nil {
		return fmt.Errorf("Error executing template name: %s\nerror: %v", fileName, err)
	}
	return nil
}

// getNameOfPath removes pwd path and .svelte to return the logical part of the path
func getNameOfPath(s string, pwd string) string {
	return strings.Split(
		strings.Replace(s, pwd+"/", "", 1),
		".",
	)[0]
}

// getNameFromPath returns the name of the svelte object
func getNameFromPath(s string) string {
	return strings.Split(
		filepath.Base(s),
		".",
	)[0]
}
