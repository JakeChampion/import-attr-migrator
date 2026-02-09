// Package transform provides tree-sitter based migration from
// legacy import assertions (`assert { type: 'json' }`) to
// import attributes (`with { type: 'json' }`).
//
// It operates on raw source bytes, preserving all formatting,
// comments, and whitespace by doing surgical byte-range replacements
// guided by the concrete syntax tree.
package transform

import (
	"fmt"
	"unsafe"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_javascript "github.com/tree-sitter/tree-sitter-javascript/bindings/go"
	tree_sitter_typescript "github.com/tree-sitter/tree-sitter-typescript/bindings/go"
)

// Language selects which tree-sitter grammar to use for parsing.
type Language int

const (
	JavaScript Language = iota
	TypeScript
	TSX
)

// Result holds the output of a migration.
type Result struct {
	// Output is the transformed source code.
	Output []byte
	// Replacements is the number of `assert` → `with` substitutions made.
	Replacements int
}

// MigrateAssertToWith rewrites all import assertion keywords in source
// from `assert` to `with`, returning the modified source and the
// number of replacements made.
//
// The function parses source using the specified Language grammar,
// walks the CST to find anonymous "assert" tokens that are children
// of import_attribute (or similar) nodes, and replaces them with "with".
//
// Both static imports/exports and dynamic import() are handled:
//
//	import data from './data.json' assert { type: 'json' }
//	export { default } from './data.json' assert { type: 'json' }
//	const data = await import('./data.json', { assert: { type: 'json' } })
func MigrateAssertToWith(source []byte, lang Language) (*Result, error) {
	tsLang, err := getLanguage(lang)
	if err != nil {
		return nil, err
	}

	parser := tree_sitter.NewParser()
	defer parser.Close()

	if err := parser.SetLanguage(tree_sitter.NewLanguage(tsLang)); err != nil {
		return nil, fmt.Errorf("setting language: %w", err)
	}

	tree := parser.Parse(source, nil)
	defer tree.Close()

	root := tree.RootNode()
	if root == nil {
		return nil, fmt.Errorf("parse returned nil root node")
	}

	// Collect byte ranges that need replacement.
	var replacements []replacement
	collectReplacements(root, source, &replacements)

	// Build output with replacements applied.
	output := applyReplacements(source, replacements)

	return &Result{
		Output:       output,
		Replacements: len(replacements),
	}, nil
}

// DumpTree returns the S-expression representation of the parsed source.
// Useful for debugging which node types the grammar produces for your code.
func DumpTree(source []byte, lang Language) (string, error) {
	tsLang, err := getLanguage(lang)
	if err != nil {
		return "", err
	}

	parser := tree_sitter.NewParser()
	defer parser.Close()

	if err := parser.SetLanguage(tree_sitter.NewLanguage(tsLang)); err != nil {
		return "", fmt.Errorf("setting language: %w", err)
	}

	tree := parser.Parse(source, nil)
	defer tree.Close()

	root := tree.RootNode()
	if root == nil {
		return "", fmt.Errorf("parse returned nil root node")
	}

	return root.ToSexp(), nil
}

// replacement records a byte range to be replaced with "with".
type replacement struct {
	start uint
	end   uint
}

// collectReplacements walks the CST and finds all "assert" tokens
// that appear in import/export attribute positions.
func collectReplacements(node *tree_sitter.Node, source []byte, out *[]replacement) {
	if node == nil {
		return
	}

	kind := node.Kind()

	// Strategy 1: Look for anonymous "assert" token inside import_attribute
	// or import_assertion nodes. Some grammar versions produce:
	//   (import_statement
	//     source: (string)
	//     (import_attribute "assert" (object ...)))
	if !node.IsNamed() && kind == "assert" {
		parent := node.Parent()
		if parent != nil && isImportAttributeNode(parent.Kind()) {
			*out = append(*out, replacement{
				start: uint(node.StartByte()),
				end:   uint(node.EndByte()),
			})
			return
		}
	}

	// Strategy 2: The grammar may not recognize "assert" as a keyword and
	// instead produce an ERROR node. Two sub-cases:
	//
	// 2a: ERROR inside import_statement/export_statement - the ERROR's
	//     first child is an identifier "assert":
	//       (import_statement ... (ERROR (identifier "assert") "{" ...))
	//
	// 2b: For re-exports the entire statement may be an ERROR at the
	//     top level, containing export_clause, string, then identifier "assert":
	//       (ERROR "export" (export_clause) "from" (string) (identifier "assert") ...)
	if kind == "ERROR" {
		parent := node.Parent()
		if parent != nil && isImportOrExportStatement(parent.Kind()) {
			// Case 2a
			firstChild := node.Child(0)
			if firstChild != nil && firstChild.Kind() == "identifier" {
				text := nodeText(firstChild, source)
				if text == "assert" {
					*out = append(*out, replacement{
						start: uint(firstChild.StartByte()),
						end:   uint(firstChild.EndByte()),
					})
					return
				}
			}
		}
		// Case 2b: top-level ERROR containing export/import structure
		if hasExportOrImportChild(node) {
			for i := uint(0); i < uint(node.ChildCount()); i++ {
				child := node.Child(i)
				if child.Kind() == "identifier" && nodeText(child, source) == "assert" {
					// Verify it follows a string node (the source path)
					if i > 0 {
						prev := node.Child(i - 1)
						if prev != nil && prev.Kind() == "string" {
							*out = append(*out, replacement{
								start: uint(child.StartByte()),
								end:   uint(child.EndByte()),
							})
							return
						}
					}
				}
			}
		}
	}

	// Strategy 3: For dynamic import(), the assertion might appear as
	// a property name `assert` inside the options object:
	//   import('./foo.json', { assert: { type: 'json' } })
	if node.IsNamed() && isPropertyIdentifier(kind) {
		text := nodeText(node, source)
		if text == "assert" && isInsideDynamicImportOptions(node) {
			*out = append(*out, replacement{
				start: uint(node.StartByte()),
				end:   uint(node.EndByte()),
			})
			return
		}
	}

	// Recurse into children.
	count := node.ChildCount()
	for i := uint(0); i < uint(count); i++ {
		child := node.Child(uint(i))
		collectReplacements(child, source, out)
	}
}

// hasExportOrImportChild returns true if the ERROR node contains an
// export_clause, import_clause, or an anonymous "export"/"import" token,
// indicating it's a malformed import/export statement.
func hasExportOrImportChild(node *tree_sitter.Node) bool {
	for i := uint(0); i < uint(node.ChildCount()); i++ {
		child := node.Child(i)
		kind := child.Kind()
		switch kind {
		case "export_clause", "import_clause", "export", "import":
			return true
		}
	}
	return false
}

// isImportAttributeNode returns true if the node kind represents an
// import attribute/assertion clause. Different grammar versions may
// use different names.
func isImportAttributeNode(kind string) bool {
	switch kind {
	case "import_attribute", "import_assertion", "assert_clause":
		return true
	}
	return false
}

// isImportOrExportStatement returns true if the node kind is an
// import or export statement.
func isImportOrExportStatement(kind string) bool {
	switch kind {
	case "import_statement", "export_statement":
		return true
	}
	return false
}

// isPropertyIdentifier returns true if the node kind is a property name.
func isPropertyIdentifier(kind string) bool {
	switch kind {
	case "property_identifier", "shorthand_property_identifier", "identifier":
		return true
	}
	return false
}

// isInsideDynamicImportOptions walks up the tree to check if this
// property identifier is inside a dynamic import()'s options argument.
func isInsideDynamicImportOptions(node *tree_sitter.Node) bool {
	// Walk up looking for: pair -> object -> arguments -> call_expression
	// where the call_expression's function is "import".
	current := node.Parent()
	depth := 0
	for current != nil && depth < 6 {
		kind := current.Kind()
		if kind == "call_expression" || kind == "import" {
			// Check if the function being called is "import"
			fn := current.ChildByFieldName("function")
			if fn != nil && fn.Kind() == "import" {
				return true
			}
			// Some grammars represent dynamic import differently
			firstChild := current.Child(0)
			if firstChild != nil && firstChild.Kind() == "import" {
				return true
			}
			return false
		}
		current = current.Parent()
		depth++
	}
	return false
}

// nodeText extracts the source text for a node.
func nodeText(node *tree_sitter.Node, source []byte) string {
	start := node.StartByte()
	end := node.EndByte()
	if uint(start) >= uint(len(source)) || uint(end) > uint(len(source)) {
		return ""
	}
	return string(source[start:end])
}

// applyReplacements applies all collected replacements to the source,
// producing a new byte slice. Replacements must be non-overlapping
// and are applied in order of their start position.
func applyReplacements(source []byte, replacements []replacement) []byte {
	if len(replacements) == 0 {
		return append([]byte(nil), source...) // Return a copy
	}

	// Estimate capacity: original size + (len("with") - len("assert")) * count
	est := len(source) + (4-6)*len(replacements)
	if est < len(source) {
		est = len(source)
	}
	result := make([]byte, 0, est)

	lastOffset := uint(0)
	for _, r := range replacements {
		// Copy everything from last position to start of this replacement
		result = append(result, source[lastOffset:r.start]...)
		// Write "with" instead of "assert"
		result = append(result, "with"...)
		lastOffset = r.end
	}

	// Copy the remainder
	result = append(result, source[lastOffset:]...)
	return result
}

// getLanguage returns the unsafe.Pointer to the tree-sitter language.
func getLanguage(lang Language) (unsafe.Pointer, error) {
	switch lang {
	case JavaScript:
		return tree_sitter_javascript.Language(), nil
	case TypeScript:
		return tree_sitter_typescript.LanguageTypescript(), nil
	case TSX:
		return tree_sitter_typescript.LanguageTSX(), nil
	default:
		return nil, fmt.Errorf("unsupported language: %d", lang)
	}
}
