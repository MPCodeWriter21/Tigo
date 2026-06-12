package main

import (
	"flag"
	"fmt"
	"os"

	"tigo/internal/config"
	"tigo/internal/ui"
	"tigo/pkg/db"
)

// The value of `version` is supposed to be set using `-ldflags` during build time. If not set, it defaults to "dev".
// `go build -ldflags="-X main.version=$(git rev-parse --short HEAD)" ./cmd/tigo` will set the `version` variable to the current git commit hash.
var version = "0.2.0-dev"

func main() {
	helpFlag := flag.Bool("h", false, "Show the help")
	helpLongFlag := flag.Bool("help", false, "Show the help")
	versionFlag := flag.Bool("v", false, "Show the version")
	versionLongFlag := flag.Bool("version", false, "Show the version")
	defaultConfigFlag := flag.Bool("default-config", false, "Print the default configuration")
	userConfigPathFlag := flag.Bool("user-config-path", false, "Print the path to the user configuration")

	flag.Usage = func() {
		fmt.Println("tigo [root]")
		fmt.Println()
		fmt.Println("By default tigo looks for a `.tigo` directory in the current working directory")
		fmt.Println("and use that as the root directory of tasks. If `.tigo` does not exist, it will")
		fmt.Println("use `$HOME/.local/share/tigo`.")
		fmt.Println()
		fmt.Println("    -h --help             Show this help and exit")
		fmt.Println("    -v --version          Show the version and exit")
		fmt.Println("    --default-config      Print the default configuration and exit")
		fmt.Println("    --user-config-path    Print the path to the user configuration and exit")
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

	if *defaultConfigFlag {
		fmt.Println("# This is the default configuration for Tigo. You can copy this to your `config.yaml` and modify it as needed.")
		fmt.Println()
		fmt.Println("sort_by: id              # Sort tasks by id, priority, due-date, or title")
		fmt.Println("default_priority: 50     # The default priority for new tasks")
		fmt.Println("frame_style: round       # The style of the frames (round, double, single)")
		fmt.Println("show_completed: false    # Whether to show completed tasks in the list by default")
		fmt.Println("due_color_enabled: false # Whether to color the due date based on how close it is")
		os.Exit(0)
	}

	if *userConfigPathFlag {
		userConfigPath, err := config.UserConfigPath()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting user config directory: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(userConfigPath)
		os.Exit(0)
	}

	rootPath := flag.Arg(0)
	if rootPath == "" {
		rootPath = db.ResolveRoot()
	}
	cfg, err := config.LoadConfig(rootPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	err = db.Init(rootPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing database: %v\n", err)
		os.Exit(1)
	}

	err = ui.Run(rootPath, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running UI: %v\n", err)
		os.Exit(1)
	}
}
