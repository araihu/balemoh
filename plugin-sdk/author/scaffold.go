// Package author implements the plugin author toolchain without host internals.
package author

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/araihu/balemoh/plugin-sdk/manifest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const SDKVersion = "v0.0.0"
const ProfileVersion = "v0.3.3"

// Init writes only into a new or empty directory. The SDK is explicitly unpublished.
func Init(dir, id, module string, backend bool) error {
	if module == "" {
		module = "example.com/" + strings.ReplaceAll(id, ".", "-")
	}
	if strings.ContainsAny(module, " \t\r\n`\"") || !strings.Contains(module, "/") {
		return errors.New("invalid module path")
	}
	m := manifest.Manifest{ManifestVersion: 1, ID: id, Version: "0.0.1", Name: id, Protocol: manifest.Protocol, UIProfile: manifest.UIProfile, Targets: map[string]manifest.Target{runtime.GOOS + "/" + runtime.GOARCH: {UI: "bin/ui"}}, ConnectionSchema: "schemas/connection.json", Secrets: []manifest.Secret{}, Queries: []manifest.Query{}, Pages: []manifest.Page{{ID: "current", Title: "Example page", Description: "Independent plugin application.", Path: "/current", Methods: []string{"GET"}, Capabilities: []string{}}}, Navigation: []manifest.Navigation{{ID: "example", Title: id, Page: "current"}}, Widgets: []manifest.Widget{{ID: "summary", Title: "Example widget", OptionsSchema: "schemas/options.json", Capabilities: []string{}}}, Assets: []manifest.Asset{{ID: "lifecycle", File: "assets/lifecycle.js", MediaType: "text/javascript", Kind: "module"}}, Capabilities: manifest.Capabilities{Required: []string{"page.content/v1", "home.widget/v1", "sidebar.navigation/v1"}, Optional: []string{}}}
	if backend {
		t := m.Targets[runtime.GOOS+"/"+runtime.GOARCH]
		t.Backend = "bin/backend"
		m.Targets[runtime.GOOS+"/"+runtime.GOARCH] = t
		m.Queries = []manifest.Query{{ID: "example.read", InputSchema: "schemas/input.json", OutputSchema: "schemas/output.json", Capability: "data.query/v1"}}
		m.Capabilities.Required = append(m.Capabilities.Required, "data.query/v1")
		m.Pages[0].Capabilities = []string{"data.query/v1"}
		m.Widgets[0].Capabilities = []string{"data.query/v1"}
	}
	raw, _ := json.MarshalIndent(m, "", "  ")
	if _, _, err := manifest.Decode(strings.NewReader(string(raw))); err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err == nil && len(entries) != 0 {
		return errors.New("destination must be empty")
	}
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	files := map[string]string{
		"assets/lifecycle.js":     `export function mount(root) { root.dataset.exampleMounted="true"; const host=document.documentElement; host.dataset.exampleActive=String(Number(host.dataset.exampleActive||0)+1); return () => { delete root.dataset.exampleMounted; host.dataset.exampleActive=String(Number(host.dataset.exampleActive||0)-1); }; }`,
		"plugin.json":             string(raw) + "\n",
		"go.mod":                  fmt.Sprintf("module %s\n\ngo 1.27.0\n\nrequire (\n github.com/araihu/balemoh/plugin-sdk %s\n github.com/a-h/templ v0.3.1070\n github.com/araihu/goshtoso %s\n)\n", module, SDKVersion, ProfileVersion),
		".gitignore":              "/dist/\n/.local/\n*.secret\n",
		"schemas/connection.json": `{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","properties":{},"additionalProperties":false}`,
		"schemas/options.json":    `{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","properties":{"title":{"type":"string","maxLength":128}},"additionalProperties":false}`,
		"internal/view/view.templ": `package view
import "github.com/araihu/goshtoso/components/panel"
import "github.com/araihu/goshtoso/components/link"
templ Content(id,message,refresh string) {
 @panel.Panel(panel.Config{Body:Message(id,message,refresh)})
}
templ Message(id,message,refresh string) {
 <p id={ id }>{ message }</p>
 if refresh!="" {
 @link.Link(refresh,link.WithAttrs(templ.Attributes{"hx-get":refresh,"hx-target":"closest [data-plugin-slot]","hx-swap":"innerHTML"})) { Refresh fixture }
 }
}
`,
		"internal/view/generate.go": "package view\n//go:generate go run github.com/a-h/templ/cmd/templ@v0.3.1070 generate\n",
		"README.md":                 fmt.Sprintf("# %s\n\nIndependent Balemoh application plugin. Go 1.27, SDK %s, Goshtoso %s.\n\nSDK is unpublished. For local development copy go.mod to .local/development.mod and add an explicit SDK replace there. Run templ generation for this project's own files, then tidy and test with GOWORK=off and the development modfile. Never add the local replace to exported go.mod.\n\nUse balemoh-plugin package --source . --output dist --target linux/amd64 --modfile .local/development.mod. Run balemoh-plugin check --package dist.\n\nUse balemoh-plugin dev --source . --host /path/to/balemoh-ui --sdk /path/to/plugin-sdk for a real-shell fixture preview. Dev uses loopback fixtures and never contacts production.\n", id, SDKVersion, ProfileVersion),
	}
	query := "message := \"Independent UI plugin\""
	if backend {
		query = `result,err:=host.Query(ctx,"example.read",json.RawMessage(` + "`{}`" + `));if err!=nil{return ui.Response{},err};var data struct{Message string ` + "`json:\"message\"`" + `};if json.Unmarshal(result.Data,&data)!=nil{return ui.Response{},ui.ErrUnavailable};message:="State: "+result.State+". "+data.Message;if data.Message==""{message+="Empty fixture"}`
		files["schemas/input.json"] = `{"type":"object","properties":{},"additionalProperties":false}`
		files["schemas/output.json"] = `{"type":"object","required":["message"],"properties":{"message":{"type":"string","maxLength":256}},"additionalProperties":false}`
		files["cmd/backend/main.go"] = backendSource
	}
	files["cmd/ui/main.go"] = fmt.Sprintf(`package main
import("context";%s"os";"%s/internal/view";"github.com/araihu/balemoh/plugin-sdk/manifest";"github.com/araihu/balemoh/plugin-sdk/runtime";"github.com/araihu/balemoh/plugin-sdk/render";"github.com/araihu/balemoh/plugin-sdk/ui")
func main(){p,err:=manifest.Load(".");if err!=nil{os.Exit(1)};runtime.Serve(p,ui.HandlerFunc(func(ctx context.Context,r ui.Request,host ui.HostServices)(ui.Response,error){%s;refresh,_:=r.WidgetURL();response,err:=render.Component(ctx,"Example",view.Content(r.DOMID("message"),message,refresh));response.AssetIDs=[]string{"lifecycle"};return response,err}))}
`, func() string {
		if backend {
			return `"encoding/json";`
		}
		return ""
	}(), module, query)
	files["internal/view/view_test.go"] = `package view
import("testing";"strings";"github.com/araihu/balemoh/plugin-sdk/render")
func TestContentEscapes(t *testing.T){r,e:=render.Component(t.Context(),"Example",Content("example","<script>bad</script>",""));if e!=nil||strings.Contains(string(r.HTML),"<script>"){t.Fatal("content boundary",e)}}
`
	if err = os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	for name, data := range files {
		target := filepath.Join(dir, name)
		if err = os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		f, e := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if e != nil {
			return e
		}
		_, e = f.WriteString(data)
		closeErr := f.Close()
		if e != nil {
			return e
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

const backendSource = `package main
import("context";"encoding/json";"os";"sync";"strconv";"time";"github.com/araihu/balemoh/plugin-sdk/backend";"github.com/araihu/balemoh/plugin-sdk/manifest";"github.com/araihu/balemoh/plugin-sdk/runtime")
type observation struct{status backend.Status;result backend.Result}
type example struct{mu sync.Mutex;connections map[string]observation}
func(e *example)Configure(ctx context.Context,c backend.Config)(backend.Status,error){
 e.mu.Lock();defer e.mu.Unlock();if ctx.Err()!=nil{return backend.Status{},ctx.Err()}
 if previous,ok:=e.connections[c.ConnectionID];ok{if previous.status.AppliedRevision==c.Revision{return previous.status,nil};old,_:=strconv.ParseUint(previous.status.AppliedRevision,10,64);next,_:=strconv.ParseUint(c.Revision,10,64);if next<old{return backend.Status{},backend.Fail("conflict")}}
 now:=time.Now().UTC();status:=backend.Status{ConnectionID:c.ConnectionID,AppliedRevision:c.Revision,State:"fresh",ObservationRevision:c.Revision,ObservedAt:&now,LastSuccessAt:&now,LastAttemptAt:&now}
 data,_:=json.Marshal(map[string]string{"message":"Independent backend fixture"});result:=backend.Result{Data:data,State:"fresh",ObservationRevision:c.Revision,ObservedAt:&now,LastSuccessAt:&now};e.connections[c.ConnectionID]=observation{status,result};return status,nil
}
func(e *example)Status(ctx context.Context,id string)(backend.Status,error){e.mu.Lock();defer e.mu.Unlock();value,ok:=e.connections[id];if !ok{return backend.Status{},backend.Fail("not_found")};return value.status,nil}
func(e *example)Query(ctx context.Context,r backend.Request)(backend.Result,error){e.mu.Lock();defer e.mu.Unlock();if ctx.Err()!=nil{return backend.Result{},ctx.Err()};value,ok:=e.connections[r.ConnectionID];if !ok{return backend.Result{},backend.Fail("not_found")};if r.OperationID!="example.read"{return backend.Result{},backend.Fail("permission_denied")};return value.result,nil}
func(e *example)CloseConnection(ctx context.Context,id string)error{e.mu.Lock();defer e.mu.Unlock();delete(e.connections,id);return nil}
func main(){p,err:=manifest.Load(".");if err!=nil{os.Exit(1)};runtime.ServeBackend(p,&example{connections:map[string]observation{}})}
`
