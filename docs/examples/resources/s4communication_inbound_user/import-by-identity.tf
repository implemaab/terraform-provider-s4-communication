# Terraform 1.12 and later: import with the resource identity, as listed by
# terraform query
import {
  to = s4communication_inbound_user.example
  identity = {
    id = "00000000-0000-0000-0000-000000000000"
  }
}
