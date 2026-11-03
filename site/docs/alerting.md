# Alerting

An instance sends its active failures to each channel every minute. An alert ends 3 minutes after the failure stops being reported.

## OpenShift cluster Alertmanager

```yaml
spec:
  alerting:
    channels:
      - name: cluster
        alertmanager:
          url: https://alertmanager-main.openshift-monitoring.svc:9094
          serviceAccountToken: true
```

With `serviceAccountToken: true` the engine sends its service account token to that URL only, and trusts the OpenShift service CA. The operator binds the role `monitoring-alertmanager-edit` in `openshift-monitoring` to the engine. Alerts then follow the routes, silences and receivers of the cluster Alertmanager.

Never set `serviceAccountToken` for an Alertmanager outside the cluster.

## Any Alertmanager

```yaml
      - name: team
        alertmanager:
          url: http://alertmanager.monitoring.svc:9093
```

## Alert content

| Label | Example |
|---|---|
| `alertname` | `RupturaCrashLoop`, `RupturaSaturation` |
| `severity` | `critical` or `warning` |
| `namespace` | `demo-shop` |
| `target` | `demo-shop/Deployment/step-leak` |
| `kind` | `predicted` or `detected` |
| `level` | `workload` |

Annotations: `summary` holds the recommended action, `state` the failure state.

The OpenShift inhibition rule applies: a `critical` alert hides `warning` alerts with the same `alertname` in the same namespace.
