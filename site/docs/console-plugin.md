# Console plugin

On OpenShift, Ruptura shows its failures inside the OpenShift console, so
nobody has to open another site:

- **Observe > Ruptures** lists every failure Ruptura names, predicted first
  with the time left and the window, then detected, each with the workload,
  the failure mode, the evidence, the recommended action and the callers it
  can take down.
- A **Ruptura** tab on the details page of a Deployment, StatefulSet or
  DaemonSet shows that workload's failures and, for a predicted memory
  failure, its memory chart with the forecast.

![Observe > Ruptures in the OpenShift console](assets/console-plugin-ruptures.png)

The pages say the same thing as the Ruptura web console: same wording, same
ordering, same chart.

## Enable it

The plugin needs OpenShift 4.14 or later and the Ruptura operator.

1. When the operator is installed from OperatorHub, the console proposes to
   enable the `ruptura` console plugin. Accept it, or later: **Administration
   > Cluster Settings > Configuration > Console (operator.openshift.io) >
   Console plugins**, set `ruptura` to Enabled.
2. Enable the plugin on the instance:

    ```yaml
    apiVersion: ruptura.io/v1alpha1
    kind: RupturaInstance
    metadata:
      name: ruptura
      namespace: ruptura-system
    spec:
      output:
        consolePlugin:
          enabled: true
    ```

The operator creates, in the namespace of the instance, the Deployment and
Service `<name>-console-plugin` with a serving certificate, and a cluster-wide
`ConsolePlugin` named `ruptura`. The console reloads with the plugin within a
minute. Setting `enabled: false`, or deleting the instance, removes them.

`output.consolePlugin.image` overrides the plugin image; by default the
operator uses the plugin of its release, or the image OLM passes through
`RELATED_IMAGE_RUPTURA_CONSOLE_PLUGIN` on a disconnected install.

The operator never edits the cluster `Console` configuration: enabling the
plugin there stays a decision of the cluster administrator.

## How the pages reach the engine

The browser never holds the API key.

- The console loads the plugin assets from the plugin Service, over the
  service serving certificate.
- The pages read the engine through the console's plugin proxy
  (`/api/proxy/plugin/ruptura/engine/...`), which reaches the plugin pod.
- In the plugin pod, nginx forwards only the read endpoints `GET
  /api/v2/failures`, `/api/v2/failures/series`, `/api/v2/failures/closest`
  and `/api/v2/callgraph` to the engine Service, adding the `Authorization`
  header from the API key Secret mounted in the pod. Any other path or method
  is refused with 403.

The Ruptures page shows the failures of the projects the user can see, as the
console lists them; the Ruptura tab appears only on workloads the user can
open. The proxy itself is reachable by any user signed in to the console and
serves read-only results, so a user who can open the console can read failure
names of other namespaces by calling the proxy directly; keep that in mind on
a cluster where namespace names are confidential.

The API key is read when the plugin pod starts; after rotating the key,
restart the `<name>-console-plugin` Deployment.
