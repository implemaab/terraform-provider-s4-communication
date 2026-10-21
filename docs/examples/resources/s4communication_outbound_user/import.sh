# Outbound users are imported by their UUID. The password is not returned by
# SAP and has to be set in the configuration again.
terraform import s4communication_outbound_user.example 00000000-0000-0000-0000-000000000000
