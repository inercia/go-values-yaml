# go-values-yaml

Go library with utilities for managing Helm values.yaml files (useful for Argo CD/Helm repos).

## Features

- Extract common structure between two YAML docs.
- File-based extraction for values.yaml siblings, .writing common to the parent dir
- Recursive extraction over a directory: for each parent with leaf sibling subdirs containing `values.yaml`, compute common and write parent `values.yaml`.

## Development

- Requires Go >= 1.22
- Common commands:
  - Build: `make build`
  - Test: `make test`
  - Lint: `make lint`
  - Format: `make fmt`
  - CI checks: `make ci`

## Module path

The default module path is `github.com/inercia/go-values-yaml`.
Update `go.mod` if your GitHub org/repo differ.

## Usage highlights

- Extract common between N sibling values files:
  ```go
  commonPath, err := values.ExtractCommonN([]string{"/repo/apps/a/values.yaml", "/repo/apps/b/values.yaml"})
  // writes /repo/apps/values.yaml and updates children
  ```
- Recursively extract across a tree, operating on groups of leaf sibling directories that have `values.yaml`:
  ```go
  created, err := values.ExtractCommonRecursive("/repo")
  // created contains parent values.yaml paths generated
  ```
