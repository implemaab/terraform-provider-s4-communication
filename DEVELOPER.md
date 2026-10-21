# Development Setup

This guide describes how to build, test and change the provider. To use the provider, see the [Quick Start Guide](docs/guides/QUICKSTART.md).

## Prerequisites

- [Go](https://go.dev/doc/install) 1.25.8 or later
- [Terraform](https://developer.hashicorp.com/terraform/downloads) on your `PATH`. The tests and the documentation generator run it. Use 1.14 or later to also run the `terraform query` tests, which are skipped on older versions.
- `make` and [Git](https://git-scm.com/)

## Clone and Build

```bash
git clone https://github.com/implemaab/terraform-provider-s4-communication.git
cd terraform-provider-s4-communication

make build     # builds ./terraform-provider-s4communication(.exe)
make install   # installs it into $(go env GOPATH)/bin
```

## Configure the Terraform CLI

Point Terraform at your local build with a `dev_overrides` block in your Terraform CLI configuration (`~/.terraformrc` on Linux and macOS, `%APPDATA%\terraform.rc` on Windows):

```hcl
provider_installation {
  dev_overrides {
    # The directory containing the provider binary, e.g. $(go env GOPATH)/bin
    "sap-community/s4communication" = "/home/you/go/bin"
  }
  direct {}
}
```

With an override in place, skip `terraform init` and run `terraform plan` directly. Terraform looks for a binary named `terraform-provider-s4communication` in that directory. `make build` and `make install` use that name; a plain `go build` or `go install` names it after the Go module, and Terraform does not find it.

## Verify the Setup

```bash
make test
cd docs/examples/provider && terraform validate
```

## Project Structure

```
terraform-provider-s4-communication/
├── main.go                      # provider server entry point, go:generate for docs
├── cmd/s4export/                # s4export command
├── internal/
│   ├── provider/                # provider configuration, and the Terraform-level tests
│   ├── service/<resource>/      # one package per resource:
│   │   ├── <resource>_resource.go   #   schema and CRUD
│   │   └── list.go                  #   list resource for terraform query
│   ├── types/                   # HTTP client: services, CSRF, authentication, logging
│   ├── util/                    # API tag mapping, request helpers, schema and identity helpers
│   ├── exporter/                # configuration generation for s4export
│   └── acctest/sapmock/         # in-memory mock of the SAP APIs for tests
├── templates/guides/            # user guides, copied to docs/guides/ by make generate
└── docs/
    ├── index.md, resources/, list-resources/, guides/   # generated, do not edit by hand
    ├── api/                     # SAP's OpenAPI definitions of the APIs
    └── examples/                # provider, resources, list-resources and complete examples
```

## Mapping Attributes to the SAP API

Resource models map attributes to SAP properties with an `api` struct tag:

```go
HostName types.String `tfsdk:"host_name" api:"HostName"`
```

The helpers in `internal/util` use these tags to build create requests (`CreateBody`), PATCH requests with only the changed values (`UpdateBody`) and to read responses (`ApplyResponse`). Tag options:

| Option | Use for |
|---|---|
| `noupdate` | Properties missing from the API's update schema. Also add a `RequiresReplace` plan modifier. |
| `readonly` | Keys assigned by SAP, such as `UUID` |
| `writeonly` | Values SAP never returns, such as passwords |

Optional attributes use `util.OptionalString`, `util.OptionalBool` or `util.OptionalInt64`, which keep the value SAP assigns when the attribute is not configured. Attributes inside nested objects, such as map entries, use the `util.NestedOptional...` variants. After create and update, `util.ResolveUnknowns` fills the values SAP assigned while keeping the planned ones.

SAP's OpenAPI definitions in the repository root are the reference for property names and for which properties can be updated.

## Adding a Resource

1. Create `internal/service/<resource>/<resource>_resource.go`, using an existing resource as a template: model with `api` tags, schema, CRUD, `IdentitySchema` and `ImportState`.
2. Add `list.go` with a list resource for `terraform query`.
3. Register both in `internal/provider/provider.go` (`Resources` and `ListResources`).
4. Extend `internal/acctest/sapmock` with the entity set, following the API definition.
5. Add tests in `internal/provider`: create, update, import, and a query test.
6. Add the resource to `internal/exporter` if `s4export` should export it.
7. Add `docs/examples/resources/<type>/resource.tf`, `import.sh`, `import-by-identity.tf` and `docs/examples/list-resources/<type>/list-resource.tfquery.hcl`.
8. Run `make generate` and update the [CHANGELOG](CHANGELOG.md).

## Testing

```bash
make test                                                         # everything
go test -v -run TestCommunicationArrangement ./internal/provider/  # one test
```

The tests in `internal/provider` run real Terraform commands against `internal/acctest/sapmock`, an in-memory implementation of the SAP communication management APIs. The mock enforces the rules of the published API definitions:
- `PATCH` only for updates, with only updatable properties;
- CSRF tokens for writes;
- keys assigned by the server;
- properties SAP fills in itself;
- paging with `@odata.nextLink`;
- client certificate logon over TLS.

Every test step also checks that a second plan is empty, so configuration drift makes the tests fail.

The tests use the `terraform` binary on your `PATH`; set `TF_ACC_TERRAFORM_PATH` to use a different one.

The mock reflects the API definitions, not a recording of a real system. When you find a difference in SAP's behaviour, change the mock to match it and add a test.

## Documentation

`docs/` is generated from the schema descriptions and from `docs/examples/provider`, `docs/examples/resources` and `docs/examples/list-resources`; `docs/guides/` is copied from `templates/guides/`:

```bash
make generate
```

Never edit `docs/` by hand. Regenerate it whenever you change a schema description or an example; CI fails if `docs/` is out of date.

## List Resources and Resource Identity

Every resource has a resource identity with one attribute, `id`: the same key as its `id` attribute and import ID (`util.IdentitySchema`, `util.SetIdentity`). The list resources in each `list.go` use `util.ListAll` to read the entity set and `util.ListResults` to stream the results. When Terraform asks for the full resource, they build it with the same mapping as Read, and for arrangements with the same `importServices` snapshot as import. That way `terraform query -generate-config-out` produces configuration that imports without changes.

The provider's type name in its registry address (`sap-community/s4communication`) must match the `s4communication_` resource prefix. Terraform writes it into generated configuration as the `provider` of each resource.

## Exporter

`cmd/s4export` is a thin command around `internal/exporter`. The exporter:
1. lists every entity set (`util.ListAll`);
2. maps each entity into the resource's model with `util.ApplyResponse`;
3. writes the configurable attributes (`util.ConfigValues`) with `hclwrite`.

Arrangement services are written from `communicationarrangement.SnapshotServices`, the same function the resource uses on import, so exported configuration plans without changes. `TestExport` checks exactly that: it seeds the mock, exports, and runs Terraform on the result.

## Debugging

Run Terraform with `TF_LOG=DEBUG` to log every API request and response status. Credentials are never logged, and passwords in request bodies are masked.

## How to Commit

- Run `make fmt`, `make lint`, `make test` and `make generate` before committing.
- Write commit messages and pull request titles as [Conventional Commits](https://www.conventionalcommits.org/), for example `feat: add business partners to communication systems` or `fix: keep property order of outbound services`. Mark breaking changes with `!`, e.g. `feat!: ...`.
- Describe user-visible changes in the [CHANGELOG](CHANGELOG.md).
