# Allows the communication user to call this system's inbound services
resource "s4communication_inbound_user" "example" {
  communication_system_uuid = s4communication_communication_system.example.id
  communication_user_id     = s4communication_communication_user.example.id
}
