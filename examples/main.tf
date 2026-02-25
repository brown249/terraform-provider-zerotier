terraform {
  required_providers {
    zerotier = {
      source = "brown249/zerotier"
    }
  }
}

provider "zerotier" {
  base_url      = "url_here"
  controller_id = "controller_id_here"
  api_token     = var.zerotier_api_token
}

variable "zerotier_api_token" {
  type      = string
  sensitive = true
}

# Create network with IP pools and routes
resource "zerotier_network" "lab" {
  name        = "lab-network"
  description = "Lab environment with IP assignments"
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

output "network_id" {
  value = zerotier_network.lab.id
}

output "join_command" {
  value = "zerotier-cli join ${zerotier_network.lab.id}"
}