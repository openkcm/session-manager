package trustmapping_test

import (
	_ "unsafe"

	sessionmanager "github.com/openkcm/session-manager"
	"github.com/openkcm/session-manager/modules/oidctrust"
	_ "github.com/openkcm/session-manager/modules/standard"
)

//go:linkname newTrust github.com/openkcm/session-manager/modules/oidctrust.newOIDCTrustModuleWithRepo
func newTrust(r oidctrust.TrustRepository) sessionmanager.Trust
