---
page_title: "zerotier_member Resource - zerotier"
description: |-
  Manages a ZeroTier network member
---

# zerotier_member (Resource)

Authorizes and configures a ZeroTier network member.

## Example Usage
```terraform
resource "zerotier_member" "server" {
  network_id         = zerotier_network.example.id
  node_id            = "a1b2c3d4e5"
  name               = "web-server-1"
  authorized         = true
  ip_assignments     = ["10.147.20.10"]
  no_auto_assign_ips = true
}
```

## Schema

### Required

- `network_id` (String) Network ID
- `node_id` (String) ZeroTier node ID

### Optional

- `name` (String) Member name
- `description` (String) Member description
- `authorized` (Boolean) Authorize member
- `ip_assignments` (List of String) Assigned IP addresses
- `no_auto_assign_ips` (Boolean) Disable auto IP assignment

### Read-Only

- `id` (String) Member ID