// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// nonOverlappingCIDRsValidator validates that no two CIDR strings in a Set
// overlap (i.e. share any IP address). Both IPv4 and IPv6 are accepted.
// Empty/null/unknown sets pass; the validator does not enforce element count.
//
// "Overlap" means: parse both as CIDRs and check whether either's network
// contains the other's network address. That covers exact equality,
// containment, and any partial overlap, since two CIDRs in the same address
// family overlap iff one contains the other's first address.
type nonOverlappingCIDRsValidator struct{}

// NonOverlappingCIDRs returns a Set validator that ensures all elements are
// valid CIDRs and that no two elements overlap.
func NonOverlappingCIDRs() validator.Set {
	return nonOverlappingCIDRsValidator{}
}

func (v nonOverlappingCIDRsValidator) Description(_ context.Context) string {
	return "ensures all elements are valid CIDRs and that no two elements overlap"
}

func (v nonOverlappingCIDRsValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v nonOverlappingCIDRsValidator) ValidateSet(ctx context.Context, req validator.SetRequest, resp *validator.SetResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	var raw []types.String
	resp.Diagnostics.Append(req.ConfigValue.ElementsAs(ctx, &raw, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	type parsed struct {
		input string
		ipnet *net.IPNet
	}
	var nets []parsed
	for _, s := range raw {
		if s.IsNull() || s.IsUnknown() {
			// Unknown values can't be checked at validate time; defer to apply.
			return
		}
		v := s.ValueString()
		_, n, err := net.ParseCIDR(v)
		if err != nil {
			resp.Diagnostics.AddAttributeError(
				req.Path,
				"Invalid CIDR",
				fmt.Sprintf("%q is not a valid CIDR: %s", v, err),
			)
			continue
		}
		nets = append(nets, parsed{input: v, ipnet: n})
	}
	if resp.Diagnostics.HasError() {
		return
	}

	// O(n^2) is fine — set sizes here are tiny (typically <10).
	// Sort first by string so the error messages are deterministic.
	sort.Slice(nets, func(i, j int) bool { return nets[i].input < nets[j].input })

	for i := 0; i < len(nets); i++ {
		for j := i + 1; j < len(nets); j++ {
			if cidrsOverlap(nets[i].ipnet, nets[j].ipnet) {
				resp.Diagnostics.AddAttributeError(
					req.Path,
					"Overlapping CIDRs",
					fmt.Sprintf("CIDRs %q and %q overlap; subnet entries must not share any IP addresses.",
						nets[i].input, nets[j].input),
				)
			}
		}
	}
}

// cidrsOverlap reports whether two networks share any IP address. Returns
// false if the two networks are in different address families.
func cidrsOverlap(a, b *net.IPNet) bool {
	if a == nil || b == nil {
		return false
	}
	// Different IP families never overlap.
	if (a.IP.To4() == nil) != (b.IP.To4() == nil) {
		return false
	}
	return a.Contains(b.IP) || b.Contains(a.IP)
}

// ensure attr import is kept; helps if future tweaks reference attr.Value.
var _ = attr.Value(types.StringNull())

// ensureNoSurprise is a small marker to silence unused-import linters during
// iteration; kept here so editors don't strip strings imports if someone
// adds a strings.* call later.
var _ = strings.TrimSpace
