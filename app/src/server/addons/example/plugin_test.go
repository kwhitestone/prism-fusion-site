package example

import (
	"errors"
	"reflect"
	"testing"

	"github.com/kwhitestone/prism-fusion/global"
	"github.com/kwhitestone/prism-fusion/plugin"
	casbinrbac "top.whitestone/prism-fusion-site/addons/casbin-rbac"
	casdoorauth "top.whitestone/prism-fusion-site/addons/casdoor-auth"
)

func TestExampleV2Contract(t *testing.T) {
	candidate := newExamplePlugin()
	manifest, err := plugin.ResolveManifest(candidate)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.APIVersion != plugin.APIVersionV2 || manifest.ID != "example" ||
		!reflect.DeepEqual(manifest.RouteScopes, []string{"/api/v1/addons/example"}) ||
		!reflect.DeepEqual(manifest.Requires, []plugin.Dependency{{ID: "casbin-rbac"}, {ID: "casdoor-auth"}}) {
		t.Fatalf("unexpected manifest: %#v", manifest)
	}
	if len(candidate.Models()) != 1 || len(candidate.Middlewares()) != 1 || len(candidate.GlobalMiddlewares()) != 1 {
		t.Fatal("missing model or middleware")
	}
}

func TestExampleRequiresActiveSiteProviders(t *testing.T) {
	previous := global.PRISM_CONFIG
	t.Cleanup(func() { global.PRISM_CONFIG = previous })
	global.PRISM_CONFIG.Auth.Provider = "casdoor"
	global.PRISM_CONFIG.RBAC.Provider = "casbin"
	registry := plugin.NewRegistry()
	for _, candidate := range []plugin.Plugin{
		newExamplePlugin(),
		&casbinrbac.CasbinRbacPlugin{BasePlugin: plugin.BasePlugin{PluginName: "casbin-rbac"}},
		&casdoorauth.CasdoorAuthPlugin{BasePlugin: plugin.BasePlugin{PluginName: "casdoor-auth"}},
	} {
		if err := registry.Register(candidate); err != nil {
			t.Fatal(err)
		}
	}
	global.PRISM_CONFIG.Auth.Provider = "builtin"
	if _, err := registry.Resolve(); !errors.Is(err, plugin.ErrMissingDependency) {
		t.Fatalf("disabled dependency accepted: %v", err)
	}
	global.PRISM_CONFIG.Auth.Provider = "casdoor"
	resolved, err := registry.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, entry := range resolved {
		ids = append(ids, entry.Manifest.ID)
	}
	if !reflect.DeepEqual(ids, []string{"casdoor-auth", "casbin-rbac", "example"}) {
		t.Fatalf("unexpected dependency order: %v", ids)
	}
}
