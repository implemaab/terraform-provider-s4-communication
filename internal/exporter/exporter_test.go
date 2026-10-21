package exporter

import (
	"testing"
)

func TestResourceNames(t *testing.T) {
	g := newGenerator()
	for label, want := range map[string]string{
		"EXT-API.1": "ext_api_1",
		"123 Sys":   "r_123_sys",
		"!!!":       "r_",
	} {
		if got := g.add("t", label, label, label); got != want {
			t.Errorf("name for %q = %q, want %q", label, got, want)
		}
	}
	// Names are unique per resource type
	if got := g.add("t", "ext api 1", "other", "other"); got != "ext_api_1_2" {
		t.Errorf("duplicate name = %q, want ext_api_1_2", got)
	}
}
