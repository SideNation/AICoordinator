package cmd

import (
	"reflect"
	"strings"
	"testing"
)

func TestMatchInstalledDocs(t *testing.T) {
	installed := map[string]string{"unity": "1.0", "unity-ui": "1.0", "dotnet": "2.0"}
	resolve := func(p string) (string, bool) {
		if strings.EqualFold(p, "DOTNET") {
			return "dotnet", true
		}
		return "", false
	}

	cases := []struct {
		patterns []string
		want     []string
	}{
		{[]string{"unity"}, []string{"unity"}},
		{[]string{"unity*"}, []string{"unity", "unity-ui"}},
		{[]string{"DOTNET", "dotnet"}, []string{"dotnet"}},
		{[]string{"missing"}, nil},
	}
	for _, c := range cases {
		if got := matchInstalled(installed, resolve, c.patterns); !reflect.DeepEqual(got, c.want) {
			t.Errorf("matchInstalled(%v) = %v, want %v", c.patterns, got, c.want)
		}
	}
}
