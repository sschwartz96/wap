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

	"github.com/fsnotify/fsnotify"
)

//go:embed embedded/backend
//go:embed embedded/frontend
//go:embed embedded/README.md
//go:embed embedded/package.json

var embedded embed.FS

func main() {
	// check pre-reqs
	checkGoVersion()

	args := os.Args
	if len(args) == 0 {
		printHelp()
		return
	}

	flags := parseFlags(os.Args[1:])

	switch flags.cmd {
	case "new":
		new(flags)
	case "run":
		run(flags)
	case "build":
		build()
	default:
		printHelp()
	}
}

// checkGoVersion checks for minimum minor go version of > 16
// exits program with error message
func checkGoVersion() {
	goCheckCmd := exec.Command("go", "version")
	goCheckOut, err := goCheckCmd.Output()
	if err != nil {
		fmtFataln("error getting command output:", err)
	}
	minor, err := strconv.Atoi(strings.Split(string(goCheckOut), ".")[1])
	if err != nil {
		fmtFatalf("error finding go minor version: %v\n", err)
	}
	if minor < 16 {
		fmtFatalf("error go minor version %d, need at least 1.16\n", minor)
	}
}

func new(flags *cmdFlags) {
	if len(flags.cmd) == 0 {
		fmt.Println("please use 'wap new {name} to create a new project'")
		return
	}

	// create directory
	err := os.Mkdir(flags.name, 0770)
	if err != nil {
		if strings.Contains(err.Error(), "file exists") {
			fmtFatalf("directory with the name %s already exists\n", flags.name)
		} else {
			handleMkdirErr(err)
		}
	}

	fs.WalkDir(embedded, "embedded", func(embedPath string, d fs.DirEntry, err error) error {
		// don't need to copy the embedded directory itself
		if embedPath == "embedded" || d == nil {
			return nil
		}

		newPath := flags.name + "/" + strings.Replace(embedPath, "embedded/", "", 1)
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

func run(flags *cmdFlags) {
	devFlag := flags.getValue("dev") != nil

	// compile svelte, js, and ts files. then generate go code
	pages, err := compile(true, devFlag)
	if err != nil {
		fmtFataln("build error: %v", err)
	}

	// start websocket server
	refreshChan := make(chan bool)
	if devFlag {
		go func() {
			fmt.Println("starting websocket server")
			wsServer := newWebsocketServer()
			go func() {
				err := wsServer.start()
				if err != nil {
					fmt.Println("error starting websocket server:", err)
				}
			}()
			for {
				<-refreshChan
				wsServer.wsConnHandler.sendUpdateMsg()
			}
		}()
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
				if devFlag {
					refreshChan <- true
				}
				continue
			}

			// build both backend
			err := sCmd.Process.Kill()
			if err != nil {
				fmt.Println("error killing app server process:", err)
			}
			fmt.Println("rebuilding backend...")
			err = generateGoCode(true, devFlag, pages)
			if err != nil {
				fmtFataln("error generating go code: %v", err)
			}
			execName := buildApp("./")
			sCmd = startApp(execName)
			fmt.Printf("built backend in: %f seconds\n", time.Now().Sub(stopwatch).Seconds())
			if devFlag {
				time.Sleep(time.Millisecond * 1000) // wait a little for the app to start
				refreshChan <- true
			}
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
		// put fw.Events in a separate go routine and allow events to pass through
		// until preivous build is complete. Another channel???
		if time.Now().Sub(elapsed) < time.Second {
			continue
		}
		elapsed = time.Now()

		if event.Op != fsnotify.Chmod {

			switch event.Op {
			// new file or directory has been created
			case fsnotify.Create:
				fmt.Println("file created! ", event.Name, " | ", event.Op)
				// add the new directory to the file watcher
				if fileInfo, _ := os.Stat(event.Name); fileInfo.IsDir() {
					fw.Add(event.Name)
				}

				// if a file in the routes dir created then we must recompile backend
				if strings.HasPrefix(event.Name, "frontend/src/routes") {
					buildOnlyFrontendChan <- true
					buildOnlyFrontendChan <- false
					break
				}
			case fsnotify.Remove:
				fmt.Println("file removed! ", event.Name, " | ", event.Op)

				if strings.HasSuffix(event.Name, ".go") {
					// TODO: think about sane way to handle deletion of go files?
					// messes up the hot refresh for some reason?
					// also ran when GoImports is used, which causes undefined behavior
					fmt.Println("is go file, skipping")
				} else {
					os.RemoveAll("backend/public/build")
					buildOnlyFrontendChan <- true
					buildOnlyFrontendChan <- false
					break
				}
				fallthrough
			default:
				fmt.Println("default action! ", event.Name, " | ", event.Op)
				// determines whether or not to compile frontend or backend
				buildOnlyFrontendChan <- strings.HasPrefix(event.Name, "frontend/")
			}
		}
	}
}

func build() {
	elapsed := time.Now()
	_, err := compile(false, false)
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
	appBuild.Stderr = os.Stderr
	appBuild.Stdout = os.Stdout
	appBuild.Dir = "backend"
	err := appBuild.Run()
	if err != nil {
		fmtFataln("error building app executable:", err)
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

func handleMkdirErr(err error) {
	if err != nil {
		if strings.Contains(err.Error(), "already exists.") {
			fmtFataln("error: directory already exists")
		}
		fmtFataln("error creating directory: %v", err)
	}
}

func printHelp() {
	wap := "'wap' usage:"
	new := "\n\tnew:\tcreate new wap program\n "
	run := "\trun:\tcompiles and runs wap program on specified port\n "
	build := "\tbuild:\tbuilds the program into a single executable in the out folder\n "
	fmt.Println(wap + new + run + build)
}

func fmtFataln(msg string, a ...interface{}) {
	fmt.Printf(msg+"\n", a...)
	os.Exit(1)
}

func fmtFatalf(msg string, a ...interface{}) {
	fmt.Printf(msg, a...)
	os.Exit(1)
}

func parseFlags(args []string) *cmdFlags {
	cmdFlags := &cmdFlags{
		args:  args,
		flags: map[string]*string{},
	}
	for i := range args {
		value := ""
		arg := args[i]

		if strings.HasPrefix(arg, "--") {
			arg = strings.TrimPrefix(arg, "--")
		} else if strings.HasPrefix(arg, "-") {
			arg = strings.TrimPrefix(arg, "-")
			if strings.Contains(arg, "=") {
				split := strings.Split(arg, "=")
				arg = split[0]
				value = split[1]
			} else if i+1 < len(args) {
				value = args[i]
				i++
			}
		} else if cmdFlags.cmd == "" {
			cmdFlags.cmd = arg
		} else {
			cmdFlags.name = arg
		}

		cmdFlags.flags[arg] = &value
	}
	return cmdFlags
}

type cmdFlags struct {
	args  []string
	flags map[string]*string // use string pointer to signify if the value does not exist
	cmd   string             // the command used not a flag
	name  string             // the name used to create a new project or build a binary
}

func (f *cmdFlags) getValue(arg string) *string {
	return f.flags[arg]
}
