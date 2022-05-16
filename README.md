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

It is advised to use the docker file to build the framework, since it pulls in the necessary dependencies to produce a working build.

If you need to build your own container, do the following:

1. Modify the `REMOTE_CONTAINER_NAME` field in `./Makefile` to point to your container repository
2. Run `make container`

## Deployment Environment

To simplify deployments, this framework assumes the existence of a Kubernetes cluster, either locally or elsewhere. 

### Local

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
helm install kube-prometheus-stack
```

4. Ensure Grafana is up and running and you are able to view the 
`go-consensus` dashboard

```
kubectl port-forward ...
```

Login to https://localhost:3000/ and open the `go-consensus` dashboard.

### Azure

The evaluation studies for the aforementioned papers were carried out on the Azure cloud platform. 
The experiment environment can be reproduced using the provided Terraform plan 
and Ansible configuration files under the `deploy/azure` directory.

#### Prerequisites
* Azure Subscription with enough VM quota
* Terraform
* Ansible

```
ansible-galaxy collection install kubernetes.core
ansible-galaxy collection install community.kubernetes
ansible-galaxy collection install cloud.common
pip3 install kubernetes
```

#### Bringing up the cluster

`cd deploy/azure/terraform`

`terraform plan`

`terraform apply`


#### Cleaning up

To clean up the cluster, run `terraform destroy experiment.tf`

## Running Experiments

Use `erunner` to generate a experiment plan file and run the plan against a Kubernetes cluster.

## Current Status

This framework is under active development.

