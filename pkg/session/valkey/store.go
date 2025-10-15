package sessionvalkey

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/valkey-io/valkey-go"

	"github.com/openkcm/session-manager/internal/serviceerr"
)

type store struct {
	valkey valkey.Client
	prefix string
}

func newStore(valkeyClient valkey.Client, prefix string) *store {
	prefix = strings.TrimSuffix(prefix, ":")
	return &store{
		valkey: valkeyClient,
		prefix: prefix,
	}
}

func (s *store) Get(ctx context.Context, objectType, objectID string, decodeInto any) error {
	key := s.key(objectType, objectID)
	return s.get(ctx, key, decodeInto)
}

func (s *store) Set(ctx context.Context, objectType, id string, val any) error {
	key := s.key(objectType, id)
	bytes, err := s.encode(val)
	if err != nil {
		return fmt.Errorf("encoding data: %w", err)
	}

	if err := s.valkey.Do(ctx, s.valkey.B().Set().Key(key).Value(valkey.BinaryString(bytes)).Build()).Error(); err != nil {
		return fmt.Errorf("executing set command: %w", err)
	}

	return nil
}

func (s *store) Destroy(ctx context.Context, objectType, id string) error {
	key := s.key(objectType, id)
	if err := s.valkey.Do(ctx, s.valkey.B().Del().Key(key).Build()).Error(); err != nil {
		return fmt.Errorf("executing del command: %w", err)
	}

	return nil
}

func (s *store) get(ctx context.Context, key string, decodeInto any) error {
	bytes, err := s.valkey.Do(ctx, s.valkey.B().Get().Key(key).Build()).AsBytes()
	if err != nil {
		valkeyErr, ok := valkey.IsValkeyErr(err)
		if ok && valkeyErr.IsNil() {
			return errors.Join(valkeyErr, serviceerr.ErrConflict)
		}

		return fmt.Errorf("executing get command: %w", err)
	}

	if err := s.decode(bytes, decodeInto); err != nil {
		return fmt.Errorf("decoding state: %w", err)
	}

	return nil
}

func (s *store) key(objectType string, objectID string) string {
	return fmt.Sprintf("%s:%s:%s", s.prefix, objectType, objectID)
}

func (s *store) encode(v any) ([]byte, error) {
	bytes, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshaling json: %w", err)
	}

	return bytes, nil
}

func (s *store) decode(data []byte, into any) error {
	if err := json.Unmarshal(data, into); err != nil {
		return fmt.Errorf("unmarshaling json: %w", err)
	}

	return nil
}

func getStoreObjects[T any](ctx context.Context, s *store, objectType string, objectID string, decodeInto *[]T) error {
	key := s.key(objectType, objectID)
	var cursor uint64
	for {
		scan, err := s.valkey.Do(ctx, s.valkey.B().Scan().Cursor(cursor).Match(key).Count(100).Build()).AsScanEntry()
		if err != nil {
			return fmt.Errorf("executing scan command: %w", err)
		}

		cursor = scan.Cursor
		*decodeInto = slices.Grow(*decodeInto, len(scan.Elements))
		for _, key := range scan.Elements {
			var decoded T
			if err := s.get(ctx, key, &decoded); err != nil {
				return fmt.Errorf("getting an element: %w", err)
			}

			*decodeInto = append(*decodeInto, decoded)
		}

		if cursor == 0 {
			return nil
		}
	}
}
