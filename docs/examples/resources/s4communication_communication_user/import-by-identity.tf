# Terraform 1.12 and later: import with the resource identity, as listed by
# terraform query
import {
  to = s4communication_communication_user.example
  identity = {
    id = "CC0000000001"
  }
}
