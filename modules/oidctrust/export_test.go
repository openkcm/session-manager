package oidctrust

func NewModule(repo TrustRepository) *TrustModule {
	return &TrustModule{
		repository: repo,
	}
}
