@description('Specifies a name for generating resource names.')
param projectName string

@description('Specifies the location for all resources.')
param location string = resourceGroup().location

@description('Specifies a username for the Virtual Machine.')
param adminUsername string

@description('Specifies the SSH rsa public key file as a string. Use "ssh-keygen -t rsa -b 2048" to generate your SSH key pairs.')
param adminPublicKey string

@description('description')
param vmSize string = 'Standard_D2s_v3'

var vNetName_var = '${projectName}-vnet'
var vNetAddressPrefixes = '10.0.0.0/16'
var vNetSubnetName = 'default'
var vNetSubnetAddressPrefix = '10.0.0.0/24'
var vmName_var = '${projectName}-vm'
var publicIPAddressName_var = '${projectName}-ip'
var networkInterfaceName_var = '${projectName}-nic'
var networkSecurityGroupName_var = '${projectName}-nsg'
var networkSecurityGroupName2_var = '${vNetSubnetName}-nsg'

resource networkSecurityGroupName 'Microsoft.Network/networkSecurityGroups@2020-05-01' = {
  name: networkSecurityGroupName_var
  location: location
  properties: {
    securityRules: [
      {
        name: 'ssh_rule'
        properties: {
          description: 'Locks inbound down to ssh default port 22.'
          protocol: 'Tcp'
          sourcePortRange: '*'
          destinationPortRange: '22'
          sourceAddressPrefix: '*'
          destinationAddressPrefix: '*'
          access: 'Allow'
          priority: 123
          direction: 'Inbound'
        }
      }
      {
        name: 'discovery_rule'
        properties: {
          description: 'Allows inbound to port 5060.'
          protocol: 'Tcp'
          sourcePortRange: '*'
          destinationPortRange: '5060'
          sourceAddressPrefix: '*'
          destinationAddressPrefix: '*'
          access: 'Allow'
          priority: 124
          direction: 'Inbound'
        }
      }
    ]
  }
}

resource publicIPAddressName 'Microsoft.Network/publicIPAddresses@2020-05-01' = {
  name: publicIPAddressName_var
  location: location
  properties: {
    publicIPAllocationMethod: 'Dynamic'
  }
  sku: {
    name: 'Basic'
  }
}

resource networkSecurityGroupName2 'Microsoft.Network/networkSecurityGroups@2020-05-01' = {
  name: networkSecurityGroupName2_var
  location: location
  properties: {
    securityRules: [
      {
        name: 'default-allow-22'
        properties: {
          priority: 1000
          access: 'Allow'
          direction: 'Inbound'
          destinationPortRange: '22'
          protocol: 'Tcp'
          sourceAddressPrefix: '*'
          sourcePortRange: '*'
          destinationAddressPrefix: '*'
        }
      }
    ]
  }
}

resource vNetName 'Microsoft.Network/virtualNetworks@2020-05-01' = {
  name: vNetName_var
  location: location
  properties: {
    addressSpace: {
      addressPrefixes: [
        vNetAddressPrefixes
      ]
    }
    subnets: [
      {
        name: vNetSubnetName
        properties: {
          addressPrefix: vNetSubnetAddressPrefix
          networkSecurityGroup: {
            id: networkSecurityGroupName2.id
          }
        }
      }
    ]
  }
}

resource networkInterfaceName 'Microsoft.Network/networkInterfaces@2020-05-01' = {
  name: networkInterfaceName_var
  location: location
  properties: {
    ipConfigurations: [
      {
        name: 'ipconfig1'
        properties: {
          privateIPAllocationMethod: 'Dynamic'
          publicIPAddress: {
            id: publicIPAddressName.id
          }
          subnet: {
            id: resourceId('Microsoft.Network/virtualNetworks/subnets', vNetName_var, vNetSubnetName)
          }
        }
      }
    ]
  }
  dependsOn: [
    vNetName
    networkSecurityGroupName
  ]
}

resource vmName 'Microsoft.Compute/virtualMachines@2021-11-01' = {
  name: vmName_var
  location: location
  properties: {
    hardwareProfile: {
      vmSize: vmSize
    }
    osProfile: {
      computerName: vmName_var
      adminUsername: adminUsername
      linuxConfiguration: {
        disablePasswordAuthentication: true
        ssh: {
          publicKeys: [
            {
              path: '/home/${adminUsername}/.ssh/authorized_keys'
              keyData: adminPublicKey
            }
          ]
        }
      }
    }
    storageProfile: {
      imageReference: {
        publisher: 'Canonical'
        offer: '0001-com-ubuntu-server-focal'
        sku: '20_04-lts-gen2'
        version: 'latest'
      }
      osDisk: {
        createOption: 'FromImage'
      }
    }
    networkProfile: {
      networkInterfaces: [
        {
          id: networkInterfaceName.id
        }
      ]
    }
  }
}
