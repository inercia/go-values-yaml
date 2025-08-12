# go-values-yaml

Go library with utilities for managing Helm values.yaml files (useful for Argo CD/Helm repos).

## Features

### Values Management

- **Create Values** from multiple sources:
  - YAML documents
  - JSON documents
  - Files (values.yaml)
  - Filesystem (fs.FS interface)
- **Deep operations**:
  - Deep copy Values
  - Deep merge with customizable options (slice merging, empty value overwriting)
  - YAML equality comparison
- **Advanced lookups**:
  - Nested key access using dot notation (`foo.bar.baz`)
  - Array indexing support (`foo[0].bar`)
  - Type-safe lookups (LookupString, LookupInt, LookupValues)
  - Multiple key fallback (LookupFirst)
- **Value manipulation**:
  - Set values with nested paths and array indices
  - Rebase values under new parent keys
  - JSON/YAML serialization

### YAML Structure Extraction

- **Extract common structure** between YAML documents:
  - Between two values.yaml files (ExtractCommon)
  - Between N values.yaml files (ExtractCommonN)
  - Preserves Helm merge semantics: `merge(common, remainder) == original`
- **File-based extraction**:
  - Operates on sibling values.yaml files
  - Writes common structure to parent directory
  - Atomic file operations for safety
- **Recursive directory extraction**:
  - Walks directory tree bottom-up
  - Progressively extracts common structures at each level
  - Creates hierarchy of values.yaml files
  - Supports mixed-depth descendants

### Additional Capabilities

- **Memory filesystem support** for testing
- **Comprehensive error handling** with typed errors
- **Thread-safe file operations** with atomic writes
- **Order-insensitive YAML comparison**
- **Unicode and special character support**

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
