# Disconnected install

This page installs Ruptura on an OpenShift cluster without internet access, with images served by a registry mirror such as Artifactory, Nexus, Quay or the OpenShift mirror registry. It was written from installs on air-gapped OpenShift 4.18 clusters.

Ruptura needs three images. Every bundle from 1.4.7 references them by digest, so a mirror can redirect them.

| Image | Source |
|---|---|
| Operator | `ghcr.io/benfradjselim/ruptura-operator` |
| Engine | `docker.io/selimbf/ruptura` |
| Console | `ghcr.io/benfradjselim/ruptura-ui` |

OperatorHub also pulls the bundle image, from `quay.io/community-operator-pipeline-prod/ruptura-operator`.

## 1. Make the images reachable

Mirror the four repositories above into your registry, keeping their paths, for example `mirror.example.com/benfradjselim/ruptura-operator`. With a pull-through registry (Artifactory remote repositories), one remote per source registry is enough: `ghcr.io`, `docker.io` and `quay.io`.

Pulling the images by hand on a node is not enough. CRI-O uses an image only under the name the pod asks for, so the cluster must know the mirror.

## 2. Tell the cluster about the mirror

Use the mechanism your cluster already uses. OpenShift refuses an `ImageDigestMirrorSet` while an `ImageContentSourcePolicy` exists, so check first:

```bash
oc get imagecontentsourcepolicy
oc get imagedigestmirrorset
```

If an `ImageContentSourcePolicy` exists, add these entries to it. Otherwise create an `ImageDigestMirrorSet` with the same entries under `spec.imageDigestMirrors`:

```yaml
  repositoryDigestMirrors:
  - source: quay.io/community-operator-pipeline-prod/ruptura-operator
    mirrors: [mirror.example.com/community-operator-pipeline-prod/ruptura-operator]
  - source: ghcr.io/benfradjselim/ruptura-operator
    mirrors: [mirror.example.com/benfradjselim/ruptura-operator]
  - source: docker.io/selimbf/ruptura
    mirrors: [mirror.example.com/selimbf/ruptura]
  - source: ghcr.io/benfradjselim/ruptura-ui
    mirrors: [mirror.example.com/benfradjselim/ruptura-ui]
```

Adding mirrors does not reboot the nodes. Wait until the machine config pools are updated before installing:

```bash
oc get mcp
```

## 3. Install the operator

Install Ruptura Operator from OperatorHub, from your mirrored community catalog. If the install fails because the bundle could not be unpacked (`no such host` for `quay.io` in the events of `openshift-marketplace`), the mirror entry for the bundle image was missing. After fixing it, remove the failed attempt so OLM tries again:

```bash
oc get job,configmap -n openshift-marketplace | grep -i ruptura   # find the unpack job and its ConfigMap
oc delete job <unpack-job> -n openshift-marketplace
oc delete configmap <unpack-job> -n openshift-marketplace
oc delete subscription ruptura-operator -n <namespace>
```

Then install again from OperatorHub.

## 4. Create the instance

```yaml
apiVersion: ruptura.io/v1alpha1
kind: RupturaInstance
metadata:
  name: ruptura
  namespace: ruptura-system
spec:
  level:
    scope: workload
  storage:
    type: persistent
  output:
    ui:
      enabled: true
```

Leave `spec.image` and `spec.output.ui.image` empty: the operator takes both from the bundle, by digest, and the mirror redirects them. Set them only to run another version, and then point them at your registry, because a mirror redirects digests, never tags:

```yaml
spec:
  image: mirror.example.com/selimbf/ruptura:v8.7.8
  output:
    ui:
      image: mirror.example.com/benfradjselim/ruptura-ui:ui-v8.7.8
```

### Storage

The engine keeps its history in a volume claim. If the claim stays `Pending`, `oc get rupturainstance` shows the provisioner's message. Two ways out:

- `spec.storage.storageClassName`: use a StorageClass that provisions on your cluster.
- `spec.storage.type: ephemeral`: keep the data in an emptyDir, for clusters without working storage. History is lost when the engine pod restarts, so forecasts based on past failures start over.

## 5. Check

```bash
oc get rupturainstance,pods,route -n ruptura-system
```

The instance reaches `Running`. Open the `ruptura-console` Route and sign in with any user name and the API key as password:

```bash
oc get secret ruptura-api-key -n ruptura-system -o jsonpath='{.data.api-key}' | base64 -d; echo
```

The engine reads cluster metrics from the in-cluster Thanos querier, `thanos-querier.openshift-monitoring.svc:9091`, which every OpenShift cluster has under that name. The engine dashboard shows how many Prometheus series it read in the last cycle; a number above zero means metrics flow. Rupture Horizon lists the pods closest to their memory limit as soon as the first readings arrive, and forecasts after about 30 minutes of readings.

## Known limits

- The engine does not use an HTTP proxy. It talks to the Kubernetes API and to services inside the cluster, so it needs none, unless an alerting channel points outside the cluster.
- A pod without a memory limit cannot get a memory forecast. Rupture Horizon counts these pods under Blind spots.
