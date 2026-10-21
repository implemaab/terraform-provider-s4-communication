# List the inbound users of one communication system
list "s4communication_inbound_user" "ext_api" {
  provider = s4communication

  config {
    communication_system_uuid = "00000000-0000-0000-0000-000000000000"
  }
}
