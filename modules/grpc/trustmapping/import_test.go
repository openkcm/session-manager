package trustmapping_test

import (
	sessionmanager "github.com/openkcm/session-manager"
	"github.com/openkcm/session-manager/modules/oidctrust"
	_ "github.com/openkcm/session-manager/modules/standard"
)

func newTrust(r oidctrust.TrustRepository) sessionmanager.Trust { return oidctrust.NewModule(r) }
