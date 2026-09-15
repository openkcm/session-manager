package session_test

import (
	sessionmanager "github.com/openkcm/session-manager"
	"github.com/openkcm/session-manager/modules/oidctrust"
	"github.com/openkcm/session-manager/modules/oidctrust/oidctrusttest"
	_ "github.com/openkcm/session-manager/modules/standard"
)

func newTrust(r oidctrust.TrustRepository) sessionmanager.Trust { return oidctrusttest.NewModule(r) }
