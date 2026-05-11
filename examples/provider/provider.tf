terraform {
  required_providers {
    harmonysase = {
      source = "X-Guardian/harmonysase"
    }
  }
}

# Configure the Harmony SASE provider. The api_key may also be set via the
# HARMONYSASE_API_KEY environment variable; region via HARMONYSASE_REGION.
provider "harmonysase" {
  api_key = var.harmonysase_api_key
  region  = "eu" # one of: us (default), eu, au, in
}

variable "harmonysase_api_key" {
  type      = string
  sensitive = true
}
