package main

import (
	"flag"
	"fmt"
	"os"

	"tigo/internal/ui"
	"tigo/pkg/db"
)

// The value of `version` is supposed to be set using `-ldflags` during build time. If not set, it defaults to "dev".
// `go build -ldflags="-X main.version=$(git rev-parse --short HEAD)" ./cmd/tigo` will set the `version` variable to the current git commit hash.
var version = "dev"

func main() {
	helpFlag := flag.Bool("h", false, "Show the help")
	helpLongFlag := flag.Bool("help", false, "Show the help")
	versionFlag := flag.Bool("v", false, "Show the version")
	versionLongFlag := flag.Bool("version", false, "Show the version")

	flag.Usage = func() {
		fmt.Println("tigo [root]")
		fmt.Println("\nBy default tigo looks for a `.tigo` directory in the current working directory")
		fmt.Println("and use that as the root directory of tasks. If `.tigo` does not exist, it will")
		fmt.Println("use `$HOME/.local/share/tigo`.")
		fmt.Println("\n    -h --help       Show the help")
		fmt.Println("    -v --version    Show the version")
	}

	flag.Parse()

	if *helpFlag || *helpLongFlag {
		flag.Usage()
		os.Exit(0)
	}

	if *versionFlag || *versionLongFlag {
		fmt.Printf("tigo-%s\n", version)
		os.Exit(0)
	}

	rootPath := flag.Arg(0)
	if rootPath == "" {
		rootPath = db.ResolveRoot()
	}

	err := db.Init(rootPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing database: %v\n", err)
		os.Exit(1)
	}

	err = ui.Run(rootPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running UI: %v\n", err)
		os.Exit(1)
	}
}
