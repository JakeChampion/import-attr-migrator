# import-attr-migrator

A Go CLI tool that migrates legacy JavaScript/TypeScript **import assertions** (`assert { type: 'json' }`) to **import attributes** (`with { type: 'json' }`), using [tree-sitter](https://tree-sitter.github.io/) for accurate, formatting-preserving source transformation.

## Background

The TC39 [Import Attributes](https://github.com/tc39/proposal-import-attributes) proposal (Stage 4) replaced the earlier [Import Assertions](https://github.com/nicolo-ribaudo/tc39-proposal-import-assertions) syntax. The keyword changed from `assert` to `with`:

```diff
- import data from './data.json' assert { type: 'json' };
+ import data from './data.json' with { type: 'json' };
```

Node.js, Deno, and bundlers are migrating to the new syntax. This tool automates the codemod.

## Install

```bash
# Requires Go 1.22+ and CGO enabled (tree-sitter uses C parsers)
go install github.com/JakeChampion/import-attr-migrator/cmd/migrate@latest
```

Or build from source:

```bash
git clone https://github.com/JakeChampion/import-attr-migrator
cd import-attr-migrator
go build -o migrate ./cmd/migrate
```

> **Note:** tree-sitter's Go bindings require CGO. Make sure you have a C compiler installed (`gcc` or `clang`).

## Usage

```bash
# Preview which files would change (dry run)
migrate -dry-run ./src

# Rewrite files in-place
migrate -w ./src

# Print migrated output to stdout (single file)
migrate ./src/config.ts

# Debug: dump the tree-sitter S-expression for a file
migrate -dump ./src/config.ts

# Custom extensions
migrate -w -ext ".js,.mjs" ./src
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-w` | `false` | Write changes back to source files |
| `-dry-run` | `false` | Show which files would change without modifying them |
| `-ext` | `.js,.jsx,.ts,.tsx,.mjs,.mts` | Comma-separated file extensions to process |
| `-dump` | `false` | Dump S-expression tree for the first file and exit |
| `-recursive` | `true` | Recurse into directories |

### Skipped directories

The tool automatically skips `node_modules`, `vendor`, `dist`, `build`, and hidden directories (starting with `.`).

## Library usage

The `transform` package can be imported directly for use in other Go programs:

```bash
go get github.com/JakeChampion/import-attr-migrator
```

```go
package main

import (
	"fmt"
	"log"

	"github.com/JakeChampion/import-attr-migrator/transform"
)

func main() {
	source := []byte(`import data from './data.json' assert { type: 'json' };`)

	result, err := transform.MigrateAssertToWith(source, transform.JavaScript)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%d replacement(s)\n", result.Replacements)
	fmt.Println(string(result.Output))
	// Output:
	// 1 replacement(s)
	// import data from './data.json' with { type: 'json' };
}
```

Supported languages: `transform.JavaScript`, `transform.TypeScript`, `transform.TSX`.

## How it works

1. Parses each file using the appropriate tree-sitter grammar (JavaScript, TypeScript, or TSX)
2. Walks the concrete syntax tree (CST) to locate anonymous `assert` tokens that are children of `import_attribute` nodes
3. Records the byte ranges of those tokens
4. Applies surgical byte-range replacements (`assert` → `with`), preserving all formatting, comments, and whitespace

Because it operates on the CST rather than regex, it won't accidentally replace `assert` in other contexts (variable names, function calls, test assertions, etc.).

## What it handles

- ✅ Static imports: `import x from '...' assert { ... }`
- ✅ Named imports: `import { x } from '...' assert { ... }`
- ✅ Namespace imports: `import * as x from '...' assert { ... }`
- ✅ Re-exports: `export { x } from '...' assert { ... }`
- ✅ Multiple imports per file
- ✅ Files already using `with` (left unchanged)
- ✅ Mixed `assert` and `with` in the same file
- ✅ `.js`, `.jsx`, `.ts`, `.tsx`, `.mjs`, `.mts` files

## Caveats

- **Dynamic import()**: The `import()` syntax with assertion options (`import('./foo.json', { assert: { type: 'json' } })`) uses a different AST structure. The tool attempts to handle it, but this path depends heavily on grammar version. Run `-dump` to verify the tree structure if you use dynamic imports with assertions.
- **Grammar versions**: The exact node types produced by tree-sitter-javascript/typescript depend on the grammar version in your `go.sum`. If the grammar doesn't produce `import_attribute` nodes for your syntax, the tool will silently produce zero replacements. Use `-dump` to debug.

## Development

```bash
# Run tests
go test ./...

# Run tests with verbose output (see S-expression dumps)
go test -v ./transform/

# Build
go build -o migrate ./cmd/migrate

# Test against sample data
./migrate ./testdata/sample.ts
```

## License

MIT
