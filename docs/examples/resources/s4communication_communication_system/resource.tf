resource "s4communication_communication_system" "example" {
  communication_system_id = "EXT_API_SYSTEM"
  name                    = "External API System"
  description             = "External API integration"
  host_name               = "api.example.com"
  tcp_port                = 443

  owner_contact_person_name  = "Jane Doe"
  owner_contact_person_email = "jane.doe@example.com"
}
