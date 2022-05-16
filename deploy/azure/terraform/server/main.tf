locals {
  admin_username = "azureuser"
}

data "azurerm_resource_group" "default" {
  name = "${var.prefix}-rg"
}

resource "azurerm_virtual_network" "master" {
  name                = "${var.prefix}-vnet-master"
  address_space       = ["10.0.100.0/24"]
  location            = var.region
  resource_group_name = data.azurerm_resource_group.default.name
}

resource "azurerm_subnet" "master" {
  name                 = "${var.prefix}-subnet-master"
  resource_group_name  = data.azurerm_resource_group.default.name
  virtual_network_name = azurerm_virtual_network.master.name
  address_prefixes     = ["10.0.100.0/24"]
}

resource "azurerm_public_ip" "master" {
  name                = "${var.prefix}-publicip-master"
  location            = var.region
  resource_group_name = data.azurerm_resource_group.default.name
  allocation_method   = "Static"
}

resource "azurerm_network_security_group" "master" {
  name                = "${var.prefix}-nsg-master"
  location            = var.region
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

resource "azurerm_network_interface" "master" {
  name                          = "${var.prefix}-nic-master"
  location                      = var.region
  resource_group_name           = data.azurerm_resource_group.default.name
  enable_accelerated_networking = true

  ip_configuration {
    name                          = "nic-ip"
    subnet_id                     = azurerm_subnet.master.id
    private_ip_address_allocation = "Dynamic"
    public_ip_address_id          = azurerm_public_ip.master.id
  }
}

resource "azurerm_network_interface_security_group_association" "master" {
  network_interface_id      = azurerm_network_interface.master.id
  network_security_group_id = azurerm_network_security_group.master.id
}

resource "azurerm_linux_virtual_machine" "master" {
  name                  = "${var.prefix}-${var.region}-vmmaster"
  location              = var.region
  resource_group_name   = data.azurerm_resource_group.default.name
  network_interface_ids = [azurerm_network_interface.master.id]
  size                  = var.vm_type

  priority              = "Spot"
  eviction_policy       = "Deallocate"

  admin_username                  = local.admin_username
  disable_password_authentication = true
  admin_ssh_key {
    username   = local.admin_username
    public_key = file("~/.ssh/ssrg-balaji.pub")
  }

  source_image_id = "/subscriptions/850d15b7-9d0f-4e19-b4db-3f92b8b2a9b0/resourceGroups/destiny-rg/providers/Microsoft.Compute/galleries/destinySig/images/destinyImage/versions/1.0.0"

  os_disk {
    caching              = "ReadWrite"
    storage_account_type = "Standard_LRS"
  }

  provisioner "remote-exec" {
    inline = ["echo 'Hello World'"]

    connection {
      type        = "ssh"
      host        = self.public_ip_address
      user        = local.admin_username
      private_key = file("~/.ssh/ssrg-balaji")
    }
  }
}

resource "local_file" "foo" {
    content  = azurerm_linux_virtual_machine.master.public_ip_address
    filename = "../../ansible/master-ip.txt"
}

resource "local_file" "ansible-inventory" {
  filename = "../../ansible/master-hosts.ini"
  content  = templatefile("${path.module}/master-hosts.ini.tmpl", { region : var.region, vm_type : var.vm_type, ip = azurerm_linux_virtual_machine.master.public_ip_address })

  depends_on = [azurerm_linux_virtual_machine.master]
}

resource "null_resource" "ansible-provision" {
  provisioner "local-exec" {
    command = "ansible-playbook -i ../../ansible/master-hosts.ini --private-key ~/.ssh/ssrg-balaji -f 160 -v ../../ansible/master-ansible-playbook.yml"
    environment = {
      ANSIBLE_CONFIG = "../../ansible/ansible.cfg"
    }
  }

  depends_on = [local_file.ansible-inventory]
}