//go:build !windows

package platform

import (
	"reflect"
	"strings"
	"testing"
)

func quoteTestArg(arg string) string { return "'" + strings.ReplaceAll(arg, "'", "'\\''") + "'" }

func TestSplitArgs(t *testing.T) {
	for _, tt := range []struct {
		raw  string
		want []string
	}{
		{`one "two three" '' 'literal\path'`, []string{"one", "two three", "", `literal\path`}},
		{`"C:\path\file" end\\ "a\"b"`, []string{`C:\path\file`, `end\`, `a"b`}},
		{`'$HOME' "$(echo unsafe)" ';'`, []string{"$HOME", "$(echo unsafe)", ";"}},
		{"escaped\\ space \\\ncontinued", []string{"escaped space", "continued"}},
	} {
		got, err := splitArgs(tt.raw)
		if err != nil || !reflect.DeepEqual(got, tt.want) {
			t.Errorf("splitArgs(%q) = %q, %v; want %q", tt.raw, got, err, tt.want)
		}
	}
	for _, raw := range []string{`"unclosed`, `trailing\`} {
		if _, err := splitArgs(raw); err == nil {
			t.Errorf("splitArgs(%q) should fail", raw)
		}
	}
}
