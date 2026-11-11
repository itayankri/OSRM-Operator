# OSRMCluster Full YAML Reference

A single example covering all available spec fields with inline comments.

```yaml
apiVersion: osrm.itayankri/v1alpha1
kind: OSRMCluster
metadata:
  name: my-osrm
spec:
  # URL to the OpenStreetMap PBF file.
  # Changing this triggers a zero-downtime blue-green map update.
  pbfUrl: https://download.geofabrik.de/australia-oceania/marshall-islands-260328.osm.pbf

  # Optional: override the OSRM backend container image.
  # Defaults to ghcr.io/project-osrm/osrm-backend:v5.27.1
  image: ghcr.io/project-osrm/osrm-backend:v5.27.1

  profiles:
  - name: car                  # OSRM profile name (also used for resource naming)
    endpointName: driving      # HTTP path segment exposed via the gateway
    minReplicas: 2             # Lower HPA bound
    maxReplicas: 10            # Upper HPA bound
    replicas: 2                # Optional: fixed replica count (disables HPA if set)

    # Optional: override the internal service endpoint path (defaults to endpointName)
    internalEndpoint: driving

    # Optional: override the OSRM profile file name (defaults to name)
    osrmProfile: car

    resources:
      requests:
        cpu: 100m
        memory: 100Mi
      limits:
        cpu: 500m
        memory: 500Mi

    # Optional: CLI flags passed to osrm-routed
    osrmRoutedOptions:
      maxViaRouteSize: 1000    # --max-viaroute-size
      maxTripSize: 1000        # --max-trip-size
      maxTableSize: 1000       # --max-table-size
      maxMatchingSize: 1000    # --max-matching-size
      maxNearestSize: 1000     # --max-nearest-size

    # Optional: periodic speed data updates via CronJob
    speedUpdates:
      schedule: "0 * * * *"   # Cron expression
      url: https://example.com/speed-data.csv
      suspend: false           # Set true to pause the CronJob without deleting it
      image: itayankri/osrm-speed-updates:osrm-v5.27.1  # defaults to this
      resources:
        requests:
          cpu: 100m
          memory: 100Mi
      env:
      - name: EXAMPLE_VAR
        value: "example-value"

  - name: foot
    endpointName: walking
    minReplicas: 1
    maxReplicas: 4

  # Map builder Job configuration (runs once per map generation)
  mapBuilder:
    # Defaults to itayankri/osrm-builder:osrm-v5.27.1
    image: itayankri/osrm-builder:osrm-v5.27.1
    resources:
      requests:
        cpu: "1"
        memory: 1Gi
    env:
    - name: EXAMPLE
      value: "--max-matching-size=1000"
    # Schedule map building on dedicated nodes (GCP node pool example)
    tolerations:
    - key: dedicated
      operator: Equal
      value: map-building
      effect: NoSchedule
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
        - matchExpressions:
          - key: cloud.google.com/gke-nodepool
            operator: In
            values:
            - map-builder

  service:
    type: LoadBalancer          # LoadBalancer or ClusterIP (default)
    loadBalancerIP: 1.2.3.4    # Optional: request a specific external IP
    annotations:
      cloud.google.com/load-balancer-type: "External"
    # Which OSRM service types to expose via the gateway
    # Options: route, table, match, trip, nearest, tile
    exposingServices: ["route", "table", "match", "trip", "nearest"]

  persistence:
    storageClassName: premium-rwo   # GCP: standard, premium-rwo; NFS: gcnv-nfs-sc
    storage: "10Gi"
    accessMode: ReadWriteOnce       # ReadWriteOnce (single node) or ReadWriteMany (NFS)
```
