package trust

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOIDCMappingr_GetIntrospectParameters(t *testing.T) {
	tests := []struct {
		name        string
		oidcMapping OIDCMapping
		keys        []string
		wantParams  map[string]string
	}{
		{
			name: "returns matching parameters",
			oidcMapping: OIDCMapping{
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
			oidcMapping: OIDCMapping{
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
			oidcMapping: OIDCMapping{
				Properties: map[string]string{
					"client_id": "my-client-id",
				},
			},
			keys:       []string{},
			wantParams: map[string]string{},
		},
		{
			name: "returns empty map when properties is nil",
			oidcMapping: OIDCMapping{
				Properties: nil,
			},
			keys:       []string{"client_id"},
			wantParams: map[string]string{},
		},
		{
			name: "returns empty map when no keys match",
			oidcMapping: OIDCMapping{
				Properties: map[string]string{
					"client_id": "my-client-id",
				},
			},
			keys:       []string{"non_existent_key"},
			wantParams: map[string]string{},
		},
		{
			name: "handles all keys matching",
			oidcMapping: OIDCMapping{
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
			got := tt.oidcMapping.GetIntrospectParameters(tt.keys)
			assert.Equal(t, tt.wantParams, got)
		})
	}
}
