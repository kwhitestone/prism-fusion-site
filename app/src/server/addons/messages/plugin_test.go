package messages

import (
	"reflect"
	"testing"

	"github.com/kwhitestone/prism-fusion/plugin"
)

func TestMessagesV1Compatibility(t *testing.T) {
	candidate := newMessagesPlugin()
	manifest, err := plugin.ResolveManifest(candidate)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.APIVersion != plugin.APIVersionV1 || manifest.ID != "messages" ||
		!reflect.DeepEqual(manifest.RouteScopes, []string{"/api/v1/addons/messages"}) {
		t.Fatalf("unexpected compatibility manifest: %#v", manifest)
	}
	if len(candidate.Middlewares()) != 0 {
		t.Fatal("Casdoor site must use global Casbin authorization, not builtin RBAC")
	}
}
