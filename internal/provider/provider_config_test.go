// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/X-Guardian/terraform-provider-harmonysase/internal/client"
)

func TestClientFromProviderData(t *testing.T) {
	t.Run("nil returns nil with no diagnostic", func(t *testing.T) {
		var diags diag.Diagnostics
		if got := clientFromProviderData(nil, &diags); got != nil {
			t.Errorf("nil providerData: got %v, want nil", got)
		}
		if diags.HasError() {
			t.Errorf("nil providerData should not produce diagnostics, got %v", diags)
		}
	})

	t.Run("valid client passes through", func(t *testing.T) {
		var diags diag.Diagnostics
		want := &client.Client{}
		got := clientFromProviderData(want, &diags)
		if got != want {
			t.Errorf("got %p, want %p", got, want)
		}
		if diags.HasError() {
			t.Errorf("unexpected diagnostics: %v", diags)
		}
	})

	t.Run("wrong type returns nil with error diagnostic", func(t *testing.T) {
		var diags diag.Diagnostics
		got := clientFromProviderData("not a client", &diags)
		if got != nil {
			t.Errorf("expected nil for wrong type, got %v", got)
		}
		if !diags.HasError() {
			t.Errorf("expected error diagnostic for wrong type")
		}
	})
}

func TestResolveIntAttr(t *testing.T) {
	const env = "HARMONYSASE_TEST_INT"

	cases := []struct {
		name    string
		v       types.Int64
		envVal  string // "" means unset
		want    int
		wantErr bool
	}{
		{"attr set", types.Int64Value(42), "", 42, false},
		{"attr null, env unset", types.Int64Null(), "", 0, false},
		{"attr null, env valid", types.Int64Null(), "75", 75, false},
		{"attr null, env whitespace", types.Int64Null(), "  ", 0, false},
		{"attr null, env invalid", types.Int64Null(), "abc", 0, true},
		{"attr unknown, env unset", types.Int64Unknown(), "", 0, false},
		{"attr set wins over env", types.Int64Value(10), "999", 10, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(env, tc.envVal)
			got, diag := resolveIntAttr(tc.v, env, "test_attr")
			if (diag != nil) != tc.wantErr {
				t.Fatalf("err diag: got %v, want err=%v", diag, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("got %d want %d", got, tc.want)
			}
		})
	}
}
