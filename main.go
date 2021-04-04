package main

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/fsnotify/fsnotify"
)

//go:embed embedded/backend
//go:embed embedded/frontend
//go:embed embedded/README.md
//go:embed embedded/package.json

var embedded embed.FS

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		printHelp()
		return
	}

	switch args[0] {
	case "new":
		new(args)
	case "run":
		run()
	case "build":
		build()
	default:
		printHelp()
	}
}

func new(args []string) {
	if len(args) < 2 {
		fmt.Println("please use 'wap new {name} to create a new project'")
		return
	}

	// check go version
	goCheckCmd := exec.Command("go", "version")
	goCheckOut, err := goCheckCmd.Output()
	if err != nil {
		fmtFataln("error getting command output:", err)
	}
	minor, err := strconv.Atoi(strings.Split(string(goCheckOut), ".")[1])
	if err != nil {
		os.RemoveAll(args[1])
		fmtFatalf("error finding go minor version: %v\n", err)
	}
	if minor < 16 {
		os.RemoveAll(args[1])
		fmtFatalf("error go minor version %d, need at least 1.16\n", minor)
	}

	// create directory
	err = os.Mkdir(args[1], 0770)
	if err != nil {
		if strings.Contains(err.Error(), "file exists") {
			fmtFatalf("directory with the name %s already exists\n", args[1])
		} else {
			handleMkdirErr(err)
		}
	}

	fs.WalkDir(embedded, "embedded", func(embedPath string, d fs.DirEntry, err error) error {
		// don't need to copy the embedded directory itself
		if embedPath == "embedded" || d == nil {
			return nil
		}

		newPath := args[1] + "/" + strings.Replace(embedPath, "embedded/", "", 1)
		if d.IsDir() {
			err := os.Mkdir(newPath, 0770)
			if err != nil {
				fmt.Println("error creating directory:", err)
			}
			return nil
		}
		// is a file:
		dst, err := os.Create(newPath)
		if err != nil {
			fmt.Printf("could not create file: %s\n\t with error: %v\n", newPath, err)
			return nil
		}
		src, err := embedded.Open(embedPath)
		if err != nil {
			fmt.Printf("could not open embedded file: %s\n\t with error: %v\n", newPath, err)
			return nil
		}
		_, err = io.Copy(dst, src)
		if err != nil {
			fmt.Printf("error copying embed file: %s\n\tto destination: %s\n\terror:%v\n", embedPath, newPath, err)
			return nil
		}
		return nil
	})

	fmt.Println("Success!")
	fmt.Println("  Please install javascript dependencies in root folder with \"npm install\"")
	fmt.Println("  and initialize backend with \"go mod init\" followed by \"go mod tidy\"")
}

func run() {
	// compile svelte, js, and ts files. then generate go code
	pages, err := compile(true)
	if err != nil {
		fmtFataln("build error: %v", err)
	}

	// start server
	buildOnlyFrontendChan := make(chan bool)
	stopwatch := time.Now()
	go func() {
		fmt.Println("starting server...")
		execName := buildApp("./")
		sCmd := startApp(execName)
		for {
			buildOnlyFrontend := <-buildOnlyFrontendChan // pauses go routine to wait for a rebuild
			stopwatch = time.Now()

			if buildOnlyFrontend {
				fmt.Println("rebuilding frontend...")
				pages, err = compileFrontEnd()
				if err != nil {
					fmt.Println("error rebuilding only frontend:", err)
				}
				fmt.Printf("built frontend in: %f seconds\n", time.Now().Sub(stopwatch).Seconds())
				continue
			}

			// build both backend
			err := sCmd.Process.Kill()
			if err != nil {
				fmt.Println("error killing app server process:", err)
			}
			fmt.Println("rebuilding backend...")
			err = generateGoCode(true, pages)
			if err != nil {
				fmtFataln("error generating go code: %v", err)
			}
			execName := buildApp("./")
			sCmd = startApp(execName)
			fmt.Printf("built backend in: %f seconds\n", time.Now().Sub(stopwatch).Seconds())
		}
	}()

	// file watching to recompile
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		fmtFataln("error creating file watcher:", err)
	}
	filepath.WalkDir("./", func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			fw.Add(path)
		}
		return nil
	})

	elapsed := time.Now()
	for {
		event := <-fw.Events
		// TODO: revisit this when build times exceed a second???
		if time.Now().Sub(elapsed) < time.Second {
			continue
		}
		elapsed = time.Now()

		if event.Op != fsnotify.Chmod {
			spew.Println("event path:", event.Name)
			if event.Op == fsnotify.Create && strings.HasPrefix(event.Name, "frontend/src/routes") {
				buildOnlyFrontendChan <- true
				buildOnlyFrontendChan <- false
				continue
			}
			buildOnlyFrontendChan <- strings.HasPrefix(event.Name, "frontend/")
		}
	}
}

func build() {
	elapsed := time.Now()
	_, err := compile(false)
	if err != nil {
		fmtFataln("error compiling:", err)
	}
	execName := buildApp("../")
	fmt.Printf("built executable named: %s in %f seconds\n", execName, time.Now().Sub(elapsed).Seconds())
}

// buildApp builds the go code into single binary
// locPath is the location path where to store it
// returns the name of the executable. Needs to have .exe for windows
func buildApp(locPath string) string {
	buildName := "app"
	if runtime.GOOS == "windows" {
		buildName = "app.exe"
	}
	appBuild := exec.Command("go", "build", "-o", locPath+buildName, ".")
	appBuild.Dir = "backend"
	err := appBuild.Run()
	if err != nil {
		fmtFataln("error building app executable", err)
	}
	return buildName
}

func startApp(execName string) *exec.Cmd {
	appRun := exec.Command("./" + execName)
	appRun.Dir = "backend"
	appRun.Stdout = os.Stdout
	appRun.Stderr = os.Stderr
	err := appRun.Start()
	if err != nil {
		fmtFataln("error starting go server:", err)
	}
	return appRun
}

func copyDir(f fs.FS, src, dest string) error {
	err := os.Mkdir(dest, 0770)
	if err != nil {
		return err
	}
	return fs.WalkDir(f, src, func(path string, d fs.DirEntry, err error) error {
		newPath := strings.Replace(path, src, dest, 1)
		if d.IsDir() {
			err := os.Mkdir(newPath, 0770)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func handleMkdirErr(err error) {
	if err != nil {
		if strings.Contains(err.Error(), "already exists.") {
			fmtFataln("error: directory already exists")
		}
		fmtFataln("error creating directory: %v", err)
	}
}

func printHelp() {
	fmt.Println("'wap' usage:\n\tnew:\tcreate new wap program\n \trun:\tcompiles and runs wap program on specified port\n \tbuild:\tbuilds the program into a single executable in the out folder\n ")
}

func fmtFataln(msg string, a ...interface{}) {
	fmt.Printf(msg+"\n", a...)
	os.Exit(1)
}

func fmtFatalf(msg string, a ...interface{}) {
	fmt.Printf(msg, a...)
	os.Exit(1)
}
