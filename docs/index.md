---
page_title: "Provider: ZeroTier"
description: |-
  The ZeroTier provider allows Terraform to manage ZeroTier networks and members.
---

# ZeroTier Provider

Manage ZeroTier networks and members using Terraform. Supports both ZeroTier self-hosted controllers.

## Example Usage
```terraform
provider "zerotier" {
  base_url      = "https://zt.example.com"
  controller_id = "abc123def456"
  api_token     = var.zerotier_api_token
}
```

## Authentication

The provider requires an API token which can be provided via:
- `api_token` attribute
- `ZEROTIER_API_TOKEN` environment variable

For self-hosted controllers, you also need:
- `controller_id` or `ZEROTIER_CONTROLLER_ID`
- `base_url` or `ZEROTIER_BASE_URL`

## Schema

### Optional

- `api_token` (String, Sensitive) API authentication token
- `base_url` (String) API base URL. 
- `controller_id` (String) Controller ID for self-hosted controllers