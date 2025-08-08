# go-values-yaml

Go library with utilities for managing Helm values.yaml files (useful for Argo CD/Helm repos).

## Features

- Extract common structure between two YAML docs.
- File-based extraction for values.yaml siblings, .writing common to the parent dir

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
