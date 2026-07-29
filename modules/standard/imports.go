package standard

import (
	_ "github.com/openkcm/session-manager/modules/database/pgxpool"
	_ "github.com/openkcm/session-manager/modules/oidctrust"
	_ "github.com/openkcm/session-manager/modules/oidctrust/migrations"
)
