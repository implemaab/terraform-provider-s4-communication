resource "s4communication_communication_user" "example" {
  name        = "EXT_API_USER"
  description = "User for the external API"
  password    = var.communication_user_password
}

variable "communication_user_password" {
  type      = string
  sensitive = true
}
