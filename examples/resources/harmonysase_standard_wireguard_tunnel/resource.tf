resource "harmonysase_standard_wireguard_tunnel" "office" {
  network_id      = harmonysase_standard_network.main.id
  region_id       = harmonysase_standard_network.main.initial_region.region_id
  gateway_id      = harmonysase_standard_network.main.initial_region.gateway_ids[0]
  name            = "london-office"
  remote_endpoint = "203.0.113.10"
  remote_subnets  = ["10.10.0.0/16"]
}
