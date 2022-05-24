# Go-Consensus Framework

> __Disclaimer:__ This is not a production ready software

This repository contains the research prototypes of various consensus protocols to enable a consistent evaluation. These prototypes were, in part, used for 
evaluation studies presented in the following papers:

* DQBFT: Scalable Byzantine Fault Tolerance via Partial Decentralization

## Implemented Protocols
- PBFT
- SBFT
- Hybster
- MinBFT
- Chained Hotstuff
- MirBFT
- RCC
- Linear Hybster
- DQPBFT
- DQSBFT
- DQHybster
- Destiny

## Building

The latest container images are made available at https://ghcr.io/ibalajiarun/go-consensus.

It is advised to use the Dockerfile to build the framework, since it pulls in 
the necessary dependencies to produce a working build.

If you need to build your own container, run the following:

```
make container -C REMOTE_CONTAINER_NAME=<path-to-your-remote-container>
```

## Deployment Environment

To simplify deployments, this framework assumes the existence of a Kubernetes 
cluster, either locally or elsewhere. 

### Local setup

1. Install k3s

```
> curl -sfL https://get.k3s.io | sh -s - server --write-kubeconfig-mode 644 --disable traefik
```

2. Install Helm

```
> curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
```

3. Setup Prometheus and Grafana to view experiment metrics

```
> helm install kube-prometheus-stack prometheus-community/kube-prometheus-stack -f deploy/local/k3s/helm-kube-prom-stack-values.yaml -n monitoring --create-namespace
```

4. Ensure Grafana is up and running and you are able to view the 
`go-consensus` dashboard

```
kubectl port-forward svc/kube-prometheus-stack-grafana 3000:80 -n monitoring
```

Login to https://localhost:3000/ with `admin` and `prom-operator` 
and verify access to the `Go Consensus` dashboard.

### Azure setup

The evaluation studies for the aforementioned papers were carried out on the Azure cloud platform. 
The experiment environment can be reproduced using the provided Terraform plan 
and Ansible configuration files under the `deploy/azure` directory.

#### Prerequisites
* Host with publicly accessible IP address
* Azure Subscription with enough VM quota
* Terraform
* Ansible

If you cannot configure your host to be reachable via a public IP, 
it is recommended to deploy a VM on Azure with a Public IP and use it as a 
jumphost. 
You can create one quickly by clicking the button below.

[![Deploy Jumphost to Azure](https://aka.ms/deploytoazurebutton)](https://portal.azure.com/#create/Microsoft.Template/uri/https%3A%2F%2Fraw.githubusercontent.com%2Fibalajiarun%2Fgo-consensus%2Fmain%2Fdeploy%2Fazure%2Fazuredeploy.json)

#### Install Tools

```
sudo apt install ansible terraform
ansible-galaxy collection install kubernetes.core
ansible-galaxy collection install community.kubernetes
ansible-galaxy collection install cloud.common
```
#### Bringing up the cluster

Preparing to deploy the Kubernetes master server VM

```
cd deploy/azure/terraform/server
```

Create a `terraform.tfvars` file with the following information:

```
prefix = "destiny"
region = "eastus"
vm_type = "Standard_D8s_v4"
  
subscription_id = "<Azure Subscription ID"
tenant_id       = "<Azure Tenent ID>"
client_id       = "<Azure Client ID>"
client_secret   = "<Azure Client Secret>"
```

You can create a service pricipal via:

```
az ad sp create-for-rbac --role="Contributor" --scopes="/subscriptions/<azure-subscription-id>"
```

Deploying the Kubernetes master server VM

```
terraform plan -state=server.tfstate -out=server.tfplan -refresh=true -parallelism=50
terraform apply -state=server.tfstate -parallelism=10 -refresh=true server.tfplan
```

Preparing to deploy the Kubernetes agents virtual machine scale sets:

```
cd deploy/azure/terraform/server
```

Create a `terraform.tfvars` file with the following information. The 
`vm_groups` defines the type and number of server and client VMs to create for 
the deployment and their region.

```
prefix = "destiny"
vm_groups = {
  "server" = {
    "Standard_DC8_v2" = {
      "canadacentral"  = 2,
      "canadaeast"     = 2,
      "eastus"         = 2,
      "southcentralus" = 2,
      "westus"         = 2,
      "westus2"        = 2,
      "uksouth"        = 2,
      "northeurope"    = 2,
      "westeurope"     = 2,
      "southeastasia"  = 1,
    },
  },
  "client" = {
    "Standard_D4s_v4" = {
      "canadacentral"  = 1,
      "canadaeast"     = 1,
      "eastus"         = 1,
      "southcentralus" = 1,
      "westus"         = 1,
      "westus2"        = 1,
      "uksouth"        = 1,
      "northeurope"    = 1,
      "westeurope"     = 1,
      "southeastasia"  = 1,
    },
  },
}

cloudconfig_file = "cloudconfig.sh.tmpl"
  
subscription_id = "<Azure Subscription ID"
tenant_id       = "<Azure Tenent ID>"
client_id       = "<Azure Client ID>"
client_secret   = "<Azure Client Secret>"
```

Deploying the Kubernetes agent virtual machine scale sets.


```
cd ../agent
terraform plan -state=agents.tfstate -out=agents.tfplan -refresh=true -parallelism=50
terraform apply -state=agents.tfstate -parallelism=10 -refresh=true agents.tfplan
```

#### Cleaning up

To clean up the cluster:

```
terraform destroy -state=server.tfstate -auto-approve -refresh -parallelism=50
```

```
terraform destroy -state=agents.tfstate -auto-approve -refresh -parallelism=50
```

## Running Experiments

Use the `./bin/erunner` to generate a experiment plan file and run the plan 
against a Kubernetes cluster.

### Building the Experiment Runner

`make erunner`

### Defining the Experiment Config

A sample experiment config file is in `./experiments/configs`.

### Generating the Experiment Plan File

```
./bin/erunner generate experiments/config/n19_bft.yaml experiments/plans/n19_bft.plan.yaml
```

### Running the plan

Forward port 9090 of prometheus to query metrics during experiments. 

Change the master-ip in `erunner.yaml` to the public IP address of your host.

```
kubectl port-forward -n monitoring svc/kube-prometheus-stack-prometheus 9090:9090
```

```
./bin/erunner run experiments/plans/n19_bft.plan.yaml
```

Use Ctrl+C to cancel the experiment and exit the erunner.

## Current Status

This framework is under active development. 
Contact Balaji Arun (balajia at vt dot edu).