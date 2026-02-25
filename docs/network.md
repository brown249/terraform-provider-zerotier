---
page_title: "zerotier_network Resource - zerotier"
description: |-
  Manages a ZeroTier network
---

# zerotier_network (Resource)

Manages a ZeroTier network with IP assignment pools and routes.

## Example Usage
```terraform
resource "zerotier_network" "example" {
  name        = "production-network"
  description = "Production environment"
  private     = false

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
```

## Schema

### Required

- `name` (String) Network name

### Optional

- `description` (String) Network description
- `private` (Boolean) Require authorization to join
- `assignment_pools` (List of Object) IP assignment pools
  - `start` (String) Starting IP address
  - `end` (String) Ending IP address
- `routes` (List of Object) Network routes
  - `target` (String) Route target CIDR
  - `via` (String) Gateway IP (optional)

### Read-Only

- `id` (String) Network ID