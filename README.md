# Firefly Assignment — Word Frequency Assay

This small Go application downloads a large list of English words, processes a set of input essays, and prints the top most frequent words found across them. It’s wired to be run with Taskfile for a smooth developer experience.

## Prerequisites
- Go 1.21+ (earlier versions may work but are not tested)
- Task (the Taskfile runner from https://taskfile.dev)
  - macOS (Homebrew): `brew install go-task`
  - Linux (Homebrew): `brew install go-task`
  - Other: see official install options: https://taskfile.dev/installation/

## Quick start (using Taskfile)
From the project root (`firefly-assignment`):

1) Install dependencies and vendor them

```bash
task install
```

2) Run the application

```bash
task run_app
```

This will run `go run` in `./cmd/assay-handler`, load the remote word list, process the sample essays, and print a JSON result to stdout.

Example output (shape may vary):

```json
{
  "top": [
    { "word": "example", "count": 42 },
    { "word": "data", "count": 37 }
  ],
  "total_distinct": 1234
}
```

## Repository layout
```
firefly-assignment/
├── Taskfile.yaml
├── cmd/
│   └── assay-handler/
│       ├── assays.list
│       └── main.go
├── pkg/
│   ├── assays/
│   ├── http/
│   ├── logger/
│   ├── models/
│   ├── utils/
│   └── words_bank/
└── vendor/
```
