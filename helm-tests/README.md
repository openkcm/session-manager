# Helm Tests for Session Manager

This directory contains comprehensive automated tests for the session-manager Helm chart.

## Directory Structure

```
helm-tests/
├── go.mod              # Go module definition
├── go.sum              # Go module checksums
├── main.go             # Main package file
├── integration/        # Integration tests
│   ├── helm-install_test.go
│   └── main_test.go
└── unit/              # Unit tests (125 test cases)
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
- **125 test cases covering 100% of templates**

**Coverage:**
- Deployments (session-manager and housekeeper)
- Services
- ConfigMaps (all 11 configuration sections)
- ServiceAccounts (including Role and RoleBinding)
- HorizontalPodAutoscaler (HPA)
- Jobs (migration with hooks)
- PodDisruptionBudget (PDB)
- Labels and annotations
- Chart validation

### Integration Tests (`integration/`)
Integration tests verify the chart deployment in a real Kubernetes cluster. These tests:
- Deploy dependencies (PostgreSQL, Valkey)
- Install the session-manager chart
- Wait for pods to become available
- Verify services are created correctly
- Test full deployment lifecycle
- Verify that all expected resources are present
- Can be extended to test actual installations in test clusters

## Running Tests

### Using Makefile Targets (Recommended)

From the repository root:

```bash
# Run all helm tests (unit + integration with k3d setup/teardown)
make helm-test

# Run unit tests only
make helm-unit-test

# Run integration tests only (with automatic k3d setup/teardown)
make helm-integration-test

# Just run integration test without setup/teardown
make helm-integration-test-run

# Setup k3d cluster for manual testing
make k3d-setup

# Teardown k3d cluster
make k3d-teardown
```

### Using Go Commands Directly

```bash
# Run all tests
cd helm-tests
go test ./...

# Run unit tests only
cd helm-tests/unit
go test -v -count=1 -race ./...

# Run integration tests only (requires k3d cluster running)
cd helm-tests/integration
go test -v -count=1 -race .

# Run specific test
cd helm-tests/unit
go test -v -run TestDeploymentRendering

# Run with coverage
cd helm-tests
go test -cover ./...
```


## Prerequisites

- Go 1.26 or later
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

### GitHub Actions

Helm tests are **automatically run** on every push via GitHub Actions.

The workflows are defined in:
- **`.github/workflows/helm-tests.yaml`** - Runs helm unit tests using race detection
- **`.github/workflows/ci.yaml`** - Runs main code quality checks

The helm-tests workflow:
- **Job: `helm-test`** - Runs unit tests only (integration tests require a k3d cluster)
- **Triggers:** On push (any branch)
- **Requirements:** Go version from helm-tests/go.mod, Helm CLI

### Running Locally

You can run the same tests that CI runs:

```bash
# From repository root - run unit tests
make helm-unit-test

# Run integration tests (sets up k3d, runs tests, tears down)
make helm-integration-test

# Run all helm tests
make helm-test
```

### Integration with CI/CD Pipelines

For other CI systems, you can use the Makefile targets:

```yaml
# Example for other CI systems
- name: Run Helm Unit Tests
  run: make helm-unit-test

- name: Run Helm Integration Tests
  run: make helm-integration-test
```

**Note:** The `make helm-integration-test` target automatically:
1. Tears down any existing k3d cluster
2. Sets up a fresh k3d cluster
3. Builds and imports the Docker image
4. Runs the integration tests
5. Tears down the k3d cluster

This ensures a clean test environment every time.

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
