data "harmonysase_standard_gateway" "lookup" {
  network_id = harmonysase_standard_network.main.id
  id         = "<gateway_id>"
}

output "gateway_ip" {
  value = data.harmonysase_standard_gateway.lookup.ip
}
