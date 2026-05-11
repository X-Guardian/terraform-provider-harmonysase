// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
)

// errFromDiags collapses a non-empty Diagnostics into an error.
func errFromDiags(diags diag.Diagnostics) error {
	if !diags.HasError() {
		return nil
	}
	return fmt.Errorf("%s", diags.Errors())
}
