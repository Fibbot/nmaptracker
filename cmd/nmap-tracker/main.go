package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sloppy/nmaptracker/internal/db"
	"github.com/sloppy/nmaptracker/internal/export"
	"github.com/sloppy/nmaptracker/internal/importer"
	"github.com/sloppy/nmaptracker/internal/scope"
	"github.com/sloppy/nmaptracker/internal/web"
)

const defaultDBPath = "nmap-tracker.db"

func usage() string {
	return "Usage: nmap-tracker <serve|import|export|projects>"
}

func main() {
	os.Exit(run(os.Args, os.Stdout, os.Stderr))
}

func run(args []string, out, errOut io.Writer) int {
	if len(args) < 2 {
		fmt.Fprintln(out, usage())
		return 1
	}

	command := strings.ToLower(args[1])
	switch command {
	case "serve":
		return runServe(args[2:], out, errOut)
	case "projects":
		return runProjects(args[2:], out, errOut)
	case "import":
		return runImport(args[2:], out, errOut)
	case "export":
		return runExport(args[2:], out, errOut)
	case "help", "-h", "--help":
		fmt.Fprintln(out, usage())
		return 0
	default:
		fmt.Fprintf(errOut, "unknown command: %s\n", command)
		fmt.Fprintln(out, usage())
		return 1
	}
}

func runServe(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(errOut)
	dbPath := fs.String("db", defaultDBPath, "path to database file")
	port := fs.Int("port", 8080, "port to listen on")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	database, err := db.Open(*dbPath)
	if err != nil {
		fmt.Fprintf(errOut, "open db: %v\n", err)
		return 1
	}
	defer database.Close()

	server := web.NewServer(database)
	addr := fmt.Sprintf(":%d", *port)
	fmt.Fprintf(out, "listening on http://localhost:%d\n", *port)
	if err := http.ListenAndServe(addr, server.Handler()); err != nil {
		fmt.Fprintf(errOut, "serve: %v\n", err)
		return 1
	}
	return 0
}

func runProjects(args []string, out, errOut io.Writer) int {
	dbPath, remaining, err := extractFlag(args, "db", defaultDBPath)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	if len(remaining) < 1 {
		fmt.Fprintln(errOut, "projects command requires subcommand: list|create <name>")
		return 1
	}
	sub := remaining[0]
	switch sub {
	case "list":
		database, err := db.Open(dbPath)
		if err != nil {
			fmt.Fprintf(errOut, "open db: %v\n", err)
			return 1
		}
		defer database.Close()
		projects, err := database.ListProjects()
		if err != nil {
			fmt.Fprintf(errOut, "list projects: %v\n", err)
			return 1
		}
		for _, p := range projects {
			fmt.Fprintf(out, "%d\t%s\n", p.ID, p.Name)
		}
		return 0
	case "create":
		if len(remaining) < 2 {
			fmt.Fprintln(errOut, "projects create requires a project name")
			return 1
		}
		name := strings.Join(remaining[1:], " ")
		database, err := db.Open(dbPath)
		if err != nil {
			fmt.Fprintf(errOut, "open db: %v\n", err)
			return 1
		}
		defer database.Close()
		p, err := database.CreateProject(name)
		if err != nil {
			fmt.Fprintf(errOut, "create project: %v\n", err)
			return 1
		}
		fmt.Fprintf(out, "created project %d\t%s\n", p.ID, p.Name)
		return 0
	default:
		fmt.Fprintf(errOut, "unknown projects subcommand: %s\n", sub)
		return 1
	}
}

func runImport(args []string, out, errOut io.Writer) int {
	dbPath, remaining, err := extractFlag(args, "db", defaultDBPath)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	projectName, remaining, err := extractFlag(remaining, "project", "")
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	if projectName == "" {
		fmt.Fprintln(errOut, "import requires --project")
		return 1
	}
	if len(remaining) < 1 {
		fmt.Fprintln(errOut, "import requires an nmap XML file path")
		return 1
	}
	filePath := remaining[0]
	if !filepath.IsAbs(filePath) {
		if abs, err := filepath.Abs(filePath); err == nil {
			filePath = abs
		}
	}

	database, err := db.Open(dbPath)
	if err != nil {
		fmt.Fprintf(errOut, "open db: %v\n", err)
		return 1
	}
	defer database.Close()

	project, found, err := database.GetProjectByName(projectName)
	if err != nil {
		fmt.Fprintf(errOut, "find project: %v\n", err)
		return 1
	}
	if !found {
		fmt.Fprintf(errOut, "project %q not found; create it first via projects create\n", projectName)
		return 1
	}

	obs, err := importer.ParseXMLFile(filePath)
	if err != nil {
		fmt.Fprintf(errOut, "parse xml: %v\n", err)
		return 1
	}

	matcher, err := scope.NewMatcher(nil)
	if err != nil {
		fmt.Fprintf(errOut, "scope matcher: %v\n", err)
		return 1
	}

	if _, err := importer.ImportObservations(database, matcher, project.ID, filepath.Base(filePath), obs, time.Now().UTC()); err != nil {
		fmt.Fprintf(errOut, "import: %v\n", err)
		return 1
	}
	fmt.Fprintf(out, "imported %s into project %s\n", filepath.Base(filePath), project.Name)
	return 0
}

func runExport(args []string, out, errOut io.Writer) int {
	dbPath, remaining, err := extractFlag(args, "db", defaultDBPath)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	projectName, remaining, err := extractFlag(remaining, "project", "")
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	format, remaining, err := extractFlag(remaining, "format", "json")
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	outputPath, remaining, err := extractFlag(remaining, "o", "")
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	if outputPath == "" {
		outputPath, remaining, err = extractFlag(remaining, "output", "")
		if err != nil {
			fmt.Fprintln(errOut, err)
			return 1
		}
	}
	if projectName == "" {
		fmt.Fprintln(errOut, "export requires --project")
		return 1
	}
	if outputPath == "" {
		fmt.Fprintln(errOut, "export requires --output or -o")
		return 1
	}
	if len(remaining) > 0 {
		fmt.Fprintf(errOut, "unexpected arguments: %s\n", strings.Join(remaining, " "))
		return 1
	}

	database, err := db.Open(dbPath)
	if err != nil {
		fmt.Fprintf(errOut, "open db: %v\n", err)
		return 1
	}
	defer database.Close()

	project, found, err := database.GetProjectByName(projectName)
	if err != nil {
		fmt.Fprintf(errOut, "find project: %v\n", err)
		return 1
	}
	if !found {
		fmt.Fprintf(errOut, "project %q not found; create it first via projects create\n", projectName)
		return 1
	}

	file, err := os.Create(outputPath)
	if err != nil {
		fmt.Fprintf(errOut, "create output: %v\n", err)
		return 1
	}
	defer file.Close()

	switch strings.ToLower(format) {
	case "json":
		if err := export.ExportProjectJSON(database, project.ID, file); err != nil {
			fmt.Fprintf(errOut, "export json: %v\n", err)
			return 1
		}
	case "csv":
		if err := export.ExportProjectCSV(database, project.ID, file); err != nil {
			fmt.Fprintf(errOut, "export csv: %v\n", err)
			return 1
		}
	default:
		fmt.Fprintf(errOut, "unknown export format: %s\n", format)
		return 1
	}

	fmt.Fprintf(out, "exported %s (%s)\n", outputPath, strings.ToLower(format))
	return 0
}

// extractFlag finds a string flag (e.g., --db value) anywhere in args and returns its value and remaining args.
func extractFlag(args []string, name string, defaultVal string) (string, []string, error) {
	val := defaultVal
	var remaining []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--"+name || arg == "-"+name {
			if i+1 >= len(args) {
				return "", nil, fmt.Errorf("%s flag requires a value", arg)
			}
			val = args[i+1]
			i++
			continue
		}
		remaining = append(remaining, arg)
	}
	return val, remaining, nil
}
