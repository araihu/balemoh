// Package plugins adapts trusted deployment files and SDK processes to core ports.
package plugins

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	app "github.com/araihu/balemoh/internal/application/plugins"
	"github.com/araihu/balemoh/plugin-sdk/credential"
	"github.com/araihu/balemoh/plugin-sdk/manifest"
	pluginruntime "github.com/araihu/balemoh/plugin-sdk/runtime"
)

type File struct {
	Installations []Installation `json:"installations"`
}
type Installation struct {
	Package        string       `json:"package"`
	ManifestDigest string       `json:"manifestDigest"`
	PackageDigest  string       `json:"packageDigest"`
	Connections    []Connection `json:"connections"`
}
type Connection struct {
	Key              string            `json:"key"`
	Name             string            `json:"name"`
	Config           json.RawMessage   `json:"config"`
	Secrets          map[string]string `json:"secrets"`
	Grants           []string          `json:"grants"`
	CapabilityGrants []string          `json:"capabilityGrants,omitempty"`
}
type preparedConnection struct {
	key              string
	config           json.RawMessage
	secrets          map[string]string
	grants           []string
	capabilityGrants []string
	record           app.Provision
}
type preparedInstallation struct {
	pkg         *manifest.Package
	connections []preparedConnection
}
type Deployment struct {
	installations []preparedInstallation
	problems      []app.InstallationProblem
}

func ReadCredential(path string) (string, error) { return credential.ReadBearerFile(path) }

func Load(path string) (*Deployment, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.New("cannot read plugin deployment")
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, (256<<10)+1))
	if err != nil || len(data) > 256<<10 {
		return nil, errors.New("invalid plugin deployment size")
	}
	var file File
	d := json.NewDecoder(strings.NewReader(string(data)))
	d.DisallowUnknownFields()
	if d.Decode(&file) != nil || d.Decode(new(any)) != io.EOF || file.Installations == nil || len(file.Installations) > 16 {
		return nil, errors.New("invalid plugin deployment")
	}
	result := &Deployment{}
	seen := map[string]bool{}
	resolve := func(name string) string {
		if !filepath.IsAbs(name) {
			return filepath.Join(filepath.Dir(path), name)
		}
		return name
	}
	for index, installation := range file.Installations {
		installationID := fmt.Sprintf("installation-%d", index+1)
		report := func(plugin, key string) {
			result.problems = append(result.problems, app.InstallationProblem{InstallationID: installationID, PluginID: plugin, ConnectionKey: key, Error: app.Fail("invalid_argument")})
		}
		if strings.TrimSpace(installation.Package) == "" {
			report("", "")
			continue
		}
		p, err := manifest.Load(resolve(installation.Package))
		if err != nil {
			report("", "")
			continue
		}
		digest, err := p.ContentDigest()
		if err != nil || len(installation.ManifestDigest) != 64 || len(installation.PackageDigest) != 64 || p.Digest != installation.ManifestDigest || digest != installation.PackageDigest || pluginruntime.Supported(p.Manifest) != nil || seen[p.Manifest.ID] || installation.Connections == nil || len(installation.Connections) > 16 {
			report(p.Manifest.ID, "")
			continue
		}
		seen[p.Manifest.ID] = true
		prepared := preparedInstallation{pkg: p}
		keys := map[string]bool{}
		for _, c := range installation.Connections {
			key := c.Key
			if !connectionKey.MatchString(key) {
				report(p.Manifest.ID, "")
				continue
			}
			if keys[key] {
				report(p.Manifest.ID, key)
				continue
			}
			keys[key] = true
			value, err := prepareConnection(p, digest, c, resolve)
			if err != nil {
				report(p.Manifest.ID, key)
				continue
			}
			prepared.connections = append(prepared.connections, value)
		}
		result.installations = append(result.installations, prepared)
	}
	return result, nil
}

var connectionKey = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)

func prepareConnection(p *manifest.Package, digest string, c Connection, resolve func(string) string) (preparedConnection, error) {
	invalid := func() (preparedConnection, error) { return preparedConnection{}, app.Fail("invalid_argument") }
	if strings.TrimSpace(c.Name) == "" || len(c.Name) > 128 || c.Grants == nil || len(c.Config) > 64<<10 || p.ValidateJSON(p.Manifest.ConnectionSchema, c.Config) != nil {
		return invalid()
	}
	var canonical any
	decoder := json.NewDecoder(bytes.NewReader(c.Config))
	decoder.UseNumber()
	if decoder.Decode(&canonical) != nil {
		return invalid()
	}
	config, _ := json.Marshal(canonical)
	secrets := map[string]string{}
	declared := map[string]bool{}
	for _, secret := range p.Manifest.Secrets {
		declared[secret.ID] = true
		if secret.Required && c.Secrets[secret.ID] == "" {
			return invalid()
		}
	}
	for id, ref := range c.Secrets {
		if !declared[id] || ref == "" {
			return invalid()
		}
		value, err := ReadCredential(resolve(ref))
		if err != nil {
			return invalid()
		}
		secrets[id] = value
	}
	grants := append([]string{}, c.Grants...)
	sort.Strings(grants)
	for i, grant := range grants {
		query, ok := p.Manifest.Query(grant)
		if !ok || query.Capability != "data.query/v1" || i > 0 && grants[i-1] == grant {
			return invalid()
		}
	}
	capabilityGrants := append([]string{}, c.CapabilityGrants...)
	sort.Strings(capabilityGrants)
	for i, capability := range capabilityGrants {
		if capability != notificationPublishCapability || !p.Manifest.Declares(capability) || i > 0 && capabilityGrants[i-1] == capability {
			return invalid()
		}
	}
	raw, _ := json.Marshal(struct {
		Config           json.RawMessage
		Secrets          map[string]string
		Grants           []string
		CapabilityGrants []string `json:",omitempty"`
		PackageDigest    string
	}{config, secrets, grants, capabilityGrants, digest})
	hash := sha256.Sum256(raw)
	record := app.Provision{PluginID: p.Manifest.ID, Key: c.Key, Name: c.Name, Digest: hex.EncodeToString(hash[:]), ManifestDigest: p.Digest, Version: p.Manifest.Version}
	return preparedConnection{key: c.Key, config: config, secrets: secrets, grants: grants, capabilityGrants: capabilityGrants, record: record}, nil
}

// Problems contains public identifiers and fixed errors only, never paths, URLs,
// references or values from rejected deployment data.
func (d *Deployment) Problems() []app.InstallationProblem {
	result := []app.InstallationProblem{}
	if d != nil {
		result = append(result, d.problems...)
	}
	return result
}

func (d *Deployment) Provisions() []app.Provision {
	result := []app.Provision{}
	if d != nil {
		for _, p := range d.installations {
			for _, c := range p.connections {
				result = append(result, c.record)
			}
		}
	}
	return result
}
