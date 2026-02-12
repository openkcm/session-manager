package trust

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProvider_GetIntrospectParameters(t *testing.T) {
	tests := []struct {
		name       string
		provider   Provider
		keys       []string
		wantParams map[string]string
	}{
		{
			name: "returns matching parameters",
			provider: Provider{
				Properties: map[string]string{
					"client_id":     "my-client-id",
					"client_secret": "my-secret",
					"scope":         "openid",
				},
			},
			keys: []string{"client_id", "client_secret"},
			wantParams: map[string]string{
				"client_id":     "my-client-id",
				"client_secret": "my-secret",
			},
		},
		{
			name: "skips missing parameters",
			provider: Provider{
				Properties: map[string]string{
					"client_id": "my-client-id",
				},
			},
			keys: []string{"client_id", "missing_key"},
			wantParams: map[string]string{
				"client_id": "my-client-id",
			},
		},
		{
			name: "returns empty map when no keys provided",
			provider: Provider{
				Properties: map[string]string{
					"client_id": "my-client-id",
				},
			},
			keys:       []string{},
			wantParams: map[string]string{},
		},
		{
			name: "returns empty map when properties is nil",
			provider: Provider{
				Properties: nil,
			},
			keys:       []string{"client_id"},
			wantParams: map[string]string{},
		},
		{
			name: "returns empty map when no keys match",
			provider: Provider{
				Properties: map[string]string{
					"client_id": "my-client-id",
				},
			},
			keys:       []string{"non_existent_key"},
			wantParams: map[string]string{},
		},
		{
			name: "handles all keys matching",
			provider: Provider{
				Properties: map[string]string{
					"key1": "value1",
					"key2": "value2",
					"key3": "value3",
				},
			},
			keys: []string{"key1", "key2", "key3"},
			wantParams: map[string]string{
				"key1": "value1",
				"key2": "value2",
				"key3": "value3",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.provider.GetIntrospectParameters(tt.keys)
			assert.Equal(t, tt.wantParams, got)
		})
	}
}
