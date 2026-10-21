![Golang](https://img.shields.io/badge/Go-1.25-informational)
![Terraform](https://img.shields.io/badge/Terraform-%3E%3D1.0-informational)
[![Tests](https://github.com/implemaab/terraform-provider-s4-communication/actions/workflows/test.yml/badge.svg)](https://github.com/implemaab/terraform-provider-s4-communication/actions/workflows/test.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

# Terraform Provider for SAP S/4HANA Cloud Communication Management

## About This Project

The Terraform provider for SAP S/4HANA Cloud Communication Management lets you manage the integration setup of an [SAP S/4HANA Cloud](https://www.sap.com/products/erp/s4hana.html) system with [Terraform](https://terraform.io/): communication systems, communication users and communication arrangements.

| Resource | Manages |
|---|---|
| `s4communication_communication_system` | Communication systems |
| `s4communication_inbound_user` | Inbound users of a communication system |
| `s4communication_outbound_user` | Outbound users of a communication system |
| `s4communication_communication_user` | Communication users |
| `s4communication_communication_arrangement` | Communication arrangements, including their inbound and outbound services and properties |

Every resource can be imported, listed with `terraform query` (Terraform 1.14+), and exported from an existing system with the bundled `s4export` tool.

> **Note**: This is a community project. It is not an official SAP product and is not published in the Terraform Registry yet; see the [Quick Start Guide](docs/guides/QUICKSTART.md) for how to build and use it locally.

## Usage of the Provider

Refer to the [Quick Start Guide](docs/guides/QUICKSTART.md) for setting up SAP S/4HANA Cloud, building the provider and creating your first resources.

```hcl
terraform {
  required_providers {
    s4communication = {
      source = "sap-community/s4communication"
    }
  }
}

provider "s4communication" {
  host     = "my000000-api.s4hana.cloud.sap"
  username = var.s4_username
  password = var.s4_password
}

resource "s4communication_communication_system" "example" {
  communication_system_id = "EXT_API_SYSTEM"
  name                    = "External API System"
  host_name               = "api.example.com"
}
```

The reference documentation for the provider and every resource is in [docs](docs/), and usage examples are in the [examples folder](docs/examples/).

## Additional Information and Guides

- [Quick Start Guide](docs/guides/QUICKSTART.md): SAP setup, installation and a first configuration
- [Authentication](docs/guides/AUTHENTICATION.md): password or X.509 client certificate (mTLS)
- [Importing and exporting existing configuration](docs/guides/IMPORT.md): `terraform import`, `terraform query` and `s4export`
- [Drift detection and managed attributes](docs/guides/DRIFTDETECTION.md): which values Terraform manages, and what SAP sets itself
- [Changelog](CHANGELOG.md), including breaking changes and how to migrate

## Repository Layout

| Path | Content |
|---|---|
| `main.go`, `internal/` | The provider (Go module) |
| [`cmd/s4export/`](cmd/s4export/) | The `s4export` tool |
| [`docs/`](docs/) | Reference documentation, with [`examples/`](docs/examples/) and [`guides/`](docs/guides/) |
| [`templates/guides/`](templates/guides/) | Sources of the guides, copied to `docs/guides/` by `make generate` |
| [`docs/api/`](docs/api/) | SAP's OpenAPI definitions of the communication management APIs the provider uses |

## Developing & Contributing to the Provider

The [developer documentation](DEVELOPER.md) describes how to build, test and develop the provider. For how to contribute, see the [Contribution Guidelines](CONTRIBUTING.md).

```bash
make install    # build and install the provider for a dev_overrides block
make test       # unit tests and Terraform tests against an SAP mock
make generate   # regenerate docs/ from the schemas and examples
make export     # export the system in S4_HOST to ./exported with s4export
```

## Support, Feedback, Contributing

🐞 - If you find a bug, please create a [bug report](https://github.com/implemaab/terraform-provider-s4-communication/issues/new?template=bug_report.yml).

💡 - If you have an idea for improvement or a feature request, please open a [feature request](https://github.com/implemaab/terraform-provider-s4-communication/issues/new?template=feature_request.yml).

> **Note**: If you believe you have found a security issue, please do not open a public issue. See the [security policy](SECURITY.md) for how to report it.

## Licensing

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

## OpenTofu Compatibility

The provider uses only standard Terraform plugin features and is expected to work with [OpenTofu](https://opentofu.org/), but it is developed and tested with the Terraform CLI only. `terraform query` and import by resource identity need Terraform 1.14 and 1.12 respectively.
