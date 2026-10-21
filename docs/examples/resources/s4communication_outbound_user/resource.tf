# Credentials used when SAP calls the remote system
resource "s4communication_outbound_user" "example" {
  communication_system_uuid = s4communication_communication_system.example.id
  name                      = "remote-user"
  password                  = var.outbound_password
}

variable "outbound_password" {
  type      = string
  sensitive = true
}
