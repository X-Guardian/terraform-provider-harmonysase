// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
)

func TestErrFromDiags(t *testing.T) {
	if err := errFromDiags(diag.Diagnostics{}); err != nil {
		t.Errorf("empty diags: got %v, want nil", err)
	}

	warnOnly := diag.Diagnostics{}
	warnOnly.AddWarning("just a warning", "details")
	if err := errFromDiags(warnOnly); err != nil {
		t.Errorf("warning-only diags: got %v, want nil", err)
	}

	withErr := diag.Diagnostics{}
	withErr.AddError("boom", "details")
	err := errFromDiags(withErr)
	if err == nil {
		t.Fatal("expected error for diags with error severity, got nil")
	}
	if got := err.Error(); got == "" {
		t.Errorf("expected non-empty error message, got %q", got)
	}
}
