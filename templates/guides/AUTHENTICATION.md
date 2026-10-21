# Authentication

The provider authenticates as an SAP S/4HANA Cloud communication user (see [Step 1 of the Quick Start Guide](QUICKSTART.md#step-1-give-terraform-access-to-sap-s4hana-cloud)). SAP S/4HANA Cloud accepts two methods for such technical users, and the provider supports both:

1. [User name and password](#user-name-and-password) (basic authentication)
2. [X.509 client certificate](#x509-client-certificate-mtls) (mutual TLS)

Configure exactly one of them. If settings for both are found, in the configuration or in the environment, the provider stops with an error rather than guessing.

OAuth 2.0 client credentials are not available for these APIs: SAP's API definitions of the communication management services only declare basic authentication, and certificate logon is the alternative SAP S/4HANA Cloud offers for communication users.

The provider always connects over HTTPS on port 443.

## User Name and Password

```hcl
provider "s4communication" {
  host     = "my000000-api.s4hana.cloud.sap"
  username = var.s4_username
  password = var.s4_password
}
```

Or leave the provider block empty and set environment variables:

| Attribute | Environment variable |
|---|---|
| `host` | `S4_HOST` |
| `username` | `S4_USERNAME` |
| `password` | `S4_PASSWORD` |

```bash
# Linux and macOS
export S4_HOST=my000000-api.s4hana.cloud.sap
export S4_USERNAME=TERRAFORM
export S4_PASSWORD='<password>'
```

```powershell
# Windows PowerShell
$Env:S4_HOST = 'my000000-api.s4hana.cloud.sap'
$Env:S4_USERNAME = 'TERRAFORM'
$Env:S4_PASSWORD = '<password>'
```

## X.509 Client Certificate (mTLS)

The communication user can log on with a client certificate instead of a password.

### Prepare the Certificate in SAP

In *Maintain Communication Users*, upload the certificate (the public part) to the communication user Terraform uses. The user still needs to be the inbound user of a communication system with a `SAP_COM_0A48` arrangement, as described in the [Quick Start Guide](QUICKSTART.md#step-1-give-terraform-access-to-sap-s4hana-cloud).

### Configure the Provider

```hcl
provider "s4communication" {
  host               = "my000000-api.s4hana.cloud.sap"
  client_certificate = file("communication-user.crt")
  client_key         = file("communication-user.key")
}
```

| Attribute | Environment variable | Content |
|---|---|---|
| `client_certificate` | `S4_CLIENT_CERTIFICATE` | PEM certificate, optionally followed by its intermediate certificates |
| `client_key` | `S4_CLIENT_KEY` | Unencrypted PEM private key (RSA or EC) |

In the configuration, the attributes take the PEM content; use `file()` to read it from a file. The environment variables accept either the PEM content or a path to a file, which is easier than putting multi-line PEM into a variable:

```bash
export S4_HOST=my000000-api.s4hana.cloud.sap
export S4_CLIENT_CERTIFICATE=$HOME/certs/communication-user.crt
export S4_CLIENT_KEY=$HOME/certs/communication-user.key
```

No password is sent when a certificate is used.

### Converting a PKCS#12 File

Certificates are often handed out as a PKCS#12 file (`.p12` or `.pfx`). Convert it to PEM with OpenSSL:

```bash
openssl pkcs12 -in user.p12 -clcerts -nokeys -out communication-user.crt
openssl pkcs12 -in user.p12 -nocerts -nodes  -out communication-user.key
```

The second command writes the private key unencrypted; protect the file accordingly.

## The Same Settings for `s4export`

The [`s4export`](IMPORT.md#exporting-a-system-with-s4export) tool reads the same environment variables: `S4_HOST` with either `S4_USERNAME` and `S4_PASSWORD`, or `S4_CLIENT_CERTIFICATE` and `S4_CLIENT_KEY`.

## Troubleshooting

- **`authentication failed (status 401)`**: SAP rejected the credentials. Check the password or certificate, and that the communication user is the inbound user of a communication system with a `SAP_COM_0A48` arrangement.
- **`invalid client certificate or key`**: the certificate or key could not be read. Both must be PEM, the key must be unencrypted, and it must belong to the certificate.
- **Certificate logon fails although the certificate is correct**: confirm in SAP that this certificate is uploaded to the same communication user, and that certificate logon works for your API host.
- Run Terraform with `TF_LOG=DEBUG` to see each request and response status. Credentials are never logged, and passwords in request bodies are masked.
