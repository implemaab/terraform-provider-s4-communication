terraform {
  required_providers {
    s4communication = {
      source = "implemaab/s4communication"
    }
  }
}

# Basic authentication with a communication user. The settings can also be
# given with the S4_HOST, S4_USERNAME and S4_PASSWORD environment variables.
provider "s4communication" {
  host     = "my000000-api.s4hana.cloud.sap"
  username = var.s4_username
  password = var.s4_password
}

# Alternatively, authenticate with the communication user's X.509 client
# certificate (mutual TLS). S4_CLIENT_CERTIFICATE and S4_CLIENT_KEY accept the
# PEM content or a file path.
#
# provider "s4communication" {
#   host               = "my000000-api.s4hana.cloud.sap"
#   client_certificate = file("communication-user.crt")
#   client_key         = file("communication-user.key")
# }

variable "s4_username" {
  type = string
}

variable "s4_password" {
  type      = string
  sensitive = true
}
