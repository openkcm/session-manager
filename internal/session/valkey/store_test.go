package sessionvalkey

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/session-manager/internal/dbtest/valkeytest"
)

func TestNewStore(t *testing.T) {
	ctx := t.Context()
	valkeyClient, _, terminate := valkeytest.Start(ctx)
	defer terminate(ctx)

	t.Run("creates store with prefix", func(t *testing.T) {
		store := newStore(valkeyClient, "test-prefix")

		assert.NotNil(t, store)
		assert.Equal(t, "test-prefix", store.prefix)
		assert.NotNil(t, store.valkey)
	})

	t.Run("trims trailing colon from prefix", func(t *testing.T) {
		store := newStore(valkeyClient, "test-prefix:")

		assert.NotNil(t, store)
		assert.Equal(t, "test-prefix", store.prefix)
	})

	t.Run("trims only last trailing colon", func(t *testing.T) {
		store := newStore(valkeyClient, "test:prefix:")

		assert.NotNil(t, store)
		assert.Equal(t, "test:prefix", store.prefix)
	})

	t.Run("handles empty prefix", func(t *testing.T) {
		store := newStore(valkeyClient, "")

		assert.NotNil(t, store)
		assert.Empty(t, store.prefix)
	})
}

func TestStoreKey(t *testing.T) {
	ctx := t.Context()
	valkeyClient, _, terminate := valkeytest.Start(ctx)
	defer terminate(ctx)

	store := newStore(valkeyClient, "prefix")

	t.Run("generates correct key format", func(t *testing.T) {
		key := store.key("session", "session-123")
		assert.Equal(t, "prefix:session:session-123", key)
	})

	t.Run("handles different object types", func(t *testing.T) {
		tests := []struct {
			objectType ObjectType
			objectID   string
			expected   string
		}{
			{"session", "id-1", "prefix:session:id-1"},
			{"state", "id-2", "prefix:state:id-2"},
			{"active", "id-3", "prefix:active:id-3"},
			{"providerSession", "id-4", "prefix:providerSession:id-4"},
		}

		for _, tt := range tests {
			t.Run(string(tt.objectType), func(t *testing.T) {
				key := store.key(tt.objectType, tt.objectID)
				assert.Equal(t, tt.expected, key)
			})
		}
	})

	t.Run("handles special characters in object ID", func(t *testing.T) {
		key := store.key("session", "session:with:colons")
		assert.Equal(t, "prefix:session:session:with:colons", key)
	})
}

func TestStoreEncode(t *testing.T) {
	ctx := t.Context()
	valkeyClient, _, terminate := valkeytest.Start(ctx)
	defer terminate(ctx)

	store := newStore(valkeyClient, "prefix")

	t.Run("encodes simple struct", func(t *testing.T) {
		type TestData struct {
			Name  string `json:"name"`
			Value int    `json:"value"`
		}

		data := TestData{Name: "test", Value: 42}
		bytes, err := store.encode(data)

		require.NoError(t, err)
		assert.NotNil(t, bytes)
		assert.Contains(t, string(bytes), "test")
		assert.Contains(t, string(bytes), "42")
	})

	t.Run("encodes string", func(t *testing.T) {
		data := "test-string"
		bytes, err := store.encode(data)

		require.NoError(t, err)
		assert.NotNil(t, bytes)
		assert.Equal(t, "\"test-string\"", string(bytes))
	})

	t.Run("encodes map", func(t *testing.T) {
		data := map[string]string{"key": "value"}
		bytes, err := store.encode(data)

		require.NoError(t, err)
		assert.NotNil(t, bytes)
		assert.Contains(t, string(bytes), "key")
		assert.Contains(t, string(bytes), "value")
	})

	t.Run("encodes slice", func(t *testing.T) {
		data := []string{"one", "two", "three"}
		bytes, err := store.encode(data)

		require.NoError(t, err)
		assert.NotNil(t, bytes)
		assert.Contains(t, string(bytes), "one")
	})

	t.Run("returns error for invalid data", func(t *testing.T) {
		// Channels cannot be marshaled to JSON
		invalidData := make(chan int)
		_, err := store.encode(invalidData)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "marshaling json")
	})
}

func TestStoreDecode(t *testing.T) {
	ctx := t.Context()
	valkeyClient, _, terminate := valkeytest.Start(ctx)
	defer terminate(ctx)

	store := newStore(valkeyClient, "prefix")

	t.Run("decodes into struct", func(t *testing.T) {
		type TestData struct {
			Name  string `json:"name"`
			Value int    `json:"value"`
		}

		jsonData := []byte(`{"name":"test","value":42}`)
		var decoded TestData
		err := store.decode(jsonData, &decoded)

		require.NoError(t, err)
		assert.Equal(t, "test", decoded.Name)
		assert.Equal(t, 42, decoded.Value)
	})

	t.Run("decodes into map", func(t *testing.T) {
		jsonData := []byte(`{"key":"value","another":"data"}`)
		var decoded map[string]string
		err := store.decode(jsonData, &decoded)

		require.NoError(t, err)
		assert.Equal(t, "value", decoded["key"])
		assert.Equal(t, "data", decoded["another"])
	})

	t.Run("decodes into slice", func(t *testing.T) {
		jsonData := []byte(`["one","two","three"]`)
		var decoded []string
		err := store.decode(jsonData, &decoded)

		require.NoError(t, err)
		assert.Len(t, decoded, 3)
		assert.Equal(t, "one", decoded[0])
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		invalidJSON := []byte(`{invalid json}`)
		var decoded map[string]string
		err := store.decode(invalidJSON, &decoded)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "unmarshaling json")
	})

	t.Run("returns error for type mismatch", func(t *testing.T) {
		jsonData := []byte(`{"key":"value"}`)
		var decoded int
		err := store.decode(jsonData, &decoded)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "unmarshaling json")
	})

	t.Run("handles null values", func(t *testing.T) {
		jsonData := []byte(`null`)
		var decoded any
		err := store.decode(jsonData, &decoded)

		require.NoError(t, err)
		assert.Nil(t, decoded)
	})
}

func TestStoreSetGetDestroy(t *testing.T) {
	ctx := t.Context()
	valkeyClient, _, terminate := valkeytest.Start(ctx)
	defer terminate(ctx)

	// Use unique prefix for each test run
	prefix := "store-test-" + strings.ReplaceAll(time.Now().Format("20060102150405.000"), ".", "-")
	store := newStore(valkeyClient, prefix)

	t.Run("set and get data successfully", func(t *testing.T) {
		type TestData struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}

		data := TestData{ID: "test-1", Name: "Test Name"}
		duration := 5 * time.Minute

		// Set the data
		err := store.Set(ctx, "session", "test-id-1", data, duration)
		require.NoError(t, err)

		// Get the data back
		var result TestData
		err = store.Get(ctx, "session", "test-id-1", &result)
		require.NoError(t, err)
		assert.Equal(t, data.ID, result.ID)
		assert.Equal(t, data.Name, result.Name)
	})

	t.Run("get returns error for non-existent key", func(t *testing.T) {
		var result map[string]string
		err := store.Get(ctx, "session", "non-existent-key", &result)

		require.Error(t, err)
	})

	t.Run("set and destroy data successfully", func(t *testing.T) {
		data := map[string]string{"key": "value"}

		// Set the data
		err := store.Set(ctx, "session", "test-id-2", data, 5*time.Minute)
		require.NoError(t, err)

		// Verify it exists
		var result map[string]string
		err = store.Get(ctx, "session", "test-id-2", &result)
		require.NoError(t, err)

		// Destroy it
		err = store.Destroy(ctx, "session", "test-id-2")
		require.NoError(t, err)

		// Verify it's gone
		err = store.Get(ctx, "session", "test-id-2", &result)
		require.Error(t, err)
	})

	t.Run("destroy non-existent key does not error", func(t *testing.T) {
		err := store.Destroy(ctx, "session", "non-existent-key-destroy")
		// Valkey DEL command returns number of keys deleted, no error for non-existent
		require.NoError(t, err)
	})

	t.Run("set with expiration expires correctly", func(t *testing.T) {
		data := map[string]string{"key": "temporary"}

		// Set with short expiration (2 seconds)
		err := store.Set(ctx, "session", "test-id-3", data, 2*time.Second)
		require.NoError(t, err)

		// Should exist immediately
		var result map[string]string
		err = store.Get(ctx, "session", "test-id-3", &result)
		require.NoError(t, err)

		// Wait for expiration
		time.Sleep(3 * time.Second)

		// Should be gone
		err = store.Get(ctx, "session", "test-id-3", &result)
		require.Error(t, err)
	})

	t.Run("handles complex nested structures", func(t *testing.T) {
		type ComplexData struct {
			ID       string            `json:"id"`
			Tags     []string          `json:"tags"`
			Metadata map[string]string `json:"metadata"`
			Count    int               `json:"count"`
		}

		data := ComplexData{
			ID:       "complex-1",
			Tags:     []string{"tag1", "tag2", "tag3"},
			Metadata: map[string]string{"key1": "value1", "key2": "value2"},
			Count:    42,
		}

		err := store.Set(ctx, "session", "test-id-4", data, 5*time.Minute)
		require.NoError(t, err)

		var result ComplexData
		err = store.Get(ctx, "session", "test-id-4", &result)
		require.NoError(t, err)
		assert.Equal(t, data.ID, result.ID)
		assert.Equal(t, data.Tags, result.Tags)
		assert.Equal(t, data.Metadata, result.Metadata)
		assert.Equal(t, data.Count, result.Count)
	})

	t.Run("overwrites existing key", func(t *testing.T) {
		// Set initial data
		data1 := map[string]string{"version": "1"}
		err := store.Set(ctx, "session", "test-id-5", data1, 5*time.Minute)
		require.NoError(t, err)

		// Overwrite with new data
		data2 := map[string]string{"version": "2"}
		err = store.Set(ctx, "session", "test-id-5", data2, 5*time.Minute)
		require.NoError(t, err)

		// Verify we get the new data
		var result map[string]string
		err = store.Get(ctx, "session", "test-id-5", &result)
		require.NoError(t, err)
		assert.Equal(t, "2", result["version"])
	})
}

func TestStoreGetMethod(t *testing.T) {
	ctx := t.Context()
	valkeyClient, _, terminate := valkeytest.Start(ctx)
	defer terminate(ctx)

	prefix := "store-get-test-" + strings.ReplaceAll(time.Now().Format("20060102150405.000"), ".", "-")
	store := newStore(valkeyClient, prefix)

	t.Run("get method retrieves correct data type", func(t *testing.T) {
		data := map[string]any{
			"string": "value",
			"number": float64(123),
			"bool":   true,
		}

		err := store.Set(ctx, "test", "data-1", data, time.Minute)
		require.NoError(t, err)

		var result map[string]any
		err = store.Get(ctx, "test", "data-1", &result)
		require.NoError(t, err)
		assert.Equal(t, "value", result["string"])
		//nolint:testifylint
		assert.Equal(t, float64(123), result["number"])
		assert.Equal(t, true, result["bool"])
	})
}
