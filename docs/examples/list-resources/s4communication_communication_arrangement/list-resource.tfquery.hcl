# List the arrangements of one communication system for a scenario.
# Generated configuration includes the arrangement's outbound services and the
# properties that have values, the same as an import.
list "s4communication_communication_arrangement" "ext_api" {
  provider = s4communication

  config {
    communication_system_id   = "EXT_API_SYSTEM"
    communication_scenario_id = "SAP_COM_0008"
  }
}
