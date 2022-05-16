locals {
  admin_username = "azureuser"
  vmss = flatten([
    for role_key, group in var.vm_groups : [
      for vm_type_key, vm_type in group : [
        for region, count in vm_type : {
          vm_type = vm_type_key
          region  = region
          role    = role_key
          count   = count
        }
      ]
    ]
  ])
  regions = distinct(local.vmss.*.region)
}

data "azurerm_resource_group" "default" {
  name = "${var.prefix}-rg"
}

resource "azurerm_virtual_network" "agent" {
  count               = length(local.regions)
  name                = "${var.prefix}-vnet${count.index}"
  address_space       = ["10.0.${count.index}.0/24"]
  location            = local.regions[count.index]
  resource_group_name = data.azurerm_resource_group.default.name
}

resource "azurerm_subnet" "agent" {
  count                = length(local.regions)
  name                 = "${var.prefix}-subnet${count.index}"
  resource_group_name  = data.azurerm_resource_group.default.name
  virtual_network_name = azurerm_virtual_network.agent[count.index].name
  address_prefixes     = ["10.0.${count.index}.0/24"]
}

# resource "azurerm_public_ip_prefix" "agent" {
#   count               = length(local.regions)
#   name                = "${var.prefix}-publicipprefix${count.index}"
#   location            = local.vmss[count.index].region
#   resource_group_name = data.azurerm_resource_group.default.name

#   prefix_length = 32 - ceil(log(sum([for x in local.vmss : x.count if x.region == local.regions[count.index]]), 2))
# }

resource "azurerm_network_security_group" "agent" {
  count               = length(local.regions)
  name                = "${var.prefix}-nsg${count.index}"
  location            = local.vmss[count.index].region
  resource_group_name = data.azurerm_resource_group.default.name

  security_rule {
    name                       = "k3s"
    priority                   = 100
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "*"
    source_port_range          = "*"
    destination_port_range     = "*"
    source_address_prefix      = "*"
    destination_address_prefix = "*"
  }
}

data "cloudinit_config" "config" {
  gzip          = true
  base64_encode = true

  part {
    content      = templatefile(var.cloudconfig_file, { master_ip: file("../../ansible/master-ip.txt"), token: file("../../ansible/node-token") })
  }
}

resource "azurerm_linux_virtual_machine_scale_set" "agent" {
  count                = length(local.vmss)
  name                 = "${var.prefix}-${local.vmss[count.index].region}-${local.vmss[count.index].role}-vm"

  resource_group_name = data.azurerm_resource_group.default.name
  location            = local.vmss[count.index].region
  sku                 = local.vmss[count.index].vm_type
  instances           = local.vmss[count.index].count

  # priority        = local.vmss[count.index].region == "canadacentral" && local.vmss[count.index].role == "client" ? "Regular" : "Spot"
  # eviction_policy = local.vmss[count.index].region == "canadacentral" && local.vmss[count.index].role == "client" ? null : "Delete"
  priority        = "Spot"
  eviction_policy = "Delete"

  admin_username                  = local.admin_username
  disable_password_authentication = true
  admin_ssh_key {
    username   = local.admin_username
    public_key = file("~/.ssh/ssrg-balaji.pub")
  }

  source_image_id = "/subscriptions/850d15b7-9d0f-4e19-b4db-3f92b8b2a9b0/resourceGroups/destiny-rg/providers/Microsoft.Compute/galleries/destinySig/images/destinyImage/versions/1.0.0"
  custom_data = data.cloudinit_config.config.rendered

  os_disk {
    storage_account_type = "Standard_LRS"
    caching              = "ReadWrite"
  }

  network_interface {
    name                          = "${var.prefix}-nic${count.index}"
    enable_accelerated_networking = true
    network_security_group_id     = azurerm_network_security_group.agent[index(local.regions, local.vmss[count.index].region)].id
    primary = true

    ip_configuration {
      name      = "internal"
      subnet_id = azurerm_subnet.agent[index(local.regions, local.vmss[count.index].region)].id
      primary   = true

      public_ip_address {
        name                = "first"
        # public_ip_prefix_id = azurerm_public_ip_prefix.agent[index(local.regions, local.vmss[count.index].region)].id
      }
    }
  }
}