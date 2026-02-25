package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const (
	DefaultBaseURL = "https://api.zerotier.com/api/v1"
)

var _ provider.Provider = &ZeroTierProvider{}

type ZeroTierProvider struct {
	version string
}

type ZeroTierProviderModel struct {
	APIToken     types.String `tfsdk:"api_token"`
	BaseURL      types.String `tfsdk:"base_url"`
	ControllerID types.String `tfsdk:"controller_id"`
}

type ZeroTierClient struct {
	APIToken     string
	BaseURL      string
	ControllerID string
	HTTPClient   *http.Client
	IsSelfHosted bool
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &ZeroTierProvider{version: version}
	}
}

func (p *ZeroTierProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "zerotier"
	resp.Version = p.version
}

func (p *ZeroTierProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Interact with ZeroTier Central API or self-hosted controller.",
		Attributes: map[string]schema.Attribute{
			"api_token": schema.StringAttribute{
				Description: "ZeroTier API token. Can also be set via ZEROTIER_API_TOKEN environment variable.",
				Optional:    true,
				Sensitive:   true,
			},
			"base_url": schema.StringAttribute{
				Description: "ZeroTier API base URL. Can also be set via ZEROTIER_BASE_URL environment variable. " +
					"Defaults to " + DefaultBaseURL + " for ZeroTier Central. " +
					"For self-hosted controllers, use http://controller-ip:9993",
				Optional: true,
			},
			"controller_id": schema.StringAttribute{
				Description: "Controller ID for self-hosted controllers (10-digit hex). " +
					"Can also be set via ZEROTIER_CONTROLLER_ID environment variable. " +
					"Required for self-hosted controllers, not needed for ZeroTier Central.",
				Optional: true,
			},
		},
	}
}

func (p *ZeroTierProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config ZeroTierProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiToken := os.Getenv("ZEROTIER_API_TOKEN")
	if !config.APIToken.IsNull() {
		apiToken = config.APIToken.ValueString()
	}

	baseURL := os.Getenv("ZEROTIER_BASE_URL")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	if !config.BaseURL.IsNull() {
		baseURL = config.BaseURL.ValueString()
	}

	controllerID := os.Getenv("ZEROTIER_CONTROLLER_ID")
	if !config.ControllerID.IsNull() {
		controllerID = config.ControllerID.ValueString()
	}

	if apiToken == "" {
		resp.Diagnostics.AddError(
			"Missing API Token Configuration",
			"API token must be provided via ZEROTIER_API_TOKEN environment variable or provider configuration.",
		)
		return
	}

	isSelfHosted := !strings.Contains(baseURL, "api.zerotier.com")

	if isSelfHosted && controllerID == "" {
		resp.Diagnostics.AddError(
			"Missing Controller ID for Self-Hosted Controller",
			"When using a self-hosted ZeroTier controller, you must provide the controller_id via "+
				"ZEROTIER_CONTROLLER_ID environment variable or provider configuration. "+
				"Get your controller ID by running: sudo zerotier-cli info",
		)
		return
	}

	client := &ZeroTierClient{
		APIToken:     apiToken,
		BaseURL:      baseURL,
		ControllerID: controllerID,
		HTTPClient:   &http.Client{},
		IsSelfHosted: isSelfHosted,
	}

	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *ZeroTierProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewNetworkResource,
		NewMemberResource,
	}
}

func (p *ZeroTierProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{}
}

func (c *ZeroTierClient) doRequest(method, path string, body interface{}) (*http.Response, error) {
	var buf *bytes.Buffer
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		buf = bytes.NewBuffer(b)
	} else {
		buf = bytes.NewBuffer(nil)
	}

	url := c.BaseURL + path
	req, err := http.NewRequest(method, url, buf)
	if err != nil {
		return nil, err
	}

	if c.IsSelfHosted {
		req.Header.Set("X-ZT1-Auth", c.APIToken)
	} else {
		req.Header.Set("Authorization", "token "+c.APIToken)
	}
	req.Header.Set("Content-Type", "application/json")

	return c.HTTPClient.Do(req)
}
