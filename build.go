package main

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
)

// contains the necessary informatino to build a Svelte script
type Page struct {
	Title   string
	URLPath string // route path as in form of url
	JS      string // route path to built javascript file
	CSS     string // route path to built css file
	// TODO: need to add description & author for html meta data?

	// build properties
	SvelteLoc      string // location where the script lies
	BuildLoc       string // location where svelte route will be built
	BuildScriptLoc string // location with build script lies
	EntryPointLoc  string // location where esbuild script lies
}

// contains the information for build
type BuildInfo struct {
	Run   bool // run or bool
	Pages []Page
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

var esBuildEntryTmpl = `import {{ .Title }} from '../{{ .SvelteLoc }}';
const app = new {{ .Title }}({
  target: document.body,
  props: {},
});
export default app;`

// compile looks for svelte files in frontend/src/routes and compiles them
// then generates appropriate go code to run the server
func compile(run bool) ([]Page, error) {
	// compile frontend
	pages, err := compileFrontEnd()
	if err != nil {
		return nil, err
	}

	// generate go code
	err = generateGoCode(run, pages)
	if err != nil {
		return nil, err
	}

	return pages, nil
}

func compileFrontEnd() ([]Page, error) {
	// go ahead and remove temp directory
	os.RemoveAll("tmp")

	// list all the routes and build them into their own javascript
	pages := []Page{}
	routePath := "frontend/src/routes"

	// check if path exists
	if _, err := os.Stat(routePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("Could not find frontend/src/routes folder are you in the proper directory?")
	}
	// get the list of routes and append them to are string slice
	err := filepath.Walk(routePath, func(path string, info fs.FileInfo, err error) error {
		if !info.IsDir() {
			path = strings.ReplaceAll(path, "\\", "/") // have to do this for windows :(
			pName := getNameOfPath(path, routePath)
			underscoreName := strings.ToLower(strings.ReplaceAll(pName, "/", "_"))
			fmt.Println("pname:", pName)
			pages = append(pages, Page{
				Title:   filepath.Clean(getNameFromPath(path)),
				URLPath: "/" + strings.Replace(strings.ToLower(pName), "index", "", 1),
				JS:      "/public/build/" + underscoreName + ".js",
				CSS:     "/public/build/" + underscoreName + ".css",

				SvelteLoc:      path,
				BuildScriptLoc: "tmp/build_" + underscoreName + ".js",
				EntryPointLoc:  "tmp/" + underscoreName + ".js",
				BuildLoc:       "backend/public/build/" + underscoreName + ".js",
			})
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("received error while walking path: %v", err)
	}

	err = os.Mkdir("tmp", 0770)
	if err != nil && !strings.Contains(err.Error(), "file exists") {
		return nil, fmt.Errorf("Error creating tmp build directory: %v", err)
	}

	type compileInfo struct {
		Index int
		NoCSS bool
	}
	buildChan := make(chan compileInfo, len(pages))
	// compile each route and place in respective directory in public/js
	for i := range pages {
		go func(b Page, index int) {
			//fmt.Println("Building Svelte Object:")
			//spew.Dump(b)
			err := compileSvelte(b)
			if err != nil {
				fmt.Printf("error compiling svelte build object:\n\tname: %s\n\tmessage: %v", b.Title, err)
			}
			// need to check if css was generated
			if _, err = os.Stat("backend" + b.CSS); os.IsNotExist(err) {
				//fmt.Println("css location does not exist: ", "backend"+b.CSS)
				buildChan <- compileInfo{Index: index, NoCSS: true}
				return
			}
			buildChan <- compileInfo{Index: index, NoCSS: false}
		}(pages[i], i)
	}
	for j := 0; j < len(pages); j++ {
		select {
		case info := <-buildChan:
			if info.NoCSS {
				pages[info.Index].CSS = ""
			}
		}
	}
	err = os.RemoveAll("tmp")
	if err != nil {
		return nil, fmt.Errorf("Error removing tmp build directory: %v", err)
	}
	return pages, nil
}

// compileSvelte compiles the svelte build object
func compileSvelte(sb Page) error {
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

func createScriptFile(fileName, templateString string, sb Page) error {
	tempFile, err := os.OpenFile(fileName, os.O_CREATE|os.O_RDWR, 0770)
	defer func() {
		err := tempFile.Close()
		if err != nil {
			fmt.Printf("error closing temp file names: %s\nerror msg: %v\n", tempFile.Name(), err)
		}
	}()
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

func generateGoCode(run bool, pages []Page) error {
	wapGoPath := "./backend/wap_gen.go"
	tmplObj, err := template.New("wap_gen").Delims("[[", "]]").Parse(wapGenTemplate)
	if err != nil {
		return fmt.Errorf("error parsing wap_gen.go template text, error: %v", err)
	}
	os.Remove(wapGoPath)
	wapGenFile, err := os.Create(wapGoPath)
	if err != nil {
		return fmt.Errorf("error creating wap_go.go file, error: %v", err)
	}
	buildInfo := BuildInfo{
		Run:   run,
		Pages: pages,
	}
	err = tmplObj.Execute(wapGenFile, &buildInfo)
	if err != nil {
		return fmt.Errorf("error executing wap_go.go template, error: %v", err)
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
	return strings.ReplaceAll(strings.Split(filepath.Base(s), ".")[0], "\\", "/")
}

const wapGenTemplate = `// Code generated by wap; DO NOT EDIT.

package main

import (
	"embed"
	"io/fs"
	"html/template"
	"log"
	"net/http"
	"github.com/julienschmidt/httprouter"
)

//go:embed public/build
var embedded embed.FS

var (
	tmplObj *template.Template
)

type Page struct {
	Title       string
	URLPath     string // http path ex. "/login"
	CSS         string // css file location
	JS          string // js file location
}

type App struct {
	Pages []Page
}

var wapApp = &App{
	Pages: []Page{
		// list pages
		[[ range .Pages ]]
		{
			Title:   "[[ .Title ]]",
			URLPath: "[[ .URLPath ]]",
			CSS:     "[[ .CSS ]]",
			JS:      "[[ .JS ]]",
		},
		[[ end ]]
	},
}

func registerWAPGen(r *httprouter.Router) {
	var err error
	tmplObj, err = template.New("template.gohtml").Parse(htmlTemplate)

	if err != nil {
		log.Fatalf("Could not parse html template, error: %v", err)
	}
	for _, page := range wapApp.Pages {
		r.GET(page.URLPath, createHandler(page))
	}

	// serve files
	[[ if .Run ]]
		r.ServeFiles("/public/build/*filepath", http.Dir("./public/build"))
	[[ else ]]
		assetHandler := &AssetHandler{fs: embedded}
		r.ServeFiles("/public/build/*filepath", http.FS(assetHandler))
	[[ end ]]
}

// AssetHandler is used to load files at ./public/build/*
// implemnts fs.FS interface
type AssetHandler struct {
	fs embed.FS
}

// Open used to load files at ./public/build/*
// needed in order to correctly serve files with correct path
func (a *AssetHandler) Open(name string) (fs.File, error) {
	return a.fs.Open("public/build/" + name)
}

func createHandler(pageData Page) httprouter.Handle {
	return func(res http.ResponseWriter, req *http.Request, p httprouter.Params) {
		err := tmplObj.Execute(res, pageData)
		if err != nil {
			log.Fatalf("Error parsing template with page data: %v", err)
		}
	}
}

var htmlTemplate =` + "`" + `<html>
<head>
  <meta charset="utf-8">

  <title>{{ .Title }}</title>

  {{ if .CSS }}
	  <link rel="stylesheet" href="{{ .CSS }}">
  {{ end }}

  <script src="{{ .JS }}" defer></script>
</head>

<body>
</body>
 </html>` + "`"
