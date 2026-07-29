package standard

import (
	_ "github.com/openkcm/session-manager/modules/app/grpcserver"
	_ "github.com/openkcm/session-manager/modules/credentials/oauth2"
	_ "github.com/openkcm/session-manager/modules/database/pgxpool"
	_ "github.com/openkcm/session-manager/modules/grpc/oidcmapping"
	_ "github.com/openkcm/session-manager/modules/grpc/session"
	_ "github.com/openkcm/session-manager/modules/grpc/trustmapping"
	_ "github.com/openkcm/session-manager/modules/oidctrust"
	_ "github.com/openkcm/session-manager/modules/oidctrust/migrations"
	_ "github.com/openkcm/session-manager/modules/sessionstore/valkey"
)
