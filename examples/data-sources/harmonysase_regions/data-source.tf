# Look up a single region by display_name.
data "harmonysase_regions" "london3" {
  display_name = "London 3"
}

# All regions in a country.
data "harmonysase_regions" "uk_all" {
  country_code = "GB"
}

# Wire into a network resource:
resource "harmonysase_standard_network" "main" {
  name   = "production-network"
  subnet = "10.255.0.0/16"

  initial_region = {
    harmony_sase_region_id = data.harmonysase_regions.london3.regions[0].id
    scale_units            = 1
  }
}
