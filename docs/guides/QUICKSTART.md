# Quick Start Guide

## Introduction

The Terraform provider for SAP S/4HANA Cloud Communication Management automates the integration setup of an SAP S/4HANA Cloud system: communication systems with their inbound and outbound users, communication users, and communication arrangements with their services and properties.

This guide sets up access in SAP S/4HANA Cloud, builds and installs the provider, and creates a first communication system and arrangement.

## Prerequisites

- An SAP S/4HANA Cloud system, and a user with access to the communication management apps (*Maintain Communication Users*, *Communication Systems*, *Communication Arrangements*).
- [Terraform](https://developer.hashicorp.com/terraform/downloads) 1.0 or later. Importing by resource identity needs 1.12, and `terraform query` needs 1.14.
- [Go](https://go.dev/doc/install) 1.25.8 or later, to build the provider. The provider is not published in the Terraform Registry yet.
- [Git](https://git-scm.com/) and `make`.

## Step 1: Give Terraform Access to SAP S/4HANA Cloud

The provider calls the SAP S/4HANA Cloud communication management APIs (communication system, communication user and communication arrangement). Like any other API client, it needs its own communication user, communication system and arrangement in SAP:

1. **Communication user**: in *Maintain Communication Users*, create a user for Terraform, for example `TERRAFORM`. Give it a password, or upload an X.509 certificate if you want to use [certificate authentication](AUTHENTICATION.md#x509-client-certificate-mtls).
2. **Communication system**: in *Communication Systems*, create a system for the machine or pipeline that runs Terraform, for example `TERRAFORM`, and add the user from step 1 as an **inbound** user.
3. **Communication arrangement**: in *Communication Arrangements*, create an arrangement for the scenario **`SAP_COM_0A48`** (Communication Management Integration) with the system from step 2. SAP's API definitions of these services list this scenario (and `SAP_COM_0A31`, Cloud Integration Automation Integration) as the ones that give access to them.

The arrangement shows the API host to use, for example `my000000-api.s4hana.cloud.sap`. Use the host only, without `https://` or a path.

## Step 2: Build and Install the Provider

Clone the repository and install the provider binary into `$(go env GOPATH)/bin`:

```bash
git clone https://github.com/implemaab/terraform-provider-s4-communication.git
cd terraform-provider-s4-communication
make install
```

> Use `make install` or `make build`, not plain `go install`: Terraform only finds the binary under the name `terraform-provider-s4communication`, which the Makefile sets.

Then tell Terraform to use your local build with a `dev_overrides` block in the Terraform CLI configuration file: `~/.terraformrc` on Linux and macOS, `%APPDATA%\terraform.rc` on Windows.

```hcl
provider_installation {
  dev_overrides {
    # The directory that contains terraform-provider-s4communication,
    # i.e. the output of: go env GOPATH, followed by /bin
    "sap-community/s4communication" = "/home/<you>/go/bin"
  }

  # All other providers are installed from the registry as usual
  direct {}
}
```

With a development override, `terraform init` is not needed for this provider. Terraform prints a warning about the override on every run, which is expected.

## Step 3: Configure the Credentials

Keep the credentials out of your configuration by setting them as environment variables.

On Linux and macOS:

```bash
export S4_HOST=my000000-api.s4hana.cloud.sap
export S4_USERNAME=TERRAFORM
export S4_PASSWORD='<password>'
```

On Windows with PowerShell:

```powershell
$Env:S4_HOST = 'my000000-api.s4hana.cloud.sap'
$Env:S4_USERNAME = 'TERRAFORM'
$Env:S4_PASSWORD = '<password>'
```

On Windows with CMD:

```shell
set S4_HOST=my000000-api.s4hana.cloud.sap
set S4_USERNAME=TERRAFORM
set S4_PASSWORD=<password>
```

To authenticate with a client certificate instead, see [Authentication](AUTHENTICATION.md).

## Step 4: Create Your First Resources

Create a new directory with a file `main.tf`. This example creates a communication system for an external application, a communication user the application uses to call SAP, and an arrangement that grants that user access to a scenario:

```hcl
terraform {
  required_providers {
    s4communication = {
      source = "sap-community/s4communication"
    }
  }
}

# Configured with S4_HOST, S4_USERNAME and S4_PASSWORD
provider "s4communication" {}

variable "api_user_password" {
  type      = string
  sensitive = true
}

resource "s4communication_communication_system" "external_app" {
  communication_system_id = "EXTERNAL_APP"
  name                    = "External Application"
  host_name               = "app.example.com"
}

resource "s4communication_communication_user" "external_app" {
  name        = "EXTERNAL_APP"
  description = "Used by the external application to call SAP"
  password    = var.api_user_password
}

resource "s4communication_inbound_user" "external_app" {
  communication_system_uuid = s4communication_communication_system.external_app.id
  communication_user_id     = s4communication_communication_user.external_app.id
}

resource "s4communication_communication_arrangement" "external_app" {
  communication_scenario_id = "SAP_COM_0008" # replace with the scenario you need
  communication_system_id   = s4communication_communication_system.external_app.communication_system_id
  name                      = "EXTERNAL_APP_SAP_COM_0008"
  inbound_user              = s4communication_inbound_user.external_app.id

  # Outbound services are created by SAP from the scenario. Some scenarios
  # validate active services on creation, so switch off the ones you do not
  # use. The service IDs are listed in the scenario's documentation and in
  # the Communication Arrangements app.
  # outbound_services = {
  #   "<outbound service ID>" = {
  #     status = "inactive"
  #   }
  # }
}
```

Two different identifiers appear here: the arrangement takes the system's own ID (`communication_system_id`), while inbound and outbound users take the system's UUID (`id`).

Run Terraform:

```bash
terraform plan
terraform apply
```

Attributes you do not configure keep the values SAP assigns, and do not show up as changes later. See [Drift detection and managed attributes](DRIFTDETECTION.md) for details.

## Next Steps

- [Authentication](AUTHENTICATION.md) with an X.509 client certificate
- [Bring an existing setup under Terraform](IMPORT.md) with `terraform query` or `s4export`
- The [resource documentation](../) and the [examples](../examples/)
