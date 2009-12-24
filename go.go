// Copyright 2009 Dimiter Stanev, malkia@gmail.com. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"os"
	"fmt"
	"io/ioutil"
	"go/parser"
	"go/ast"
	"strconv"
	"path"
)

func getmap(m map[string]string, k string) (v string) {
	v, ok := m[k]
	if !ok {
		v = ""
	}
	return
}

var (
	curdir, _ = os.Getwd()
	envbin    = os.Getenv("GOBIN")
	arch      = getmap(map[string]string{"amd64": "6", "386": "8", "arm": "5"}, os.Getenv("GOARCH"))
)

func exec(args []string, dir string) {
	p, error := os.ForkExec(args[0], args, os.Environ(), dir, []*os.File{os.Stdin, os.Stdout, os.Stderr})
	if error != nil {
		fmt.Fprintf( os.Stderr, "Can't %s\n", error );
		os.Exit(1);
	}
	m, error := os.Wait(p, 0)
	if error != nil {
		fmt.Fprintf( os.Stderr, "Can't %s\n", error );
		os.Exit(1);
	}
	if m.WaitStatus != 0 {
		os.Exit(int(m.WaitStatus));
	}
}

func getLocalImports(filename string) (imports map[string]bool, error os.Error) {
	source, error := ioutil.ReadFile(filename)
	if error != nil {
		return
	}
	file, error := parser.ParseFile(filename, source, parser.ImportsOnly)
	if error != nil {
		return
	}
	for _, importDecl := range file.Decls {
		importDecl, ok := importDecl.(*ast.GenDecl)
		if ok {
			for _, importSpec := range importDecl.Specs {
				importSpec, ok := importSpec.(*ast.ImportSpec)
				if ok {
					for _, importPath := range importSpec.Path {
						importPath, _ := strconv.Unquote(string(importPath.Value))
						if len(importPath) > 0 && importPath[0] == '.' {
							if imports == nil {
								imports = make(map[string]bool)
							}
							dir, _ := path.Split(filename)
							imports[path.Join(dir, path.Clean(importPath))] = true
						}
					}
				}
			}
		}
	}

	return
}

func collectSourceFiles(sourcePath string, sourceTable map[int]string, sourceSet map[string]int) (error os.Error) {
	sourcePath = path.Clean(sourcePath)

	if index, exists := sourceSet[sourcePath]; exists {
		sourceTable[index] = ""
		sourceSet[sourcePath] = 0, false
	}

	localImports, error := getLocalImports(sourcePath + ".go")
	if error != nil {
		return
	}

	index := len(sourceTable)
	sourceSet[sourcePath] = index
	sourceTable[index] = sourcePath

	for k, _ := range localImports {
		if error = collectSourceFiles(k, sourceTable, sourceSet); error != nil {
			break
		}
	}

	return
}

func CollectSourceFiles(sourcePath string) (sourceTable map[int]string, error os.Error) {
	sourceTable = make(map[int]string)
	return sourceTable, collectSourceFiles(sourcePath, sourceTable, make(map[string]int))
}

func shouldUpdate(sourceFile, targetFile string) (doUpdate bool, error os.Error) {
	sourceStat, error := os.Lstat(sourceFile)
	if error != nil {
		return false, error
	}
	targetStat, error := os.Lstat(targetFile)
	if error != nil {
		return true, error
	}
	return targetStat.Mtime_ns < sourceStat.Mtime_ns, error
}

func compile(target string) {
	dir, filename := path.Split(target)
	dir = path.Join(curdir, dir)
	source := path.Join(dir, filename+".go")
	object := path.Join(dir, filename+"."+arch)
	doUpdate, error := shouldUpdate(source, object)
	if doUpdate {
		exec([]string{path.Join(envbin, arch+"g"), filename + ".go"}, dir)
	} else if error != nil {
		fmt.Fprintln(os.Stderr, error)
	}
}

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "go main-program [arg0 [arg1 ...]]")
		os.Exit(1)
	}

	target := path.Clean(args[0])
	if path.Ext(target) == ".go" {
		target = target[0 : len(target)-3]
	}

	files, error := CollectSourceFiles(target)
	if error != nil {
		fmt.Fprintf(os.Stderr, "Can't %v\n", error)
		os.Exit(1)
	}

	// Compiling source files
	for k := len(files) - 1; k >= 0; k-- {
		v := files[k]
		if v != "" {
			compile(v)
		} else {
			files[k] = "", false
		}
	}

	targets := make([]string, len(files)+3)
	targets[0] = path.Join(envbin, arch+"l")
	targets[1] = "-o"
	targets[2] = target
	doLink := false
	for i, v := range files {
		targets[i+3] = v + "." + arch
		if !doLink {
			if shouldUpdate, _ := shouldUpdate(targets[i+3], target); shouldUpdate {
				doLink = true
			}
		}
	}
	if doLink {
		exec(targets, "")
	}
	os.Exec(path.Join(curdir, target), args, os.Environ())
	fmt.Fprintf(os.Stderr, "Error running %v\n", args)
	os.Exit(1)
}
