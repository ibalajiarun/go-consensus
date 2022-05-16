variable subscription_id {}
variable tenant_id {}
variable client_id {}
variable client_secret {}
variable vm_groups {
  // map of group => map of vm_type => map of region => count
  type = map(map(map(number)))
}
variable prefix {}
variable cloudconfig_file {}

provider "azurerm" {
  subscription_id = var.subscription_id
  tenant_id       = var.tenant_id
  client_id       = var.client_id
  client_secret   = var.client_secret

  features {}
}
