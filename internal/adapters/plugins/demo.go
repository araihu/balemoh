package plugins

import (
	"encoding/json"
	"fmt"
)

// ValidateDemo rejects real or unknown sources before any child starts.
func (d *Deployment) ValidateDemo(sourceURL string) error {
	if d == nil {
		return nil
	}
	if len(d.problems) > 0 {
		return fmt.Errorf("demo plugin deployment contains invalid installations")
	}
	for _, installation := range d.installations {
		if installation.pkg.Manifest.ID != "araihu.alertmanager" {
			return fmt.Errorf("demo allows only the synthetic Alertmanager connection")
		}
		for _, connection := range installation.connections {
			var source struct {
				APIBaseURL     string `json:"apiBaseURL"`
				BrowserBaseURL string `json:"browserBaseURL"`
			}
			if json.Unmarshal(connection.config, &source) != nil || connection.key != "demo" || source.APIBaseURL != sourceURL || source.BrowserBaseURL != "" || len(connection.secrets) > 0 || len(connection.capabilityGrants) > 0 {
				return fmt.Errorf("demo plugin source must use its synthetic loopback listener without source credentials or capabilities")
			}
		}
	}
	return nil
}
