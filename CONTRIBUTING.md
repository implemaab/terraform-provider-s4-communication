# Contributing

## General Remarks

You are welcome to contribute code, documentation, examples and bug reports to this project.

1. By contributing, you agree that your contributions are licensed under the project's [license](LICENSE), the MIT License.
2. **Discuss larger changes first.** Bug fixes are always welcome. For a new resource, a new attribute group or a change in behaviour, please open a [feature request](https://github.com/implemaab/terraform-provider-s4communication/issues/new?template=feature_request.yml) first, and say that you intend to implement it. That avoids work that does not fit the direction of the project.
3. **Never include credentials** in issues, pull requests, examples or test data. This includes host names of real systems, passwords, certificates and keys.

## Reporting Bugs

Open a [bug report](https://github.com/implemaab/terraform-provider-s4communication/issues/new?template=bug_report.yml) with:
- the provider and Terraform versions;
- a minimal configuration that reproduces the problem;
- the error message.

For problems with the SAP API, run Terraform with `TF_LOG=DEBUG` and include the relevant request and response lines. Credentials are never logged, but check the output for system-specific data before posting it.

Security issues must not be reported as public issues; see the [security policy](SECURITY.md).

## How to Contribute

1. Make sure the change is welcome (see [General Remarks](#general-remarks)).
2. Fork the repository and create a branch for your change.
3. Follow the [developer documentation](DEVELOPER.md): run `make fmt`, `make lint`, `make test` and `make generate`, and update the [CHANGELOG](CHANGELOG.md) for user-visible changes.
4. Use [Conventional Commits](https://www.conventionalcommits.org/) for commit messages and the pull request title, e.g. `feat: ...`, `fix: ...`, `docs: ...`, and `feat!: ...` for breaking changes.
5. Open a pull request and fill in the template.
6. Address review comments. A maintainer merges the pull request once CI is green and it is approved.

## Contributions Made with AI Tools

AI-assisted contributions are welcome under the same rules as any other contribution:
- you are responsible for the code you submit, and must understand it;
- it must be tested;
- it must not contain material you are not allowed to contribute.

Mention in the pull request if a substantial part was generated.

## Testing Against a Real System

The automated tests run against an in-memory mock of the SAP APIs and need no SAP system. If you verified a change against a real SAP S/4HANA Cloud system, say so in the pull request and describe what you tested. If SAP behaves differently from the mock, please update the mock as part of the change.
