// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"net"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func mustCIDR(s string) *net.IPNet {
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		panic(err)
	}
	return n
}

func TestCIDRsOverlap(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		// Identical
		{"10.0.0.0/16", "10.0.0.0/16", true},
		// Containment
		{"10.0.0.0/16", "10.0.1.0/24", true},
		{"10.0.1.0/24", "10.0.0.0/16", true},
		// Disjoint, same family
		{"10.0.0.0/24", "10.0.1.0/24", false},
		{"10.0.0.0/16", "10.1.0.0/16", false},
		// Adjacent /17 halves of a /16
		{"10.0.0.0/17", "10.0.128.0/17", false},
		{"10.0.0.0/17", "10.0.0.0/16", true}, // /17 is inside /16
		// Different families never overlap
		{"10.0.0.0/8", "::/0", false},
	}
	for _, tc := range cases {
		got := cidrsOverlap(mustCIDR(tc.a), mustCIDR(tc.b))
		if got != tc.want {
			t.Errorf("cidrsOverlap(%s, %s) = %v want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

// runValidator builds a SetRequest, runs the validator, and returns the
// collected diagnostics.
func runValidator(t *testing.T, cidrs []string) string {
	t.Helper()
	ctx := context.Background()

	elems := make([]attr.Value, 0, len(cidrs))
	for _, c := range cidrs {
		elems = append(elems, types.StringValue(c))
	}
	set, diags := types.SetValue(types.StringType, elems)
	if diags.HasError() {
		t.Fatalf("build set: %v", diags)
	}

	req := validator.SetRequest{
		Path:        path.Root("remote_subnets"),
		ConfigValue: set,
	}
	resp := &validator.SetResponse{}
	NonOverlappingCIDRs().ValidateSet(ctx, req, resp)

	var msgs []string
	for _, d := range resp.Diagnostics.Errors() {
		msgs = append(msgs, d.Summary()+": "+d.Detail())
	}
	return strings.Join(msgs, "\n")
}

func TestValidator_HappyPaths(t *testing.T) {
	cases := [][]string{
		{},              // empty (size validator handles min 1; this validator passes)
		{"10.0.0.0/24"}, // single
		{"10.0.0.0/24", "10.0.1.0/24"},
		{"10.0.0.0/16", "10.1.0.0/16", "10.2.0.0/16"},
		// IPv6 disjoint
		{"2001:db8::/64", "2001:db8:1::/64"},
		// Mixed family — disjoint by definition
		{"10.0.0.0/8", "2001:db8::/64"},
		// Real live data sample
		{"10.18.0.0/16", "10.19.0.0/24", "10.153.0.0/21", "10.181.0.0/21"},
	}
	for _, c := range cases {
		if msg := runValidator(t, c); msg != "" {
			t.Errorf("expected pass for %v, got: %s", c, msg)
		}
	}
}

func TestValidator_RejectsOverlap(t *testing.T) {
	cases := []struct {
		input []string
		want  string // substring expected in error
	}{
		{[]string{"10.0.0.0/16", "10.0.1.0/24"}, "overlap"},
		{[]string{"10.0.0.0/24", "10.0.0.0/24"}, "overlap"}, // duplicates
		{[]string{"10.0.0.0/16", "10.0.0.0/8"}, "overlap"},
	}
	for _, tc := range cases {
		msg := runValidator(t, tc.input)
		if msg == "" || !strings.Contains(strings.ToLower(msg), tc.want) {
			t.Errorf("expected overlap error for %v, got: %q", tc.input, msg)
		}
	}
}

func TestValidator_RejectsInvalidCIDR(t *testing.T) {
	msg := runValidator(t, []string{"not-a-cidr"})
	if !strings.Contains(strings.ToLower(msg), "invalid cidr") {
		t.Errorf("expected invalid CIDR error, got: %q", msg)
	}
	msg = runValidator(t, []string{"10.0.0.0"}) // missing prefix length
	if !strings.Contains(strings.ToLower(msg), "invalid cidr") {
		t.Errorf("expected invalid CIDR error for missing prefix, got: %q", msg)
	}
}
