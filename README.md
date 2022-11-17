# OSRMCluster Operator
A kubernetes operator to deploy and manage [OSRM](https://project-osrm.org/) routing engine clusters. This operator efficiently deploys OSRM instances by sharing map data accross all pods of a specific instance.

## Quickstart
First, make sure you have a running Kubernetes cluster and kubectl installed to access it. Then run the following command to install the operator:
```
kubectl apply -f https://github.com/itayankri/OSRM-Operator/releases/latest/download/osrm-cluster-operator.yaml
```

Then you can deploy an OSRMCluster:
```
kubectl apply -f https://github.com/itayankri/OSRM-Operator/blob/master/examples/multi_profile_osrm_cluster.yaml
```

## Pausing the Operator
The reconciliation can be paused by adding the following annotation to the OSRMCluster resource:
```bash
osrm.itayankri/operator.paused: "true"
```
The operator will not react to any changes to the OSRMCluster resource or any of the watched resources. If a paused OSRMCluster resource is deleted, the dependent resources will still be cleaned up because thay all have an ownerReference.