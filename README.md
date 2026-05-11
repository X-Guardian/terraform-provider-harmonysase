# Terraform Provider for Harmony SASE

[![GitHub release (latest by date)][release-badge]][releases]
[![Tests][tests-badge]][tests]
[![Go Report Card][report-badge]][report]
[![License: MPL-2.0][license-badge]][license]

The Harmony SASE Terraform provider lets you manage [Check Point Harmony SASE][harmonysase] (formerly Perimeter81) standard networks, gateways, and site-to-site tunnels via the [public API][api-docs].

## Requirements

- [Terraform][terraform] >= 1.0
- [Go][go] >= 1.26

## API Coverage

The Harmony SASE API splits into two generations: **Standard** networks and **Enhanced** networks. This provider currently covers Standard only.

## Supported Resources

### Networks
- `harmonysase_standard_network` — Manage a Standard network and its initial region (the seed gateway is part of the network; scale it via `initial_region.scale_units`).

### Gateways
- `harmonysase_standard_gateway` — Manage an additional gateway in any SASE region of a Standard network. If the requested region isn't already attached to the network, it is attached as part of create and detached as part of delete (when no other gateways remain in it).

### Tunnels
- `harmonysase_standard_wireguard_tunnel` — Manage a WireGuard site-to-site tunnel attached to a specific gateway in a Standard network.

## Supported Data Sources

- `harmonysase_standard_gateway` — Look up an existing gateway by network ID and gateway ID.

## Authentication

The provider exchanges a long-lived API key for a short-lived bearer JWT (60-minute lifetime, refreshed automatically). Generate the API key from the Harmony SASE admin UI.

```hcl
provider "harmonysase" {
  api_key = var.harmonysase_api_key
  region  = "eu" # one of: "us" (default), "eu", "au", "in"
}
```

Credentials can also be provided via environment variables:

- `HARMONYSASE_API_KEY`
- `HARMONYSASE_REGION` (optional, defaults to `us`)

## Usage

```hcl
terraform {
  required_providers {
    harmonysase = {
      source = "X-Guardian/harmonysase"
    }
  }
}

provider "harmonysase" {}

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

resource "harmonysase_standard_gateway" "secondary" {
  network_id             = harmonysase_standard_network.main.id
  harmony_sase_region_id = "eu-central-1"
}

resource "harmonysase_standard_wireguard_tunnel" "office" {
  network_id      = harmonysase_standard_network.main.id
  region_id       = harmonysase_standard_network.main.initial_region.region_id
  gateway_id      = harmonysase_standard_network.main.initial_region.gateway_ids[0]
  name            = "london-office"
  remote_endpoint = "203.0.113.10"
  remote_subnets  = ["10.10.0.0/16"]
}
```

## Building the Provider

1. Clone the repository
2. Enter the repository directory
3. Build the provider:

```shell
go install
```

## Developing the Provider

If you wish to work on the provider, you'll first need [Go][go] installed on your machine (see [Requirements](#requirements) above).

To compile the provider, run `go install`. This will build the provider and put the provider binary in the `$GOPATH/bin` directory.

To generate or update documentation, run `make generate`.

### Running Tests

Unit tests:

```shell
make test
```

[release-badge]: https://img.shields.io/github/v/release/X-Guardian/terraform-provider-harmonysase
[releases]: https://github.com/X-Guardian/terraform-provider-harmonysase/releases
[tests-badge]: https://github.com/X-Guardian/terraform-provider-harmonysase/actions/workflows/test.yml/badge.svg
[tests]: https://github.com/X-Guardian/terraform-provider-harmonysase/actions/workflows/test.yml
[report-badge]: https://goreportcard.com/badge/github.com/X-Guardian/terraform-provider-harmonysase
[report]: https://goreportcard.com/report/github.com/X-Guardian/terraform-provider-harmonysase
[license-badge]: https://img.shields.io/badge/License-MPL_2.0-yellow.svg
[license]: https://opensource.org/licenses/MPL-2.0
[harmonysase]: https://www.checkpoint.com/harmony/sase/
[api-docs]: https://app.swaggerhub.com/apis/Check-Point/Harmony-SASE-API/
[terraform]: https://developer.hashicorp.com/terraform/downloads
[go]: https://golang.org/doc/install
