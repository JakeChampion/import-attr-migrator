// Command migrate rewrites JavaScript/TypeScript/TSX files to replace
// legacy import assertions (`assert { ... }`) with import attributes
// (`with { ... }`).
//
// Usage:
//
//	migrate [flags] <file|dir> [file|dir...]
//
// Flags:
//
//	-w          Write changes back to files (default: print to stdout)
//	-dry-run    Show which files would be changed without modifying them
//	-ext        Comma-separated file extensions to process (default: .js,.jsx,.ts,.tsx,.mjs,.mts)
//	-dump       Dump the S-expression tree for the first file and exit (debug)
//	-recursive  Recurse into directories (default: true)
package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/JakeChampion/import-attr-migrator/transform"
)

func main() {
	var (
		write     = flag.Bool("w", false, "write result back to source files")
		dryRun    = flag.Bool("dry-run", false, "show which files would change without modifying them")
		exts      = flag.String("ext", ".js,.jsx,.ts,.tsx,.mjs,.mts", "comma-separated file extensions to process")
		dump      = flag.Bool("dump", false, "dump S-expression tree for the first file and exit")
		recursive = flag.Bool("recursive", true, "recurse into directories")
	)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags] <file|dir> [file|dir...]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Migrate import assertions (assert) to import attributes (with).\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s -w ./src                 # Rewrite all files in src/\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -dry-run ./src           # Preview which files would change\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s ./src/foo.ts             # Print migrated file to stdout\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -dump ./src/foo.ts       # Show parsed S-expression tree\n", os.Args[0])
	}

	flag.Parse()

	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(1)
	}

	extSet := parseExtensions(*exts)

	// Dump mode: parse first file and print S-expression.
	if *dump {
		path := flag.Arg(0)
		source, err := os.ReadFile(path)
		if err != nil {
			fatalf("reading %s: %v", path, err)
		}
		lang := languageForFile(path)
		sexp, err := transform.DumpTree(source, lang)
		if err != nil {
			fatalf("parsing %s: %v", path, err)
		}
		fmt.Println(sexp)
		return
	}

	// Collect files to process.
	var files []string
	for _, arg := range flag.Args() {
		info, err := os.Stat(arg)
		if err != nil {
			fatalf("stat %s: %v", arg, err)
		}

		if info.IsDir() {
			dirFiles, err := collectFiles(arg, extSet, *recursive)
			if err != nil {
				fatalf("walking %s: %v", arg, err)
			}
			files = append(files, dirFiles...)
		} else {
			files = append(files, arg)
		}
	}

	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "no matching files found\n")
		os.Exit(0)
	}

	// Process each file.
	var (
		totalFiles        int
		totalReplacements int
	)

	for _, path := range files {
		source, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "WARN: skipping %s: %v\n", path, err)
			continue
		}

		lang := languageForFile(path)
		result, err := transform.MigrateAssertToWith(source, lang)
		if err != nil {
			fmt.Fprintf(os.Stderr, "WARN: skipping %s: %v\n", path, err)
			continue
		}

		if result.Replacements == 0 {
			continue
		}

		totalFiles++
		totalReplacements += result.Replacements

		if *dryRun {
			fmt.Printf("  %s (%d replacement(s))\n", path, result.Replacements)
			continue
		}

		if *write {
			// Preserve original file permissions.
			info, err := os.Stat(path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "WARN: stat %s: %v\n", path, err)
				continue
			}

			if err := os.WriteFile(path, result.Output, info.Mode()); err != nil {
				fmt.Fprintf(os.Stderr, "ERROR: writing %s: %v\n", path, err)
				continue
			}

			fmt.Printf("  ✓ %s (%d replacement(s))\n", path, result.Replacements)
		} else {
			// No -w flag: print to stdout (only useful for single files).
			os.Stdout.Write(result.Output)
		}
	}

	if *dryRun || *write {
		fmt.Fprintf(os.Stderr, "\n%d file(s) with %d total replacement(s)\n", totalFiles, totalReplacements)
	}
}

// collectFiles walks a directory and returns all files matching the extension set.
func collectFiles(root string, extSet map[string]bool, recursive bool) ([]string, error) {
	var files []string

	walkFn := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			// Skip hidden directories and node_modules.
			name := d.Name()
			if name != "." && strings.HasPrefix(name, ".") {
				return fs.SkipDir
			}
			if name == "node_modules" || name == "vendor" || name == "dist" || name == "build" {
				return fs.SkipDir
			}
			if !recursive && path != root {
				return fs.SkipDir
			}
			return nil
		}

		ext := filepath.Ext(path)
		if extSet[ext] {
			files = append(files, path)
		}
		return nil
	}

	if err := filepath.WalkDir(root, walkFn); err != nil {
		return nil, err
	}
	return files, nil
}

// languageForFile determines the tree-sitter Language based on file extension.
func languageForFile(path string) transform.Language {
	ext := filepath.Ext(path)
	switch ext {
	case ".ts", ".mts":
		return transform.TypeScript
	case ".tsx":
		return transform.TSX
	default:
		// .js, .jsx, .mjs — use JavaScript grammar.
		// JSX is a superset handled by the JS grammar.
		return transform.JavaScript
	}
}

// parseExtensions splits a comma-separated extension list into a set.
func parseExtensions(s string) map[string]bool {
	m := make(map[string]bool)
	for _, ext := range strings.Split(s, ",") {
		ext = strings.TrimSpace(ext)
		if ext == "" {
			continue
		}
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		m[ext] = true
	}
	return m
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}
