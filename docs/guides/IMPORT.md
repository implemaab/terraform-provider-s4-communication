# Importing and Exporting Existing Configuration

## Overview

Communication systems, users and arrangements that already exist in SAP S/4HANA Cloud can be brought under Terraform management. There are three ways, from manual to fully generated:

| Way | Terraform version | You write | Generated configuration |
|---|---|---|---|
| [Import by ID or identity](#importing-single-resources) | 1.0 (`import` blocks 1.5, identity 1.12) | The resource configuration | None |
| [`terraform query`](#finding-resources-with-terraform-query) | 1.14 | A `.tfquery.hcl` file | All attributes, with literal IDs |
| [`s4export`](#exporting-a-system-with-s4export) | 1.5 | Nothing | Only values that are set, with references between resources |

Whichever way you choose, check that `terraform plan` shows only imports and no changes before you apply.

## Resource Overview

| Resource | Import ID and identity `id` | Not imported |
|---|---|---|
| `s4communication_communication_system` | UUID | |
| `s4communication_inbound_user` | UUID | |
| `s4communication_outbound_user` | UUID | `password` |
| `s4communication_communication_user` | ID assigned by SAP, e.g. `CC0000000001` (not the user name) | `password` |
| `s4communication_communication_arrangement` | UUID | |

Passwords are never returned by SAP. Leave `password` out of the configuration of imported users, or expect the first apply to set it to the value you configure.

Importing a communication arrangement also takes all of its outbound services and every property that has values into state, so that generated configuration matches. If your configuration lists fewer, the first plan shows an update that only drops them from state; nothing changes in SAP.

## Importing Single Resources

With the import ID:

```shell
terraform import s4communication_communication_system.example 00000000-0000-0000-0000-000000000000
```

With an `import` block (Terraform 1.5+):

```hcl
import {
  to = s4communication_communication_system.example
  id = "00000000-0000-0000-0000-000000000000"
}
```

With the resource identity (Terraform 1.12+), as returned by `terraform query`:

```hcl
import {
  to       = s4communication_communication_system.example
  identity = { id = "00000000-0000-0000-0000-000000000000" }
}
```

The UUIDs are shown in the Fiori apps, or can be listed with `terraform query` or `s4export`.

## Finding Resources with `terraform query`

Terraform 1.14 and later can list existing objects with list blocks in a `.tfquery.hcl` file next to your configuration:

```hcl
# query.tfquery.hcl
list "s4communication_communication_system" "all" {
  provider = s4communication
}

list "s4communication_communication_arrangement" "ext_api" {
  provider = s4communication
  config {
    communication_system_id   = "EXT_API_SYSTEM"
    communication_scenario_id = "SAP_COM_0008"
  }
}
```

```bash
terraform query                                    # list what exists
terraform query -generate-config-out=generated.tf  # write resources and import blocks
terraform plan                                     # should only import
```

Every resource type can be listed. The optional filters are:

| List resource | Filters |
|---|---|
| `s4communication_communication_system` | `communication_system_id` |
| `s4communication_communication_user` | `name` |
| `s4communication_inbound_user` | `communication_system_uuid` |
| `s4communication_outbound_user` | `communication_system_uuid` |
| `s4communication_communication_arrangement` | `communication_system_id`, `communication_scenario_id` |

A `limit` argument caps the number of results. See [docs/list-resources](../list-resources/) for details.

The generated configuration contains every attribute, including `false` and `0` values, and refers to other resources by literal ID. Review it and move the resources into your configuration.

## Exporting a System with `s4export`

`s4export` exports a whole system in one step. It writes configuration in which resources refer to each other (for example an inbound user to `s4communication_communication_system.<name>.id`), leaves out values that are empty in SAP, and adds import blocks.

```bash
cd terraform-provider-s4communication
export S4_HOST=my000000-api.s4hana.cloud.sap S4_USERNAME=... S4_PASSWORD=...
# or: export S4_CLIENT_CERTIFICATE=user.crt S4_CLIENT_KEY=user.key
go run ./cmd/s4export -out ../exported      # or: make export

cd ../exported
terraform plan    # should only import, with no changes
terraform apply
```

It writes `provider.tf`, `communication_systems.tf` (systems with their inbound and outbound users), `communication_users.tf`, `communication_arrangements.tf` and `imports.tf`. The import blocks can be deleted after the apply.

| Flag | Effect |
|---|---|
| `-systems REGEX` | Only export communication systems whose ID matches, with their users and arrangements |
| `-all-users` | Export every communication user, not only those used by an exported inbound user |
| `-out DIR` | Output directory (default `exported`) |
| `-force` | Overwrite existing files in the output directory |

Exported users contain a comment showing how to add their password, which SAP does not return.

`s4export` exports every communication system it finds, including any that SAP manages itself. Use `-systems` to limit the export to your own systems.
