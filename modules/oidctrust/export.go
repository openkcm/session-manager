package oidctrust

import (
	_ "unsafe"

	sessionmanager "github.com/openkcm/session-manager"
)

//nolint:unused
//go:linkname newOIDCTrustModuleWithRepo
func newOIDCTrustModuleWithRepo(r TrustRepository) sessionmanager.Trust {
	return &TrustModule{
		repository: r,
	}
}
