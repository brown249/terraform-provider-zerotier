# Terraform Provider for ZeroTier

Manage ZeroTier networks and members with Terraform. Supports self-hosted controllers.

## Installation
```hcl
terraform {
  required_providers {
    zerotier = {
      source  = "brown249/zerotier"
      version = "~> 0.1"
    }
  }
}
```

## Usage
```hcl
provider "zerotier" {
  base_url      = "https://zt.example.com"  # Self-hosted controller
  controller_id = "abc123def456"             # Your controller ID
  api_token     = var.zerotier_api_token    # API token
}

resource "zerotier_network" "lab" {
  name    = "lab-network"
  private = false

  assignment_pools = [
    {
      start = "10.147.20.1"
      end   = "10.147.20.254"
    }
  ]

  routes = [
    {
      target = "10.147.20.0/24"
    }
  ]
}

resource "zerotier_member" "server" {
  network_id         = zerotier_network.lab.id
  node_id            = "a1b2c3d4e5"
  name               = "server-1"
  authorized         = true
  ip_assignments     = ["10.147.20.10"]
  no_auto_assign_ips = true
}
```

## Documentation

Full documentation available at: [registry.terraform.io/providers/brown249/zerotier](https://registry.terraform.io/providers/brown249/zerotier/latest/docs)

## License

MIT