package buildinfo

import (
	"reflect"
	"runtime/debug"
	"testing"
)

func TestDependencies(t *testing.T) {
	if got := dependencies(nil); len(got) != 0 {
		t.Fatalf("missing build info returned dependencies: %+v", got)
	}
	info := &debug.BuildInfo{Deps: []*debug.Module{
		{Path: "example.org/gui", Version: "v1.2.3"},
		{Path: "example.org/replaced", Version: "v1.0.0", Replace: &debug.Module{Path: "example.org/fork", Version: "v2.0.0"}},
		{Path: "example.org/local", Version: "v1.0.0", Replace: &debug.Module{Path: "../local"}},
	}}
	want := []Dependency{
		{Path: "example.org/gui", Version: "v1.2.3"},
		{Path: "example.org/replaced => example.org/fork", Version: "v2.0.0"},
		{Path: "example.org/local => ../local", Version: "(devel)"},
	}
	if got := dependencies(info); !reflect.DeepEqual(got, want) {
		t.Fatalf("dependencies = %+v, want %+v", got, want)
	}
}
