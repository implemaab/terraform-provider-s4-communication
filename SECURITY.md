# Security Policy

## Reporting a Vulnerability

Please do **not** report security vulnerabilities through public GitHub issues, discussions or pull requests.

Report them privately through GitHub's [private vulnerability reporting](https://github.com/implemaab/terraform-provider-s4communication/security/advisories/new) for this repository. Include:

- a description of the issue and its impact;
- the steps or configuration needed to reproduce it;
- the affected provider version or commit.

You will get a response as soon as possible. Please give the maintainers reasonable time to fix the issue before disclosing it publicly.

## Scope

Examples of issues to report this way:
- credentials, certificates or keys being written to logs, state or output;
- the provider connecting without verifying the server certificate;
- requests being sent to a host other than the configured one.

Vulnerabilities in SAP S/4HANA Cloud itself must be reported to SAP, not to this project.

## Handling Credentials

- Pass passwords and keys through variables marked `sensitive` or through environment variables, never as literals in configuration that is committed.
- Terraform state contains the passwords of communication users and outbound users that you configure. Store state in an encrypted backend with restricted access.
