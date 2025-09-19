package config

import (
	"fmt"
	"testing"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/stretchr/testify/assert"
)

func TestMakeConnStr(t *testing.T) {
	tests := []struct {
		name        string
		conf        Database
		wantConnStr string
		assertErr   assert.ErrorAssertionFunc
	}{
		{
			name: "Make connection string",
			conf: Database{
				Host: commoncfg.SourceRef{
					Source: "embedded",
					Value:  "my_host",
				},
				User: commoncfg.SourceRef{
					Source: "embedded",
					Value:  "my_user",
				},
				Password: commoncfg.SourceRef{
					Source: "embedded",
					Value:  "my_password",
				},
				Name: "my_db_name",
				Port: "5432",
			},
			wantConnStr: "host=my_host user=my_user password=my_password dbname=my_db_name port=5432",
			assertErr:   assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			connStr, err := MakeConnStr(tt.conf)
			if !tt.assertErr(t, err, fmt.Sprintf("MakeConnStr() error = %v", err)) || err != nil {
				return
			}

			assert.Equal(t, tt.wantConnStr, connStr, "MakeConnStr() = %v", connStr)
		})
	}
}
