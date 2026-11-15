# Ruptura console plugin

An OpenShift dynamic console plugin that shows the failures the Ruptura
engine names inside the OpenShift console:

- **Observe > Ruptures**: every failure, predicted first, with workload,
  failure mode, evidence, recommended action and the callers it can take down.
- A **Ruptura** tab on Deployment, StatefulSet and DaemonSet details, with the
  memory chart and forecast of a predicted memory failure.

The image serves the plugin assets with nginx on 9443 behind the service
serving certificate and proxies the read endpoints of the engine
(`/api/v2/failures`, `/api/v2/failures/series`, `/api/v2/failures/closest`,
`/api/v2/callgraph`), adding the API key server-side. The browser never
holds the key.

The Ruptura operator deploys it when `spec.output.consolePlugin.enabled` is
true on a `RupturaInstance`. See `site/docs/console-plugin.md`.

## Develop

```sh
npm ci
npm test
npm run build
```
