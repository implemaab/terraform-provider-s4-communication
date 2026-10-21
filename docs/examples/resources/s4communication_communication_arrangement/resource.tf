resource "s4communication_communication_arrangement" "example" {
  communication_scenario_id = "SAP_COM_0008"
  communication_system_id   = s4communication_communication_system.example.communication_system_id
  name                      = "EXT_API_SAP_COM_0008"
  inbound_user              = s4communication_inbound_user.example.id
  outbound_user             = s4communication_outbound_user.example.id

  # Arrangement properties: property name to values. Only listed properties
  # are managed.
  properties = {
    EXAMPLE_PROPERTY = ["value"]
  }

  # Services are created by SAP from the scenario and are keyed by their ID.
  # Only listed services are managed; unlisted attributes keep SAP's values.
  outbound_services = {
    "EXAMPLE_OUTBOUND_SERVICE" = {
      status   = "INACTIVE"
      url_path = "/api/v1"
    }
  }

  inbound_services = {
    "EXAMPLE_INBOUND_SERVICE" = {
      properties = {
        EXAMPLE_SERVICE_PROPERTY = ["value"]
      }
    }
  }
}
