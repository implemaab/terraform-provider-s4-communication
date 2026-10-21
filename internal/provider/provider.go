package provider

import (
	"context"
	"crypto/x509"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/list"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-s4-communication/internal/service/communicationarrangement"
	"terraform-provider-s4-communication/internal/service/communicationsystem"
	"terraform-provider-s4-communication/internal/service/communicationuser"
	"terraform-provider-s4-communication/internal/service/inbounduser"
	"terraform-provider-s4-communication/internal/service/outbounduser"
	s4types "terraform-provider-s4-communication/internal/types"
)

// Ensure S4CommunicationProvider satisfies various provider interfaces.
var _ provider.Provider = &S4CommunicationProvider{}
var _ provider.ProviderWithListResources = &S4CommunicationProvider{}

// S4CommunicationProvider defines the provider implementation.
type S4CommunicationProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string

	// rootCAs verifies the server certificate; nil uses the system roots.
	// Only set by tests that run against a TLS mock server.
	rootCAs *x509.CertPool
}

// S4CommunicationProviderModel describes the provider data model.
type S4CommunicationProviderModel struct {
	Host     types.String `tfsdk:"host"`
	Username types.String `tfsdk:"username"`
	Password types.String `tfsdk:"password"`

	ClientCertificate types.String `tfsdk:"client_certificate"`
	ClientKey         types.String `tfsdk:"client_key"`
}

func (p *S4CommunicationProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "s4communication"
	resp.Version = p.version
}

func (p *S4CommunicationProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "The S4 Communication provider manages SAP S/4HANA Cloud communication systems (with their inbound and outbound users), communication users and communication arrangements.",
		Attributes: map[string]schema.Attribute{
			"host": schema.StringAttribute{
				MarkdownDescription: "SAP S/4HANA Cloud API host, e.g. `my000000-api.s4hana.cloud.sap`. The provider connects over HTTPS on port 443. Can also be set with the `S4_HOST` environment variable.",
				Optional:            true,
			},
			"username": schema.StringAttribute{
				MarkdownDescription: "Communication user for basic authentication. Can also be set with the `S4_USERNAME` environment variable. Configure either `username` and `password`, or `client_certificate` and `client_key`.",
				Optional:            true,
				Sensitive:           true,
			},
			"password": schema.StringAttribute{
				MarkdownDescription: "Password for basic authentication. Can also be set with the `S4_PASSWORD` environment variable.",
				Optional:            true,
				Sensitive:           true,
			},
			"client_certificate": schema.StringAttribute{
				MarkdownDescription: "PEM encoded X.509 client certificate of the communication user, for authentication with mutual TLS. Intermediate certificates may follow it. Use `file(\"cert.pem\")` to read it from a file. Can also be set with the `S4_CLIENT_CERTIFICATE` environment variable, as PEM or as a file path.",
				Optional:            true,
			},
			"client_key": schema.StringAttribute{
				MarkdownDescription: "PEM encoded, unencrypted private key of `client_certificate`. Can also be set with the `S4_CLIENT_KEY` environment variable, as PEM or as a file path.",
				Optional:            true,
				Sensitive:           true,
			},
		},
	}
}

func (p *S4CommunicationProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data S4CommunicationProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Values not set in the configuration are read from the environment
	fromEnv := func(value *types.String, name string, pem bool) {
		if value.ValueString() != "" {
			return
		}
		env := os.Getenv(name)
		if pem {
			var err error
			if env, err = s4types.ReadPEM(env); err != nil {
				resp.Diagnostics.AddError("Invalid "+name+" Environment Variable", err.Error())
				return
			}
		}
		if env != "" {
			*value = types.StringValue(env)
		}
	}
	fromEnv(&data.Host, "S4_HOST", false)
	fromEnv(&data.Username, "S4_USERNAME", false)
	fromEnv(&data.Password, "S4_PASSWORD", false)
	fromEnv(&data.ClientCertificate, "S4_CLIENT_CERTIFICATE", true)
	fromEnv(&data.ClientKey, "S4_CLIENT_KEY", true)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.Host.ValueString() == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("host"),
			"Missing SAP S/4HANA Host Configuration",
			"Set the host value in the configuration or use the S4_HOST environment variable.",
		)
	}
	resp.Diagnostics.Append(validateAuthentication(data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	clientConfig := &s4types.ClientConfig{
		Host:              data.Host.ValueString(),
		Username:          data.Username.ValueString(),
		Password:          data.Password.ValueString(),
		ClientCertificate: data.ClientCertificate.ValueString(),
		ClientKey:         data.ClientKey.ValueString(),
		RootCAs:           p.rootCAs,
	}

	client, err := s4types.NewClient(clientConfig)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Create SAP S/4HANA API Client",
			err.Error(),
		)
		return
	}

	// Make the client available to the Configure methods of resources and
	// list resources.
	resp.DataSourceData = client
	resp.ResourceData = client
	resp.ListResourceData = client
}

// validateAuthentication checks that exactly one authentication method is
// fully configured: basic authentication or a client certificate.
func validateAuthentication(data S4CommunicationProviderModel) diag.Diagnostics {
	var diags diag.Diagnostics
	basic := data.Username.ValueString() != "" || data.Password.ValueString() != ""
	certificate := data.ClientCertificate.ValueString() != "" || data.ClientKey.ValueString() != ""

	switch {
	case basic && certificate:
		diags.AddError(
			"Conflicting SAP S/4HANA Authentication Configuration",
			"Configure either username and password, or client_certificate and client_key, not both. "+
				"Check the S4_USERNAME, S4_PASSWORD, S4_CLIENT_CERTIFICATE and S4_CLIENT_KEY environment variables as well.",
		)
	case certificate:
		if data.ClientCertificate.ValueString() == "" {
			diags.AddAttributeError(path.Root("client_certificate"), "Missing SAP S/4HANA Client Certificate",
				"client_key is set, so client_certificate is required. Set it in the configuration or use the S4_CLIENT_CERTIFICATE environment variable.")
		}
		if data.ClientKey.ValueString() == "" {
			diags.AddAttributeError(path.Root("client_key"), "Missing SAP S/4HANA Client Key",
				"client_certificate is set, so client_key is required. Set it in the configuration or use the S4_CLIENT_KEY environment variable.")
		}
	case basic:
		if data.Username.ValueString() == "" {
			diags.AddAttributeError(path.Root("username"), "Missing SAP S/4HANA Username",
				"password is set, so username is required. Set it in the configuration or use the S4_USERNAME environment variable.")
		}
		if data.Password.ValueString() == "" {
			diags.AddAttributeError(path.Root("password"), "Missing SAP S/4HANA Password",
				"username is set, so password is required. Set it in the configuration or use the S4_PASSWORD environment variable.")
		}
	default:
		diags.AddError(
			"Missing SAP S/4HANA Authentication Configuration",
			"Configure username and password (S4_USERNAME, S4_PASSWORD) for basic authentication, "+
				"or client_certificate and client_key (S4_CLIENT_CERTIFICATE, S4_CLIENT_KEY) for a client certificate.",
		)
	}
	return diags
}

func (p *S4CommunicationProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		communicationsystem.NewCommunicationSystemResource,
		communicationuser.NewCommunicationUserResource,
		communicationarrangement.NewCommunicationArrangementResource,
		inbounduser.NewInboundUserResource,
		outbounduser.NewOutboundUserResource,
	}
}

// ListResources supports terraform query (Terraform 1.14 and later).
func (p *S4CommunicationProvider) ListResources(ctx context.Context) []func() list.ListResource {
	return []func() list.ListResource{
		communicationsystem.NewCommunicationSystemListResource,
		communicationuser.NewCommunicationUserListResource,
		communicationarrangement.NewCommunicationArrangementListResource,
		inbounduser.NewInboundUserListResource,
		outbounduser.NewOutboundUserListResource,
	}
}

func (p *S4CommunicationProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		// Data sources will be added in a future version
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &S4CommunicationProvider{
			version: version,
		}
	}
}
