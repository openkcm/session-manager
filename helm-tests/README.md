# Helm Tests for Session Manager

This directory contains automated tests for the session-manager Helm chart.

## Directory Structure

```
helm-tests/
├── go.mod              # Go module definition
├── go.sum              # Go module checksums
├── main.go             # Main package file
├── integration/        # Integration tests
│   ├── helm-install_test.go
│   └── main_test.go
└── unit/              # Unit tests
    ├── chart_test.go
    ├── configmap_test.go
    ├── deployment_test.go
    ├── hpa_test.go
    ├── job_test.go
    ├── main_test.go
    ├── pdb_test.go
    ├── service_test.go
    ├── serviceaccount_test.go
    └── README.md
```

## Test Types

### Unit Tests (`unit/`)
Unit tests validate individual Helm templates and their rendering with various value configurations. These tests:
- Run `helm template` commands with different value sets
- Validate that expected Kubernetes resources are rendered
- Check for correct labels, annotations, and configurations
- Test various deployment scenarios (replicas, images, custom values)

**Coverage:**
- Deployments (session-manager and housekeeper)
- Services
- ConfigMaps
- ServiceAccounts
- HorizontalPodAutoscaler (HPA)
- Jobs (migration)
- PodDisruptionBudget (PDB)
- Labels and annotations
- Chart validation

### Integration Tests (`integration/`)
Integration tests verify the chart as a whole. These tests:
- Run `helm template` on the entire chart
- Verify that all expected resources are present
- Can be extended to test actual installations in test clusters

## Running Tests

### Run All Tests
```bash
cd helm-tests
go test ./...
```

### Run Unit Tests Only
```bash
cd helm-tests/unit
go test -v
```

### Run Integration Tests Only
```bash
cd helm-tests/integration
go test -v
```

### Run Specific Test
```bash
cd helm-tests/unit
go test -v -run TestDeploymentRendering
```

### Run with Coverage
```bash
cd helm-tests
go test -cover ./...
```

### Run with Race Detection
```bash
cd helm-tests
go test -race ./...
```

## Prerequisites

- Go 1.21 or later
- Helm 3.x installed and available in PATH
- Access to the session-manager chart at `../charts/session-manager`

## Adding New Tests

### Unit Test
1. Create a new file in `unit/` directory: `<resource>_test.go`
2. Use `package main_test`
3. Import required packages
4. Use constants from `main_test.go` (`path`, `appName`)
5. Write test functions following existing patterns

Example:
```go
package main_test

import (
    "bytes"
    "os/exec"
    "strings"
    "testing"
)

func TestNewResource(t *testing.T) {
    cmd := exec.Command("helm", "template", appName, path, "-s", "templates/myresource.yaml")
    var out bytes.Buffer
    cmd.Stdout = &out
    cmd.Stderr = &out

    err := cmd.Run()
    if err != nil {
        t.Fatalf("helm template failed: %v\nOutput: %s", err, out.String())
    }

    output := out.String()
    if !strings.Contains(output, "kind: MyResource") {
        t.Errorf("expected MyResource in output")
    }
}
```

### Integration Test
1. Add test functions to `integration/helm-install_test.go`
2. Test the chart as a whole or specific integration scenarios

## Continuous Integration

These tests can be integrated into CI/CD pipelines:

```yaml
# Example GitHub Actions workflow
- name: Run Helm Tests
  run: |
    cd helm-tests
    go test -v ./...
```

## Best Practices

1. **Test with default values first** - Ensure the chart works with minimal configuration
2. **Test common customizations** - Cover typical deployment scenarios
3. **Test edge cases** - Include tests for unusual or boundary conditions
4. **Keep tests fast** - Use `helm template` instead of actual installations when possible
5. **Clear test names** - Use descriptive names that explain what is being tested
6. **Validate errors** - Test that invalid configurations fail appropriately

## Troubleshooting

### Test Failures
- Verify Helm is installed: `helm version`
- Check chart path is correct: `ls -la ../charts/session-manager`
- Run helm template manually to see full output
- Check for recent chart changes that might affect tests

### Module Issues
```bash
cd helm-tests
go mod tidy
```

### Debugging
Add `-v` flag for verbose output:
```bash
go test -v ./...
```

Print helm output in tests for debugging:
```go
t.Logf("Helm output:\n%s", out.String())
```

## Contributing

When contributing tests:
1. Follow existing code structure and patterns
2. Add tests for new chart features
3. Update tests when modifying chart templates
4. Ensure all tests pass before submitting PRs
5. Document complex test scenarios

## Resources

- [Helm Testing Guide](https://helm.sh/docs/topics/charts_tests/)
- [Go Testing Package](https://pkg.go.dev/testing)
- [Helm Template Command](https://helm.sh/docs/helm/helm_template/)

