# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Terraform provider for SAP S/4HANA Cloud communication management, built with the Terraform Plugin Framework. Registry address `sap-community/s4communication`, resource prefix `s4communication_`. It manages communication systems (with inbound and outbound users), communication users and communication arrangements through SAP's OData V4 APIs. It also ships `s4export`, which exports an existing system as Terraform configuration.

The Go module is at the repository root. `docs/api/` holds SAP's OpenAPI definitions (`sap-s4-CE_APS_COM_*_ODATA_0001-v1.json`), the reference for property names and update rules, and a Bruno collection in `SAP API/`.

## Commands

```bash
make build      # builds ./terraform-provider-s4communication
make install    # installs it into $(go env GOPATH)/bin for dev_overrides
make test       # all tests; resource tests run Terraform against an in-memory SAP mock
make generate   # regenerates docs/ (tfplugindocs) and runs terraform fmt on examples
make lint       # go vet
go test -v -run TestCommunicationArrangement ./internal/provider/   # a single test
```

- The resource tests need a `terraform` binary on `PATH`, or in `TF_ACC_TERRAFORM_PATH`. Query tests need Terraform 1.14+ and identity import tests need 1.12+; they are skipped on older versions.
- Never edit the generated parts of `docs/` by hand (`index.md`, `resources/`, `list-resources/`, `guides/`). Change schema descriptions, `docs/examples/` or the guide sources in `templates/guides/`, then run `make generate`. CI fails if `docs/` is out of date.
- Do not use plain `go install` for the provider binary: Terraform only finds it as `terraform-provider-s4communication`.

## Architecture

- `internal/provider/`: provider schema and configuration (basic auth or client certificate), plus all Terraform-level tests.
- `internal/service/<resource>/`: one package per resource. `<resource>_resource.go` holds the schema and CRUD; `list.go` holds the list resource for `terraform query`.
- `internal/types/client.go`: the HTTP client. It routes requests to one of three OData services (`ServiceCommunicationSystem`, `ServiceCommunicationUser`, `ServiceCommunicationArrangement`). It fetches a CSRF token before each write, serialized because SAP binds tokens to the session. It also handles TLS client certificates.
- `internal/util/`: reflection over `api:"Name[,noupdate|readonly|writeonly]"` struct tags (`CreateBody`, `UpdateBody`, `ApplyResponse`, `ResolveUnknowns`, `ConfigValues`), plus request helpers, schema helpers, identity, and `ListAll` / `ListResults`.
- `internal/exporter/` and `cmd/s4export/`: s4export.
- `internal/acctest/sapmock/`: mock of the SAP APIs, enforcing the API definitions (PATCH only, update allowlists, CSRF, server-assigned keys, auto-filled properties, paging, mTLS).

## Conventions That Matter

- **Drift:** optional attributes are Optional+Computed with `UseStateForUnknown` (`util.OptionalString` etc.); attributes in nested objects use `util.NestedOptional...` (`UseNonNullStateForUnknown`). After create and update, keep the planned values and only fill unknowns from SAP (`util.ResolveUnknowns`). Do not rebuild state from the response.
- **Updates** are `PATCH` with only changed, updatable fields (`util.UpdateBody`). SAP does not support `PUT`.
- **Arrangement services and properties:** only configured entries are managed, and unlisted ones are left untouched. Import and list use `SnapshotServices` / `importServices`, the same definition the exporter writes, so generated configuration imports without changes.
- **Passwords** are `writeonly`: SAP never returns them.
- **New attributes or behaviour:** check the OpenAPI definitions, update the mock to match, and add a test. Every test step also asserts an empty second plan.
- **Changes:** describe user-visible changes in `CHANGELOG.md`.
- **Commits:** use Conventional Commits (`feat:`, `fix:`, `docs:`, `feat!:` for breaking changes).
- **Credentials:** never put credentials, certificates or real host names in code, examples or test data.
