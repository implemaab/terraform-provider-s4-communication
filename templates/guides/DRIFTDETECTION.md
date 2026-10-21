# Drift Detection and Managed Attributes

## Overview

Terraform compares your configuration with what SAP returns on every plan. A difference is reported as drift. This guide explains which values the provider manages, which ones SAP sets itself, and what is deliberately left alone.

## Values SAP Sets Itself

Most optional attributes are *optional and computed*: if you do not configure them, Terraform keeps whatever SAP assigns, and later plans do not show a change for them. Configure an attribute only when you want Terraform to enforce its value.

A few attributes have defaults, because SAP rejects the create request without them:

| Resource | Attribute | Default |
|---|---|---|
| `s4communication_communication_system` | `type` | `customer_managed` |
| `s4communication_communication_user` | `type` | `customer_communication` |
| `s4communication_inbound_user` | `authentication_method` | `basic` |
| `s4communication_outbound_user` | `authentication_method` | `basic` |

## Changes Made Outside Terraform

| Change in SAP | Next plan |
|---|---|
| A configured attribute is changed | Shown as drift; `apply` changes it back |
| An attribute you did not configure is changed | Not shown; the new value is kept |
| An object is deleted | The resource is planned to be created again |
| A password is changed | Not detected, because SAP does not return passwords |

## Arrangement Services and Properties

SAP creates the inbound and outbound services and the properties of an arrangement from its communication scenario. The provider only manages what you list:

- **Outbound services** in `outbound_services` are matched by service ID. Their configured attributes (`status`, `url_path`, `tcp_port`, job settings) are kept as configured; the others keep SAP's values. Services that are not listed are left alone.
- **Properties** in `properties` and in each service's `properties` are sets of values. Properties that are not listed keep their values. Removing a property from the configuration stops managing it without changing it in SAP; set it to `[]` to remove its values.
- **Inbound services** cannot be changed in SAP. Only their property values can be managed.
- Services cannot be removed from an arrangement. A service you list that the scenario did not create is added.
- Configured outbound services are sent with the create request, because SAP validates the scenario's services when the arrangement is created. For example, `SAP_COM_0008` requires replication settings on active services, so a service you configure as `inactive` is created inactive.

## Changes That Replace a Resource

Some values cannot be changed through SAP's update APIs. Changing them in the configuration destroys the object and creates a new one:

| Resource | Attribute |
|---|---|
| `s4communication_communication_user` | `type` |
| `s4communication_inbound_user`, `s4communication_outbound_user` | `communication_system_uuid` |
| `s4communication_communication_arrangement` | `communication_scenario_id` |
| `s4communication_communication_arrangement` | Adding or removing `inbound_user` or `outbound_user`; switching to another user is an in-place update |

Terraform shows these as `must be replaced` in the plan.

## Not Managed

- Additional OAuth scopes of an arrangement's outbound user.
- Business partners of a communication system.
- Certificates of communication users (other than using one to authenticate the provider).
