// Package oidctrusttest provides test helpers for the oidctrust module.
package oidctrusttest

import (
	sessionmanager "github.com/openkcm/session-manager"
	"github.com/openkcm/session-manager/modules/oidctrust"
)

// NewModule returns a TrustModule backed by the given repository.
func NewModule(repo oidctrust.TrustRepository) sessionmanager.Trust {
	return oidctrust.NewModule(repo)
}
