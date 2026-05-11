resource "harmonysase_standard_network" "main" {
  name   = "production-network"
  subnet = "10.255.0.0/16"
  tags   = ["prod"]

  initial_region = {
    harmony_sase_region_id = "eu-west-1"
    scale_units            = 1
    idle                   = false
  }
}
