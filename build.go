package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// conains the necessary informatino to build a Svelte script
type SvelteBuild struct {
	Name          string
	SvelteLoc     string // location where the script lies
	BuildLoc      string // location where svelte route will be built
	EntryPointLoc string // location where esbuild script lies
}

var buildScriptTmpl = `const sveltePreprocess = require('svelte-preprocess');
const esbuildSvelte = require('esbuild-svelte');
const esbuild = require('esbuild');

esbuild
  .build({
    entryPoints: ['{{.EntryPointLoc}}'],
    bundle: true,
    outdir: '../backend/public/build',
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
	buildObjs := []SvelteBuild{}
	routePath := "frontend\\src\\routes\\"
	// check if path exists
	if _, err := os.Stat(routePath); os.IsNotExist(err) {
		return fmt.Errorf("could not find frontend/src/routes folder are you in the proper directory?")
	}
	// get the list of routes and append them to are string slice
	err := filepath.Walk(routePath, func(path string, info fs.FileInfo, err error) error {
		if !info.IsDir() {
			name := getNameOfPath(path, routePath)
			underscoreName := strings.ReplaceAll(name, "\\", "_") + ".js"
			buildObjs = append(buildObjs, SvelteBuild{
				Name:          name,
				SvelteLoc:     path,
				EntryPointLoc: "tmp\\esbuild_" + name + ".js",
				BuildLoc:      "backend\\js\\" + underscoreName + ".js",
			})
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("received error while walking path: %v", err)
	}

	err = os.Mkdir("tmp", 0770)
	if err != nil {
		return fmt.Errorf("Error creating tmp build directory: %v", err)
	}
	// compile each route and place in respective directory in public/js
	for i := range buildObjs {
		fmt.Println("got here")
		func(b SvelteBuild) {
			fmt.Println("building obj: ", b)
			err := compileSvelte(b)
			if err != nil {
				fmt.Printf("error compiling svelte build object:\n\tname: %s\n\tmessage: %v", b.Name, err)
			}
		}(buildObjs[i])
	}

	//err = os.RemoveAll("tmp")
	if err != nil {
		return fmt.Errorf("Error removing tmp build directory: %v", err)
	}
	return nil
}

// compileSvelte compiles the svelte build object
func compileSvelte(sb SvelteBuild) error {
	// write build script to tmp/ folder
	err := createScriptFile("tmp\\build_"+sb.Name+".js", buildScriptTmpl, sb)
	if err != nil {
		return err
	}
	err = createScriptFile(sb.EntryPointLoc, esBuildEntryTmpl, sb)
	if err != nil {
		return err
	}

	// exec build script

	// capture and return errors
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
	return strings.Replace(
		strings.Replace(s, pwd, "", 1),
		".svelte", "", 1,
	)
}
