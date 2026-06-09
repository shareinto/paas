package argocd

type Config struct {
	BaseURL        string
	TokenSecretRef string
}

// Control plane intentionally does not expose Argo CD UI or directly operate clusters in MVP.
type Placeholder struct {
	Config Config
}
