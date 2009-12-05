/*
# Makefile for go.go

all: install

include $(GOROOT)/src/Make.$(GOARCH)

TARG    = go
GOFILES = $(TARG).go

include $(GOROOT)/src/Make.cmd
*/

package main

import (
	"os";
	"fmt";
	"flag";
	"io";
	"go/parser";
	"go/ast";
	"strconv";
	"path";
)

var (
	wd, _	= os.Getwd();
	debug	= flag.Bool("d", false, "Debug mode");
	verbose	= flag.Bool("v", false, "Verbose mode");
	bindir	= flag.String("b", os.Getenv("GOBIN")+"/", "Go binaries directory");
	arch	= flag.String("a", "6", "Architecture type 5=arm 6=amd64 8=x86");
)

func chk(e os.Error) {
	if e != nil {
		fmt.Fprintf(os.Stderr, "Can't %s\n", e);
		os.Exit(1);
	}
}

func exec(args []string, dir string) (returnCode int) {
	p, e := os.ForkExec(args[0], args, os.Environ(), dir, nil);
	if *debug {
		fmt.Printf("exec pid=%v\terr=%v\tdir=%v\tcmd=%v\n", p, e, dir, args)
	}
	chk(e);
	m, e := os.Wait(p, 0);
	chk(e);
	returnCode = int(m.WaitStatus);
	return;
}

func getLocalImports(filename string) (imports map[string]bool, error os.Error) {
	source, error := io.ReadFile(filename);
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
	sourceSet := make(map[string]int);
	error = collectSourceFiles(sourcePath, sourceTable, sourceSet);
	return;
}

func compile(target string) {
	dir, filename := path.Split(target);
	dir = path.Join(wd, dir);
	returnCode := exec([]string{*bindir + *arch + "g", filename + ".go"}, dir);
	if returnCode != 0 {
		fmt.Printf("Error compiling %v\n", filename+".go")
	}
}

func main() {
	flag.Parse();
	args := flag.Args();
	if len(args) == 0 {
		flag.Usage();
		os.Exit(1);
	}

	target := args[0];
	files, error := CollectSourceFiles(target);
	if error != nil {
		fmt.Printf("Can't %v\n", error);
		os.Exit(1);
	}

	for k := len(files) - 1; k >= 0; k-- {
		v := files[k];
		if v != "" {
			compile(v)
		} else {
			files[k] = "", false
		}
	}

	targets := make([]string, len(files)+3);
	targets[0] = *bindir + *arch + "l";
	targets[1] = "-o";
	targets[2] = target;
	i := 3;
	for _, v := range files {
		targets[i] = v + "." + *arch;
		i++;
	}
	returnCode := exec(targets, "");
	if returnCode != 0 {
		fmt.Printf("Error linking %v %v\n", target, files)
	}
	os.Exec(wd+"/"+target, args, os.Environ());
	fmt.Printf("Error running %v %v\n", target, args);
}
