resource "harmonysase_standard_gateway" "secondary" {
  network_id             = harmonysase_standard_network.main.id
  harmony_sase_region_id = "eu-central-1"
}
