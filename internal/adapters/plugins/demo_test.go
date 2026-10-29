package plugins

import (
	"github.com/araihu/balemoh/plugin-sdk/manifest"
	"testing"
)

func TestDemoRejectsRealOrUnknownPluginSources(t *testing.T) {
	for _, test := range []struct {
		name, plugin, key, config string
		secrets                   bool
		allowed                   bool
	}{
		{"synthetic", "araihu.alertmanager", "demo", `{"apiBaseURL":"http://127.0.0.1:9093"}`, false, true},
		{"private endpoint", "araihu.alertmanager", "demo", `{"apiBaseURL":"http://alerts.private:9093"}`, false, false},
		{"browser source link", "araihu.alertmanager", "demo", `{"apiBaseURL":"http://127.0.0.1:9093","browserBaseURL":"https://alerts.private"}`, false, false},
		{"private credentials", "araihu.alertmanager", "demo", `{"apiBaseURL":"http://127.0.0.1:9093"}`, true, false},
		{"wrong connection", "araihu.alertmanager", "homelab", `{"apiBaseURL":"http://127.0.0.1:9093"}`, false, false},
		{"other plugin", "example.other", "demo", `{}`, false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := preparedConnection{key: test.key, config: []byte(test.config)}
			if test.secrets {
				c.secrets = map[string]string{"bearer-token": "private-file"}
			}
			d := &Deployment{installations: []preparedInstallation{{pkg: &manifest.Package{Manifest: manifest.Manifest{ID: test.plugin}}, connections: []preparedConnection{c}}}}
			if err := d.ValidateDemo("http://127.0.0.1:9093"); (err == nil) != test.allowed {
				t.Fatalf("allowed=%v err=%v", test.allowed, err)
			}
		})
	}
}
