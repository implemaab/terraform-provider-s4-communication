# List all communication users. SAP does not return passwords, so generated
# configuration contains none.
list "s4communication_communication_user" "all" {
  provider = s4communication
}
