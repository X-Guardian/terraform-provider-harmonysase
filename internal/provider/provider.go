// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"os"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/X-Guardian/terraform-provider-harmonysase/internal/client"
)

var _ provider.Provider = &HarmonySaseProvider{}

// HarmonySaseProvider implements the Terraform provider for the Check Point
// Harmony SASE public API.
type HarmonySaseProvider struct {
	// version is set to the provider version on release, "dev" when built and
	// run locally, and "test" during acceptance testing.
	version string
}

// HarmonySaseProviderModel describes the provider configuration as exposed in
// HCL.
type HarmonySaseProviderModel struct {
	APIKey             types.String `tfsdk:"api_key"`
	Region             types.String `tfsdk:"region"`
	Endpoint           types.String `tfsdk:"endpoint"`
	RateLimitPerMinute types.Int64  `tfsdk:"rate_limit_per_minute"`
	RateLimitBurst     types.Int64  `tfsdk:"rate_limit_burst"`
}

func (p *HarmonySaseProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "harmonysase"
	resp.Version = p.version
}

func (p *HarmonySaseProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Provider for the Check Point Harmony SASE (formerly Perimeter81) public API.",
		Attributes: map[string]schema.Attribute{
			"api_key": schema.StringAttribute{
				MarkdownDescription: "Harmony SASE API key. Falls back to `HARMONYSASE_API_KEY`.",
				Optional:            true,
				Sensitive:           true,
			},
			"region": schema.StringAttribute{
				MarkdownDescription: "Harmony SASE deployment region. One of `us`, `eu`, `au`, `in`. Defaults to `us`. Falls back to `HARMONYSASE_REGION`.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("us", "eu", "au", "in"),
				},
			},
			"endpoint": schema.StringAttribute{
				MarkdownDescription: "Override the API host (used for testing/mocks). When set, both auth and REST traffic are sent to this host.",
				Optional:            true,
			},
			"rate_limit_per_minute": schema.Int64Attribute{
				MarkdownDescription: "Maximum API requests per minute. Defaults to 100, matching the documented Harmony SASE ceiling of 500 requests per 5-minute window per source IP. Lower this if running multiple processes from the same IP. Falls back to `HARMONYSASE_RATE_LIMIT_PER_MINUTE`.",
				Optional:            true,
				Validators: []validator.Int64{
					int64validator.AtLeast(1),
				},
			},
			"rate_limit_burst": schema.Int64Attribute{
				MarkdownDescription: "Token-bucket burst size for the rate limiter. Defaults to 10. Falls back to `HARMONYSASE_RATE_LIMIT_BURST`.",
				Optional:            true,
				Validators: []validator.Int64{
					int64validator.AtLeast(1),
				},
			},
		},
	}
}

func (p *HarmonySaseProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data HarmonySaseProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiKey := strings.TrimSpace(data.APIKey.ValueString())
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("HARMONYSASE_API_KEY"))
	}
	if apiKey == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_key"),
			"Missing API key",
			"Set provider.api_key or the HARMONYSASE_API_KEY environment variable.",
		)
		return
	}

	region := data.Region.ValueString()
	if region == "" {
		region = os.Getenv("HARMONYSASE_REGION")
	}

	rpm, diag := resolveIntAttr(data.RateLimitPerMinute, "HARMONYSASE_RATE_LIMIT_PER_MINUTE", "rate_limit_per_minute")
	if diag != nil {
		resp.Diagnostics.Append(diag)
		return
	}
	burst, diag := resolveIntAttr(data.RateLimitBurst, "HARMONYSASE_RATE_LIMIT_BURST", "rate_limit_burst")
	if diag != nil {
		resp.Diagnostics.Append(diag)
		return
	}

	c, err := client.New(client.Config{
		APIKey:             apiKey,
		Region:             client.Region(region),
		Endpoint:           data.Endpoint.ValueString(),
		UserAgent:          "terraform-provider-harmonysase/" + p.version,
		RateLimitPerMinute: rpm,
		RateLimitBurst:     burst,
	})
	if err != nil {
		resp.Diagnostics.AddError("Invalid provider configuration", err.Error())
		return
	}

	resp.DataSourceData = c
	resp.ResourceData = c
}

func (p *HarmonySaseProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewNetworkResource,
		NewGatewayResource,
		NewWireguardTunnelResource,
	}
}

func (p *HarmonySaseProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewGatewayDataSource,
		NewRegionsDataSource,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &HarmonySaseProvider{version: version}
	}
}
