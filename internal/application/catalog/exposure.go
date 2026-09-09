package catalog

// Exposure retains route scope and match semantics that a display URL cannot.
// Stored as JSON metadata so discovery snapshots keep the evidence unchanged.
type Exposure struct {
	Scope        string `json:"scope"`
	Host         string `json:"host"`
	Path         string `json:"path"`
	Match        string `json:"match"`
	Service      string `json:"service,omitempty"`
	Port         int    `json:"port,omitempty"`
	RedirectHost string `json:"redirectHost,omitempty"`
	RedirectPath string `json:"redirectPath,omitempty"`
}

const ExposureMetadata = "kubernetes.exposures.v1"
const BackendMetadata = "kubernetes.backend.v1"
