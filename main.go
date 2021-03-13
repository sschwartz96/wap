package main

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"strings"
)

//go:embed embedded/*
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
	// 			and copy them from the go binary
}

func run() {
	// compile svelte, js, and ts files
	// generate go code for endpoint
	// file watching to recompile
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
	fmt.Println("'wap' usage:\n\n \tnew:\tcreate new wap program\n \trun:\tcompiles and runs wap program on specified port\n \tbuild:\tbuilds the program into a single executable in the out folder\n ")
}

func fmtFataln(msg string, a ...interface{}) {
	fmt.Printf(msg+"\n", a...)
	os.Exit(1)
}
