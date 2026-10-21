# List all communication systems. Run "terraform query", or
# "terraform query -generate-config-out=generated.tf" to write configuration
# with import blocks for them.
list "s4communication_communication_system" "all" {
  provider = s4communication
}

# List a single communication system by its ID
list "s4communication_communication_system" "ext_api" {
  provider = s4communication

  config {
    communication_system_id = "EXT_API_SYSTEM"
  }
}
