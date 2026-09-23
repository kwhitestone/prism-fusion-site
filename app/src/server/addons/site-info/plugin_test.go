package siteinfo

import (
	"reflect"
	"testing"

	"github.com/kwhitestone/prism-fusion/plugin"
)

func TestSiteInfoV1Compatibility(t *testing.T) {
	manifest, err := plugin.ResolveManifest(newSiteInfoPlugin())
	if err != nil {
		t.Fatal(err)
	}
	if manifest.APIVersion != plugin.APIVersionV1 || manifest.ID != "site-info" ||
		!reflect.DeepEqual(manifest.RouteScopes, []string{"/api/v1/addons/site-info"}) {
		t.Fatalf("unexpected compatibility manifest: %#v", manifest)
	}
}
