package sessionvalkey

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

func (s *store) Get(ctx context.Context, objectType, tenantID, objectID string, decodeInto any) error {
	key := s.key(objectType, tenantID, objectID)
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

func (s *store) Set(ctx context.Context, objectType, tenantID, id string, val any) error {
	key := s.key(objectType, tenantID, id)
	bytes, err := s.encode(val)
	if err != nil {
		return fmt.Errorf("encoding data: %w", err)
	}

	if err := s.valkey.Do(ctx, s.valkey.B().Set().Key(key).Value(valkey.BinaryString(bytes)).Build()).Error(); err != nil {
		return fmt.Errorf("executing set command: %w", err)
	}

	return nil
}

func (s *store) Destroy(ctx context.Context, objectType, tenantID, id string) error {
	key := s.key(objectType, tenantID, id)
	if err := s.valkey.Do(ctx, s.valkey.B().Del().Key(key).Build()).Error(); err != nil {
		return fmt.Errorf("executing del command: %w", err)
	}

	return nil
}

func (s *store) key(objectType string, tenantID, objectID string) string {
	return fmt.Sprintf("%s:%s:%s:%s", s.prefix, objectType, tenantID, objectID)
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
