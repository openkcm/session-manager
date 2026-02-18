# Helm Unit Tests

This directory contains unit tests for the session-manager Helm chart.

## Test Files

### chart_test.go
- **TestFullChartRendering**: Tests complete chart rendering with various configurations
  - Default values
  - Name override
  - Multiple replicas
- **TestChartValidation**: Validates chart syntax and structure
- **TestLabelsAndAnnotations**: Verifies standard Kubernetes labels are applied

### configmap_test.go
- **TestConfigMapRendering**: Tests ConfigMap template rendering

### deployment_test.go
- **TestDeploymentRendering**: Tests session-manager deployment with various configurations
  - Default values
  - Custom replica count
  - Custom image tag
  - Custom pull policy
- **TestHousekeeperDeployment**: Tests housekeeper deployment rendering

### hpa_test.go
- **TestHPARendering**: Tests HorizontalPodAutoscaler rendering when enabled

### job_test.go
- **TestMigrateJobRendering**: Tests database migration job rendering

### pdb_test.go
- **TestPDBRendering**: Tests PodDisruptionBudget rendering (skips if template doesn't exist)

### service_test.go
- **TestServiceRendering**: Tests service configuration with various settings
  - Default service
  - Custom service type
  - Custom service port

### serviceaccount_test.go
- **TestServiceAccountRendering**: Tests ServiceAccount rendering
  - Default service account
  - Custom service account name

## Running Tests

Run all tests:
```bash
cd helm-tests/unit
go test -v
```

Run specific test:
```bash
go test -v -run TestDeploymentRendering
```

Run with coverage:
```bash
go test -v -cover
```

## Adding New Tests

1. Create a new test file following the pattern `<resource>_test.go`
2. Use the `package main_test` declaration
3. Import necessary packages (`testing`, `os/exec`, `bytes`, `strings`)
4. Use the constants `path` and `appName` from `main_test.go`
5. Follow the existing test structure for consistency

## Test Structure

Each test typically:
1. Defines test cases with different values
2. Runs `helm template` command with specific values
3. Captures and validates the output
4. Checks for expected strings or counts in the rendered YAML

Example:
```go
func TestMyResource(t *testing.T) {
    tests := []struct {
        name     string
        values   string
        expected []string
    }{
        {
            name:   "default",
            values: "",
            expected: []string{"kind: MyResource"},
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Run helm template and validate
        })
    }
}
```

