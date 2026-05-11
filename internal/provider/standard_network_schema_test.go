// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/X-Guardian/terraform-provider-harmonysase/internal/client"
)

func TestBuildRegionsAttr_Empty(t *testing.T) {
	got, err := buildRegionsAttr(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.IsNull() {
		t.Errorf("expected non-null empty map, got null")
	}
	if len(got.Elements()) != 0 {
		t.Errorf("expected 0 elements, got %d", len(got.Elements()))
	}
}

func TestBuildRegionsAttr_KeyedByName(t *testing.T) {
	regions := []client.NetworkRegion{
		{
			ID:   "r-1",
			Name: "us-east-1",
			Instances: []client.NetworkInstance{
				{ID: "i-1", IP: "1.1.1.1"},
			},
		},
		{
			ID:   "r-2",
			Name: "eu-west-1",
			Instances: []client.NetworkInstance{
				{ID: "i-2", IP: "2.2.2.2"},
			},
		},
	}
	got, err := buildRegionsAttr(regions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	elems := got.Elements()
	if len(elems) != 2 {
		t.Fatalf("expected 2 regions, got %d", len(elems))
	}
	if _, ok := elems["us-east-1"]; !ok {
		t.Errorf("missing region keyed by name us-east-1: %v", elems)
	}
	if _, ok := elems["eu-west-1"]; !ok {
		t.Errorf("missing region keyed by name eu-west-1: %v", elems)
	}
}

func TestBuildRegionsAttr_GatewaysSortedByID(t *testing.T) {
	regions := []client.NetworkRegion{
		{
			ID:   "r-1",
			Name: "us-east-1",
			Instances: []client.NetworkInstance{
				{ID: "i-c"},
				{ID: "i-a"},
				{ID: "i-b"},
			},
		},
	}
	got, err := buildRegionsAttr(regions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Round-trip through string form is the simplest way to verify ordering
	// without unpacking the nested attr.Value structure.
	s := got.String()
	posA := indexOf(s, `"id":"i-a"`)
	posB := indexOf(s, `"id":"i-b"`)
	posC := indexOf(s, `"id":"i-c"`)
	if posA < 0 || posB < 0 || posC < 0 {
		t.Fatalf("expected all gateway IDs in output, got %s", s)
	}
	if posA >= posB || posB >= posC {
		t.Errorf("expected gateways sorted by id (i-a < i-b < i-c), got positions %d, %d, %d in %s", posA, posB, posC, s)
	}
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
