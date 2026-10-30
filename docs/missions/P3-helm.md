# Mission P3-helm

- Component: C13-packaging
- Allowed paths: helm/
- Defects to fix:
  1. `runAsUser` and `fsGroup` 65532 are refused by OpenShift. Add `openshift.enabled` (default false). When true, omit both on engine and UI pods.
  2. Engine Deployment and Service selectors also match UI pods. Add `app.kubernetes.io/component: engine` to engine labels and selectors.
  3. UI listens on port 80, which fails for a non-root arbitrary UID. Make the nginx listen port a value, default 8080, with the Service `targetPort` following it.
  4. UI container adds `NET_BIND_SERVICE`. Remove it.
  5. `appVersion` is 7.1.0. Set chart appVersion to 8.3.0-rc.3 and align image defaults so engine and UI resolve existing tags: engine `v8.3.0-rc.3`, UI `ui-v8.3.0-rc.3`.
  6. No API key means every call returns 503. Default `autoGenerateApiKey` to true, and replace any `bitnami/kubectl` image with `registry.access.redhat.com/openshift4/ose-cli:latest`.
  7. Add `thanos.url` (default empty) mapped to `RUPTURA_THANOS_URL`. When `openshift.enabled`, default it to `https://thanos-querier.openshift-monitoring.svc:9091` and add a ClusterRoleBinding to `cluster-monitoring-view`.
  8. When `openshift.enabled`, add a Route with edge TLS for the UI.
  9. When `openshift.enabled`, grant ClusterRole read on `config.openshift.io` clusteroperators and `machineconfiguration.openshift.io` machineconfigpools.
- Numeric gate: `helm lint helm` passes. `helm template ruptura helm --set openshift.enabled=true,ui.enabled=true` renders no `runAsUser: 65532`, no `fsGroup: 65532`, no `bitnami`, and engine selectors contain the component label. Default render with `openshift.enabled=false` keeps `runAsUser: 65532`.
- Proof command: the commands above, with grep output pasted in the report.
- Constraints: do not apply anything to a cluster, English only.
- Stop rule: 2 attempts without progress, return to the lead.
- Report: defects fixed, changed files, blockers, 150 words maximum.
