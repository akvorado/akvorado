# Akvorado Helm Template

This is a simple Helm template for setting up your Akvorado namespace with the IPInfo container within your Google Kubernetes Engine (GKE) cluster.

**Note:**
- Compatibility has been tested only with GKE.
- The shared PV from the IPInfo container has not been tested.

## Features

Out of the box, this template provides the following features:

- Metrics collection for Akvorado within Prometheus using service discovery.
- A Grafana dashboard for visualizing GKE Pods and Akvorado Metrics.

## Passing Secrets

You can pass secrets directly into the Helm template by placing them within the `secrets` section in the specific configuration. For `orchestrator`, use `akvorado_orchestrator`, and for `ipinfo`, place it within `ipinfo`. This allows you to use environment variables to change the Akvorado configuration, or you can update the configuration files inside the [config directory](config/).

You can also use a specific image of Akvorado if you decide to run your own image and tag.

## Akvorado Configuration

Configuration of Akvorado itself should be done. Here are some of the parameters you should configure:

- `config/akvorado.yml`:
  - `kafka: brokers`
  - `clickhouse: orchestrator_url`
  - `clickhouse: servers`
  - `username`
  - `password`
  - `database`
  - `asns`

- `config/console.yml`: If you want to use Redis for caching, you should configure Redis in this section.

- `config/inlet.yml`:
  - `snmp: communities`
  - `classifiers`

## Resource Limits

There are default limits for each deployment, but you can override them, as in the example:

```yaml
deployment:
  akvorado_inlet:
    resources:
      requests:
        memory: 800Mi
        cpu: 0.5
      limits:
        memory: 1000Mi
        cpu: 0.8
```

You can also change the arguments by overriding the defaults. The default contains only the application and the path to the configuration. For example:

```yaml
deployment:
  akvorado_inlet:
    args:
      - arg1
      - arg2
      - arg3
      - argn
```

## Services

Due to the limitation of GKE in running only TCP or UDP in a service, multiple services were created for the inlet:

- bmp: LoadBalancer service for BMP, configured for port 10179 (no option provided in the template to change this port).
- web: ClusterIP service for the inlet web services.
- inlet: LoadBalancer service for flow ingestion (default ports 2055 and 6343; no option is provided in the Helm chart for custom ports).

## Volume Sizes

You can change the volume sizes, as in the following example:
```yaml
volumes:
  akvorado_inlet:
    storageSize: 10Gi
```

## Nginx Ingress

The console web service (/), the inlet API (/api/v0/orchestrator), and the orchestrator API (/api/v0/orchestrator) were configured using an nginx ingress service. This service also has the nginx.ingress.kubernetes.io/whitelist-source-range, which provides a simple ACL for the console front-end that you can configure from the values file (line 17 of the values file).

## LoadBalancer Source Ranges

The services for ingesting flows and BMP have a loadBalancerSourceRanges as well, which you can configure for the routers/switches to receive flows (line 20 of the example file).

## Additional Configuration

This template provides various other configurable options directly in the values file, so we encourage you to explore the templates and try them out.

Please note: This template does not provide a Kafka and ClickHouse cluster for you.