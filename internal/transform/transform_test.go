package transform

import (
	"strings"
	"testing"
)

func TestMigrateAssertToWith_StaticImport(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		lang     Language
		want     string
		wantN    int
	}{
		{
			name:  "basic import assertion",
			input: `import data from './data.json' assert { type: 'json' };`,
			lang:  JavaScript,
			want:  `import data from './data.json' with { type: 'json' };`,
			wantN: 1,
		},
		{
			name:  "named import assertion",
			input: `import { foo } from './data.json' assert { type: 'json' };`,
			lang:  JavaScript,
			want:  `import { foo } from './data.json' with { type: 'json' };`,
			wantN: 1,
		},
		{
			name:  "namespace import assertion",
			input: `import * as data from './data.json' assert { type: 'json' };`,
			lang:  JavaScript,
			want:  `import * as data from './data.json' with { type: 'json' };`,
			wantN: 1,
		},
		{
			name:  "export re-export with assertion",
			input: `export { default } from './data.json' assert { type: 'json' };`,
			lang:  JavaScript,
			want:  `export { default } from './data.json' with { type: 'json' };`,
			wantN: 1,
		},
		{
			name:  "already using with (no change)",
			input: `import data from './data.json' with { type: 'json' };`,
			lang:  JavaScript,
			want:  `import data from './data.json' with { type: 'json' };`,
			wantN: 0,
		},
		{
			name:  "no assertion clause at all",
			input: `import data from './data.json';`,
			lang:  JavaScript,
			want:  `import data from './data.json';`,
			wantN: 0,
		},
		{
			name: "multiple imports in one file",
			input: strings.Join([]string{
				`import a from './a.json' assert { type: 'json' };`,
				`import b from './b.css' assert { type: 'css' };`,
				`import c from './c.js';`,
			}, "\n"),
			lang: JavaScript,
			want: strings.Join([]string{
				`import a from './a.json' with { type: 'json' };`,
				`import b from './b.css' with { type: 'css' };`,
				`import c from './c.js';`,
			}, "\n"),
			wantN: 2,
		},
		{
			name:  "TypeScript import assertion",
			input: `import data from './data.json' assert { type: 'json' };`,
			lang:  TypeScript,
			want:  `import data from './data.json' with { type: 'json' };`,
			wantN: 1,
		},
		{
			name:  "TSX import assertion",
			input: `import data from './data.json' assert { type: 'json' };`,
			lang:  TSX,
			want:  `import data from './data.json' with { type: 'json' };`,
			wantN: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := MigrateAssertToWith([]byte(tt.input), tt.lang)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got := string(result.Output)
			if got != tt.want {
				t.Errorf("output mismatch:\n  got:  %q\n  want: %q", got, tt.want)
			}

			if result.Replacements != tt.wantN {
				t.Errorf("replacement count: got %d, want %d", result.Replacements, tt.wantN)
			}
		})
	}
}

func TestMigrateAssertToWith_PreservesFormatting(t *testing.T) {
	input := `// This is a comment
import data from './data.json' assert {
  type: 'json'
};

// Another import
import css from './styles.css' assert {
  type: 'css'
};
`
	want := `// This is a comment
import data from './data.json' with {
  type: 'json'
};

// Another import
import css from './styles.css' with {
  type: 'css'
};
`

	result, err := MigrateAssertToWith([]byte(input), JavaScript)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(result.Output) != want {
		t.Errorf("formatting not preserved:\n  got:\n%s\n  want:\n%s", result.Output, want)
	}
}

func TestMigrateAssertToWith_DynamicImport(t *testing.T) {
	tests := []struct {
		name  string
		input string
		lang  Language
		want  string
		wantN int
	}{
		{
			name:  "dynamic import with assert",
			input: `const data = await import('./data.json', { assert: { type: 'json' } });`,
			lang:  JavaScript,
			want:  `const data = await import('./data.json', { with: { type: 'json' } });`,
			wantN: 1,
		},
		{
			name:  "dynamic import already using with",
			input: `const data = await import('./data.json', { with: { type: 'json' } });`,
			lang:  JavaScript,
			want:  `const data = await import('./data.json', { with: { type: 'json' } });`,
			wantN: 0,
		},
		{
			name:  "dynamic import without options",
			input: `const data = await import('./data.json');`,
			lang:  JavaScript,
			want:  `const data = await import('./data.json');`,
			wantN: 0,
		},
		{
			name:  "dynamic import TypeScript",
			input: `const data = await import('./data.json', { assert: { type: 'json' } });`,
			lang:  TypeScript,
			want:  `const data = await import('./data.json', { with: { type: 'json' } });`,
			wantN: 1,
		},
		{
			name: "mixed static and dynamic",
			input: strings.Join([]string{
				`import config from './config.json' assert { type: 'json' };`,
				`const data = await import('./data.json', { assert: { type: 'json' } });`,
			}, "\n"),
			lang: JavaScript,
			want: strings.Join([]string{
				`import config from './config.json' with { type: 'json' };`,
				`const data = await import('./data.json', { with: { type: 'json' } });`,
			}, "\n"),
			wantN: 2,
		},
		{
			name:  "assert in non-import context is not changed",
			input: `console.assert(true, 'should be true');`,
			lang:  JavaScript,
			want:  `console.assert(true, 'should be true');`,
			wantN: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := MigrateAssertToWith([]byte(tt.input), tt.lang)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got := string(result.Output)
			if got != tt.want {
				t.Errorf("output mismatch:\n  got:  %q\n  want: %q", got, tt.want)
			}

			if result.Replacements != tt.wantN {
				t.Errorf("replacement count: got %d, want %d", result.Replacements, tt.wantN)
			}
		})
	}
}

func TestMigrateAssertToWith_Comprehensive(t *testing.T) {
	input := `import { createRequire } from 'node:module';
import data from './data.json' assert { type: 'json' };
import systemOfADown from './system;of;a;down.json' assert { type: 'json' };
import { default as config } from './config.json'assert{type: 'json'};
import { thing } from "./data.json"assert{type: 'json'};
import { fileURLToPath } from 'node:url' invalid { };
const require = createRequire(import.meta.url);
const foo = require('./foo.ts');

const data2 = await import('./data2.json', {
	assert: { type: 'json' },
});

await import('foo-bis');
`
	want := `import { createRequire } from 'node:module';
import data from './data.json' with { type: 'json' };
import systemOfADown from './system;of;a;down.json' with { type: 'json' };
import { default as config } from './config.json'with{type: 'json'};
import { thing } from "./data.json"with{type: 'json'};
import { fileURLToPath } from 'node:url' invalid { };
const require = createRequire(import.meta.url);
const foo = require('./foo.ts');

const data2 = await import('./data2.json', {
	with: { type: 'json' },
});

await import('foo-bis');
`

	result, err := MigrateAssertToWith([]byte(input), JavaScript)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Replacements != 5 {
		t.Errorf("replacement count: got %d, want 5", result.Replacements)
	}

	got := string(result.Output)
	if got != want {
		t.Errorf("output mismatch:\n  got:\n%s\n  want:\n%s", got, want)
	}
}

func TestDumpTree(t *testing.T) {
	input := `import data from './data.json' assert { type: 'json' };`
	sexp, err := DumpTree([]byte(input), JavaScript)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Logf("S-expression:\n%s", sexp)

	// Sanity check: should contain import_statement
	if !strings.Contains(sexp, "import_statement") {
		t.Errorf("S-expression should contain import_statement, got: %s", sexp)
	}
}
