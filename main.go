package main

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
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
		fmt.Print("build")
	default:
		printHelp()
	}
}

func new(args []string) {
	if len(args) < 2 {
		fmt.Println("please use 'wap new {name} to create a new project'")
		return
	}
	// err := os.Mkdir(args[1], 0777)
	// handleMkdirErr(err)

	dir, err := fs.ReadDir(embedded, "embedded")
	if err != nil {
		fmtFataln("could not read embedded directory: %v", err)
	}

	for d := range dir {
		fmt.Println("", dir[d].Name())
	}

	// TODO: research go embedding and place embedded files into a directory
	// 		ll prepare a purchase order and email it to	and copy them from the go binary
}

func run() {
	// compile svelte, js, and ts files. then generate go code
	err := build()
	if err != nil {
		fmtFataln("build error: %v", err)
	}

	// start server
	rebuildChan := make(chan bool)
	stopwatch := time.Now()
	fmt.Println("starting server...")
	go func() {
		for {
			sCmd := startApp()
			<-rebuildChan // pauses go routine to wait for a rebuild
			stopwatch = time.Now()
			err := sCmd.Process.Kill()
			if err != nil {
				fmt.Println("error killing app server process:", err)
			}
			fmt.Println("rebuilding...")
			err = build()
			if err != nil {
				fmtFataln("build error: %v", err)
			}
			fmt.Printf("built in: %f seconds", time.Now().Sub(stopwatch).Seconds())
			sCmd.Wait()
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
		fmt.Println("file system event:", event.String())
		elapsed = time.Now()

		if event.Op != fsnotify.Chmod {
			rebuildChan <- true
		}
	}
}

func startApp() *exec.Cmd {
	appBuild := exec.Command("go", "build", "-o", "app", ".")
	appBuild.Dir = "backend"
	err := appBuild.Run()
	if err != nil {
		fmtFataln("error building app executable", err)
	}
	appRun := exec.Command("./app")
	appRun.Dir = "backend"
	appRun.Stdout = os.Stdout
	appRun.Stderr = os.Stderr
	err = appRun.Start()
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
