// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/X-Guardian/terraform-provider-harmonysase/internal/client"
)

func TestOldestRegion(t *testing.T) {
	if got := oldestRegion(nil); got != nil {
		t.Errorf("nil input: got %+v want nil", got)
	}

	regions := []client.NetworkRegion{
		{
			ID:        "r-newer",
			CreatedAt: "2026-04-01T00:00:00Z",
			Instances: []client.NetworkInstance{
				{ID: "i-2", CreatedAt: "2026-04-02T00:00:00Z"},
			},
		},
		{
			ID:        "r-older",
			CreatedAt: "2026-03-01T00:00:00Z",
			Instances: []client.NetworkInstance{
				{ID: "i-1", CreatedAt: "2026-01-15T00:00:00Z"},
				{ID: "i-3", CreatedAt: "2026-02-01T00:00:00Z"},
			},
		},
	}
	got := oldestRegion(regions)
	if got == nil || got.ID != "r-older" {
		t.Errorf("expected r-older, got %+v", got)
	}

	// Region with no instances should fall back to its own createdAt.
	regions = []client.NetworkRegion{
		{ID: "r-a", CreatedAt: "2026-05-01T00:00:00Z"},
		{ID: "r-b", CreatedAt: "2026-04-01T00:00:00Z"},
	}
	got = oldestRegion(regions)
	if got == nil || got.ID != "r-b" {
		t.Errorf("expected r-b, got %+v", got)
	}
}
