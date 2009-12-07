// Copyright 2009 Dimiter Stanev, malkia@gmail.com. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"os";
	"fmt";
	"flag";
	"io/ioutil";
	"go/parser";
	"go/ast";
	"strconv";
	"path";
)

func getmap(m map[string]string, k string) (v string) {
	v, ok := m[k];
	if !ok {
		v = ""
	}
	return;
}

var (
	curdir, _	= os.Getwd();
	envbin		= os.Getenv("GOBIN");
	envarch		= os.Getenv("GOARCH");
	envos		= os.Getenv("GOOS");
	archmap		= map[string]string{"amd64": "6", "x86": "8", "arm": "5"};
	bindir		= &envbin;	//flag.String("b", envbin, "Go binaries directory");
	defarch		= getmap(archmap, envarch);
	arch		= &defarch;	//flag.String("a", getmap(archmap, envarch), "Architecture type 5=arm 6=amd64 8=x86");
	debug		= flag.Bool("d", false, "Debug mode");
)

func chk(e os.Error) {
	if e != nil {
		fmt.Fprintln(os.Stderr, e);
		os.Exit(1);
	}
}

func exec(args []string, dir string) int {
	p, e := os.ForkExec(args[0], args, os.Environ(), dir, []*os.File{os.Stdin, os.Stdout, os.Stderr});
	if *debug {
		fmt.Fprintf(os.Stderr, "exec pid=%v\terr=%v\tdir=%v\tcmd=%v\n", p, e, dir, args)
	}
	chk(e);
	m, e := os.Wait(p, 0);
	chk(e);
	return int(m.WaitStatus);
}

func getLocalImports(filename string) (imports map[string]bool, error os.Error) {
	source, error := ioutil.ReadFile(filename);
	if error != nil {
		return
	}

	file, error := parser.ParseFile(filename, source, parser.ImportsOnly);
	if error != nil {
		return
	}

	for _, importDecl := range file.Decls {
		importDecl, ok := importDecl.(*ast.GenDecl);
		if ok {
			for _, importSpec := range importDecl.Specs {
				importSpec, ok := importSpec.(*ast.ImportSpec);
				if ok {
					for _, importPath := range importSpec.Path {
						importPath, _ := strconv.Unquote(string(importPath.Value));
						if len(importPath) > 0 && importPath[0] == '.' {
							if imports == nil {
								imports = make(map[string]bool)
							}
							dir, _ := path.Split(filename);
							imports[path.Join(dir, path.Clean(importPath))] = true;
						}
					}
				}
			}
		}
	}

	return;
}

func collectSourceFiles(sourcePath string, sourceTable map[int]string, sourceSet map[string]int) (error os.Error) {
	sourcePath = path.Clean(sourcePath);

	if index, exists := sourceSet[sourcePath]; exists {
		sourceTable[index] = "";
		sourceSet[sourcePath] = 0, false;
	}

	localImports, error := getLocalImports(sourcePath + ".go");
	if error != nil {
		return
	}

	index := len(sourceTable);
	sourceSet[sourcePath] = index;
	sourceTable[index] = sourcePath;

	for k, _ := range localImports {
		if error = collectSourceFiles(k, sourceTable, sourceSet); error != nil {
			break
		}
	}

	return;
}

func CollectSourceFiles(sourcePath string) (sourceTable map[int]string, error os.Error) {
	sourceTable = make(map[int]string);
	return sourceTable, collectSourceFiles(sourcePath, sourceTable, make(map[string]int));
}

func shouldUpdate(sourceFile, targetFile string) (doUpdate bool, error os.Error) {
	sourceStat, error := os.Lstat(sourceFile);
	if error != nil {
		return false, error
	}
	targetStat, error := os.Lstat(targetFile);
	if error != nil {
		return true, error
	}
	return targetStat.Mtime_ns < sourceStat.Mtime_ns, error;
}

func compile(target string) {
	dir, filename := path.Split(target);
	dir = path.Join(curdir, dir);
	source := path.Join(dir, filename+".go");
	object := path.Join(dir, filename+"."+*arch);
	doUpdate, error := shouldUpdate(source, object);
	if doUpdate {
		returnCode := exec([]string{path.Join(*bindir, *arch+"g"), filename + ".go"}, dir);
		if returnCode != 0 {
			fmt.Fprintf(os.Stderr, "Error compiling %v\n", filename+".go")
		}
	} else if error != nil {
		fmt.Fprintln(os.Stderr, error)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "go main-program [arg0 [arg1 ...]]\n\nOptions:");
	flag.PrintDefaults();
	fmt.Fprintf(os.Stderr, "\nExamples:\n  go test arg1 arg2 arg3 -- This would compile test.go and all other local source files it refers to and then call it with arg1 arg2 arg3\n\n");
}

func main() {
	flag.Usage = usage;

	flag.Parse();
	args := flag.Args();
	if len(args) == 0 {
		flag.Usage();
		os.Exit(1);
	}

	target := args[0];
	files, error := CollectSourceFiles(target);
	if error != nil {
		fmt.Fprintf(os.Stderr, "Can't %v\n", error);
		os.Exit(1);
	}

	// Compiling source files
	for k := len(files) - 1; k >= 0; k-- {
		v := files[k];
		if v != "" {
			compile(v)
		} else {
			files[k] = "", false
		}
	}

	//	Linking produced objects into executable
	//	TODO: On Windows this should add ".exe" to the target
	targets := make([]string, len(files)+3);
	targets[0] = path.Join(*bindir, *arch+"l");
	targets[1] = "-o";
	targets[2] = target;

	doLink := false;
	i := 3;
	for _, v := range files {
		targets[i] = v + "." + *arch;
		if !doLink {
			if shouldUpdate, _ := shouldUpdate(targets[i], target); shouldUpdate {
				doLink = true
			}
		}
		i++;
	}
	if doLink {
		returnCode := exec(targets, "");
		if returnCode != 0 {
			fmt.Fprintf(os.Stderr, "Error linking %v %v\n", target, files)
		}
	}

	os.Exec(path.Join(curdir, target), args, os.Environ());
	fmt.Fprintf(os.Stderr, "Error running %v\n", args);
}
