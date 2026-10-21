# Changelog

## 1.0.0

First public release of the Terraform provider for SAP S/4HANA Cloud
Communication Management.

### Provider

- Provider address `sap-community/s4communication`, configured with `host`,
  `username` and `password`, or the `S4_HOST`, `S4_USERNAME` and `S4_PASSWORD`
  environment variables. The provider connects over HTTPS on port 443.
- Authentication with the communication user's X.509 client certificate
  (mutual TLS): `client_certificate` and `client_key` in the provider block,
  or `S4_CLIENT_CERTIFICATE` and `S4_CLIENT_KEY` (PEM or file path).
- Requires Terraform 1.0 or later. `terraform query` needs Terraform 1.14,
  import by resource identity needs Terraform 1.12. Building the provider
  requires Go 1.25.8 or later.

### Resources

- `s4communication_communication_system`: communication systems, with every
  attribute of the SAP API. `type` defaults to `customer_managed`.
- `s4communication_inbound_user` and `s4communication_outbound_user`: the
  users of a communication system. `authentication_method` defaults to
  `basic`.
- `s4communication_communication_user`: communication users. `type` defaults
  to `customer_communication`.
- `s4communication_communication_arrangement`: communication arrangements,
  including their `properties`, `inbound_services` and `outbound_services`.
  Only the services and properties listed in the configuration are managed;
  everything else is left as SAP created it. Configured outbound services are
  sent with the create request, so that a service configured as `inactive`
  is inactive from the start.
- Attributes that are not configured are set by SAP and kept as computed
  values, so they do not cause drift. Updates use `PATCH` and only send
  changed attributes.
- Changing a value SAP does not allow to be updated (the type of a
  communication user, the scenario of an arrangement, the system of an inbound
  or outbound user, or adding or removing an arrangement's users) replaces the
  resource.

### Import and export

- Import for all resources, by UUID or by resource identity
  (`import { identity = { id = "..." } }`). Importing an arrangement also
  imports its outbound services and every property that has values.
- `terraform query` support: list resources for all five resource types, with
  filters. `-generate-config-out` writes configuration that imports without
  changes.
- `s4export` (`go run ./cmd/s4export`) exports the communication systems,
  users and arrangements of a system as Terraform configuration with import
  blocks.

### Security

- Credentials, session cookies, CSRF tokens and passwords are never written to
  the output. Requests are logged at debug level with passwords masked.
- Modifying requests are serialized, because SAP binds the CSRF token to the
  session cookie.

### Documentation

- Reference documentation in `docs/`, generated from the schemas and
  examples, and guides for the quick start, authentication, import and
  export, and drift detection in `docs/guides/`.
