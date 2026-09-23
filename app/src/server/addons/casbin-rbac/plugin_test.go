package casbinrbac

import (
	"errors"
	"reflect"
	"testing"

	"github.com/kwhitestone/prism-fusion/global"
	"github.com/kwhitestone/prism-fusion/plugin"
)

func TestCasbinContractAndFrozenActivation(t *testing.T) {
	previous := global.PRISM_CONFIG
	t.Cleanup(func() { global.PRISM_CONFIG = previous })
	candidate := &CasbinRbacPlugin{BasePlugin: plugin.BasePlugin{PluginName: "casbin-rbac"}}
	manifest, err := plugin.ResolveManifest(candidate)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.APIVersion != plugin.APIVersionV2 || manifest.ID != "casbin-rbac" ||
		!reflect.DeepEqual(manifest.RouteScopes, []string{"/api/v1/addons/casbin-rbac"}) ||
		!reflect.DeepEqual(manifest.Requires, []plugin.Dependency{{ID: "casdoor-auth"}}) ||
		!reflect.DeepEqual(manifest.Provides, []string{"authorization.rbac"}) {
		t.Fatalf("unexpected manifest: %#v", manifest)
	}
	registry := plugin.NewRegistry()
	if err := registry.Register(candidate); err != nil {
		t.Fatal(err)
	}
	global.PRISM_CONFIG.RBAC.Provider = "builtin"
	resolved, err := registry.Resolve()
	if err != nil || len(resolved) != 0 {
		t.Fatalf("disabled provider resolved: %v, %v", resolved, err)
	}
	global.PRISM_CONFIG.RBAC.Provider = "casbin"
	if _, err := registry.Resolve(); !errors.Is(err, plugin.ErrMissingDependency) {
		t.Fatalf("missing Casdoor dependency: %v", err)
	}
	if err := registry.Register(&plugin.BasePlugin{PluginName: "casdoor-auth"}); err != nil {
		t.Fatal(err)
	}
	if err := registry.Freeze(); err != nil {
		t.Fatal(err)
	}
	global.PRISM_CONFIG.RBAC.Provider = "builtin"
	resolved, err = registry.Resolve()
	if err != nil || len(resolved) != 2 || resolved[1].Manifest.ID != "casbin-rbac" ||
		len(candidate.GlobalMiddlewares()) != 1 || len(candidate.Models()) != 1 || candidate.Priority() != 20 {
		t.Fatalf("activation/order changed after freeze: %v, %v", resolved, err)
	}
}
