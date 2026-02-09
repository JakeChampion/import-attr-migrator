.PHONY: build test clean install

BINARY := migrate
MODULE := github.com/user/import-attr-migrator

build:
	go build -o $(BINARY) ./cmd/migrate

test:
	go test -v ./...

clean:
	rm -f $(BINARY)

install:
	go install ./cmd/migrate

# Run against test fixtures
demo: build
	./$(BINARY) ./testdata/sample.ts

# Dump tree for debugging
dump: build
	./$(BINARY) -dump ./testdata/sample.ts

# Dry run against test fixtures
dry-run: build
	./$(BINARY) -dry-run ./testdata/
