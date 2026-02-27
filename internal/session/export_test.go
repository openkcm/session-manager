package session

type TokenResponse = tokenResponse

// SetAllowHttpScheme sets the allowHttpScheme field for testing purposes.
func (m *Manager) SetAllowHttpScheme(allow bool) {
	m.allowHttpScheme = allow
}
