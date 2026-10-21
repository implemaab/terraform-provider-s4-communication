# Complete example: a communication system for an external application, a
# communication user it uses to call SAP, and an arrangement for a scenario.

terraform {
  required_providers {
    s4communication = {
      source = "implemaab/s4communication"
    }
  }
}

# Configured through the S4_HOST, S4_USERNAME and S4_PASSWORD environment variables
provider "s4communication" {}

variable "communication_user_password" {
  type      = string
  sensitive = true
}

variable "outbound_password" {
  type      = string
  sensitive = true
}

resource "s4communication_communication_system" "external_api" {
  communication_system_id = "EXT_API_SYSTEM"
  name                    = "External API System"
  description             = "External API integration"
  host_name               = "api.external-service.com"
  tcp_port                = 443

  owner_contact_person_name  = "Jane Doe"
  owner_contact_person_email = "jane.doe@example.com"
}

resource "s4communication_communication_user" "api_user" {
  name        = "EXT_API_USER"
  description = "User for the external API"
  password    = var.communication_user_password
}

resource "s4communication_inbound_user" "api_user" {
  communication_system_uuid = s4communication_communication_system.external_api.id
  communication_user_id     = s4communication_communication_user.api_user.id
}

resource "s4communication_outbound_user" "remote" {
  communication_system_uuid = s4communication_communication_system.external_api.id
  name                      = "remote-user"
  password                  = var.outbound_password
}

resource "s4communication_communication_arrangement" "api_integration" {
  communication_scenario_id = "SAP_COM_0008" # Replace with the scenario you need
  communication_system_id   = s4communication_communication_system.external_api.communication_system_id
  name                      = "EXT_API_SAP_COM_0008"
  inbound_user              = s4communication_inbound_user.api_user.id
  outbound_user             = s4communication_outbound_user.remote.id

  # Outbound services are created by SAP from the scenario; list the ones to
  # configure, keyed by their service ID (see the scenario's documentation)
  outbound_services = {
    "EXAMPLE_OUTBOUND_SERVICE" = {
      url_path = "/api/v1"
    }
  }
}

output "communication_system_uuid" {
  value = s4communication_communication_system.external_api.id
}

output "communication_user_id" {
  value = s4communication_communication_user.api_user.id
}
