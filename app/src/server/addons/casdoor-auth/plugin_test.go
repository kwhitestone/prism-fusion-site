package casdoorauth

import (
	"reflect"
	"testing"

	"github.com/kwhitestone/prism-fusion/global"
	"github.com/kwhitestone/prism-fusion/plugin"
)

func TestCasdoorContractAndFrozenActivation(t *testing.T) {
	previous := global.PRISM_CONFIG
	t.Cleanup(func() { global.PRISM_CONFIG = previous })
	candidate := &CasdoorAuthPlugin{BasePlugin: plugin.BasePlugin{PluginName: "casdoor-auth"}}
	manifest, err := plugin.ResolveManifest(candidate)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.APIVersion != plugin.APIVersionV2 || manifest.ID != "casdoor-auth" ||
		!reflect.DeepEqual(manifest.RouteScopes, []string{"/api/v1/addons/casdoor-auth"}) ||
		len(manifest.Requires) != 0 || !reflect.DeepEqual(manifest.Provides, []string{"auth.identity"}) {
		t.Fatalf("unexpected manifest: %#v", manifest)
	}
	registry := plugin.NewRegistry()
	if err := registry.Register(candidate); err != nil {
		t.Fatal(err)
	}
	global.PRISM_CONFIG.Auth.Provider = "builtin"
	resolved, err := registry.Resolve()
	if err != nil || len(resolved) != 0 {
		t.Fatalf("disabled provider resolved: %v, %v", resolved, err)
	}
	global.PRISM_CONFIG.Auth.Provider = "casdoor"
	if err := registry.Freeze(); err != nil {
		t.Fatal(err)
	}
	global.PRISM_CONFIG.Auth.Provider = "builtin"
	resolved, err = registry.Resolve()
	if err != nil || len(resolved) != 1 || len(candidate.GlobalMiddlewares()) != 1 {
		t.Fatalf("activation changed after freeze: %v, %v", resolved, err)
	}
	if len(candidate.Models()) != 2 || candidate.Priority() != 10 {
		t.Fatal("unexpected model or priority contract")
	}
}
