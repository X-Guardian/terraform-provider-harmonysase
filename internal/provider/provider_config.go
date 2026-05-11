// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/X-Guardian/terraform-provider-harmonysase/internal/client"
)

// resolveIntAttr returns the int value of an Int64 attribute, falling back
// to envVar when the attribute is null/unknown, and returning 0 when neither
// is set (so the client can apply its default). The attrName is used only
// for the error diagnostic when the env var contains a non-integer value.
func resolveIntAttr(v types.Int64, envVar, attrName string) (int, *diag.ErrorDiagnostic) {
	if !v.IsNull() && !v.IsUnknown() {
		return int(v.ValueInt64()), nil
	}
	raw := strings.TrimSpace(os.Getenv(envVar))
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		d := diag.NewErrorDiagnostic(
			fmt.Sprintf("Invalid %s", envVar),
			fmt.Sprintf("Environment variable %s for provider attribute %q must be an integer; got %q.", envVar, attrName, raw),
		)
		return 0, &d
	}
	return n, nil
}

// clientFromProviderData type-asserts the provider's configured client out of
// the ResourceData/DataSourceData any value. It returns nil and appends a
// diagnostic on mismatch; callers should bail when nil.
func clientFromProviderData(providerData any, diags *diag.Diagnostics) *client.Client {
	if providerData == nil {
		return nil
	}
	c, ok := providerData.(*client.Client)
	if !ok {
		diags.AddError(
			"Unexpected provider data type",
			fmt.Sprintf("Expected *client.Client, got %T. This is a provider bug.", providerData),
		)
		return nil
	}
	return c
}
