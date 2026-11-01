# Engine design

Status: draft for owner review, 2026-11-01. Nothing here is implemented yet unless marked current.

Read `docs/engine/CONTRACT.md` for stage rules and `STATE.md` for the active work.

## Goal

A user gets a reliable answer: what will fail, when, why, and how sure the engine is.
Every signal must earn its place with measured evidence on real failures.

## Principles

1. Evidence over formulas. A signal, weight or threshold ships only with a measured result.
2. Missing is not zero. A value is measured or unavailable, and unavailable never lowers health.
3. Consume broadly, conclude narrowly. Collect all workload metrics, publish only proven conclusions.
4. Every workload is compared with its own history before any global limit.
5. Every conclusion names its inputs, so a result can be explained and audited.
6. One implementation per stage.

## Product position

Status: proposal, 2026-11-01. Nothing below has been removed or changed.

To be indispensable, Ruptura must answer one question that the standard stack does not: which part of this cluster will fail, when, why, what else it will hit, and what to do.

Prometheus, Alertmanager, Grafana and OpenShift Console already collect, alert and display. Ruptura sits on top of them and adds the conclusion. It does not replace them.

| Product rule | Meaning |
|---|---|
| Integrate first | Read existing telemetry, write conclusions to existing tools |
| Native outputs | Prometheus metrics, Alertmanager, Kubernetes Events, CRD status, OpenShift Console |
| Trust is the product | Each prediction shows evidence and confidence, and misses are measured |
| Small footprint | Cost stays measured and bounded |

Evidence still needed: the difference from tools such as `predict_linear` rules must be measured, and competing products reviewed.

## Component review

Line counts exclude tests. "Not imported" means no import from `cmd` or `internal`. Go forbids importing `internal` from another module, so Autopilot cannot use these directly, but check it before removal.

| Area | Components | Verdict | Why |
|---|---|---|---|
| Failure prediction | `failuremodes` 364, `replay` 159 | Keep, core | The part with measured results |
| Forecast | `pipeline/metrics` 2521 with `pipeline`, `predictor` 1568 | Merge | Near identical copies |
| Signals | `analyzer` | Redesign | See current state |
| Risk | `fusion` 325 | Redesign | FRI disagrees across endpoints |
| Explanation | `explain` 393, `context` 211 | Keep, rebuild on proven data | Core of user trust |
| Infra levels | `collector` 4679, `/infra/*` routes | Keep, connect | Already covers nodes, network, storage, operators, MCP, and maps to pipeline levels |
| Topology | `correlator` 478, `discovery` 1020 | Keep | Needed for blast radius |
| Ingestion | `ingest` 794, `receiver` 859, `sources` 241 | Merge | Two OTLP paths, target is one adapter layer |
| Notification | `alerter` 551, `notify` 237, `notifier` 106 | Merge, then favour Alertmanager | Three implementations |
| Events | `eventbus` 349, `events` 143 | Review | Possible overlap |
| Actions | `actions` 781 | Keep, raise tests first | Touches clusters, provider coverage 34.8 percent |
| Operator | `workdir/operator` | Keep, the only operator | Generates objects from `RupturaInstance` |
| Not imported | `billing`, `grpcserver`, `notifier`, `orchestrator`, `plugin`, `processor`, `vault`, about 1250 lines | Check, then delete | Unused in this binary |
| Duplicate tooling | Dashboards, datasources, orgs, users, notifications UI and routes | Reduce | Overlaps Grafana, Alertmanager and cluster RBAC. 106 routes is too large a surface |
| UI | `workdir/ui` | Refocus | Prediction, evidence and status, plus an OpenShift Console plugin as an option |

## Current state

Measured on the lab, one reading of `/api/v2/metrics` on 98 workloads.

Inputs: `sources/catalog.yaml` reads 5 of 2594 Prometheus metric names, plus kubeletstats over OTLP.
The lab collector sends no traces or logs.

| Signal | Current formula | Lab value | Defect |
|---|---|---|---|
| stress | `.30 cpu + .20 ram + .20 load + .20 errors + .10 timeouts` | 0 to 0.30 | Fixed weights. Half the inputs are absent. `load_avg_1` stands in for latency. |
| fatigue | Accumulates `stress - threshold` | 0 | Stress never reaches the threshold, because of the defect above. |
| mood | `log(uptime * requests / (errors * timeouts * restarts + e)) / 35` | 0 | A product of absent inputs. Scale depends on `e`. Reported as `1 - mood`, so absence counts as a full penalty. |
| pressure | Z-score of latency and errors, mapped to 0 to 1 | 0.5 | Inputs absent, 0.5 is the neutral value. |
| humidity | `errors * timeouts / requests` | 0 | Zero as soon as either factor is zero. |
| contagion | Trace edges, otherwise `errors * cpu` | 0 | No traces on the lab. The fallback has no causal meaning. |
| resilience, throughput | Not reviewed yet | 0 | To review. |
| entropy | Deviation of health history | Under 0.012 | Derived from health, adds no new information. |
| velocity | Change of health | 0.02 to 0.19 | Derived from health, adds no new information. |
| health_score | Weighted penalties | 0.79 to 1 | Stays above the 0.70 degraded limit. Absent inputs distort it. |

Failure detection that works today: memory saturation with time to failure, CPU throttling, and restarts.
Scored on the lab with few cycles.

Algorithms, as read in code:

| Role | Algorithms |
|---|---|
| Forecast | ILR, Holt-Winters, ARIMA, combined by inverse error |
| Rupture index | CA-ILR |
| Anomaly detection | z-score, MAD, seasonal |
| Model weighting | EWMA of squared error |

The README lists CA-ILR, ARIMA, Holt-Winters, MAD and EWMA as a 5 model ensemble. Their roles differ, so the README must be corrected. EWMA is not a forecaster.

Open: `/metrics` shows FRI 10 on 25 workloads while `/fleet` shows 3 nonzero.

## Target architecture

```
Sources (Prometheus, OTLP metrics, traces, logs, Kubernetes API)
  -> Adapters
  -> Metric catalog: semantic role, unit, scope
  -> Normalizer: per workload series, unit conversion, explicit missing data
  -> Feature store: rates, headroom, deviation from own baseline
  -> Forecast and anomaly models
  -> Signals, each with value, coverage and confidence
  -> Failure modes: saturation, throttling, crash loop, and new proven ones
  -> Risk and state
  -> Explanation
  -> API, UI, CLI, Autopilot
```

Key change: metrics are matched by semantic role, not by hardcoded name. A role such as `http.errors` can be filled by HAProxy, by an application metric or by traces. A signal declares the roles it needs. When they are absent it reports `unavailable` and its weight is removed from health.

Design patterns:

| Need | Pattern |
|---|---|
| Sources | Ports and adapters |
| Forecasters and detectors | Strategy behind one interface |
| Signals and failure modes | Registry with declared inputs |
| Stages | Pipeline, one direction only |
| Value that may be absent | Explicit type, value plus availability |
| Storage | Repository |

## Product vision

Agreed with the owner, 2026-11-01. Not implemented. Supersedes the earlier `RupturaPipeline` proposal.

Ruptura is a customizable predictive observability tool and a cloud native standard. The engine stays the same: same algorithms, same calculation principles. The user declares what it does through one central CRD, as with an OpenTelemetry Collector or a ClusterLogForwarder.

### Operator lifecycle

1. The user installs the operator with OpenShift Console (OLM), Helm, Kustomize or Argo CD.
2. The user creates a `RupturaInstance`. Each team can have its own instance, each runs its own engine.
3. Validation has three layers: CRD schema and webhook reject at apply, the operator deploys nothing and logs the error, `status` gives the reason.
4. When valid, the operator generates standard Kubernetes objects. The user sees only `RupturaInstance`. Internal CRDs are added only if a component later justifies one.
5. The operator keeps objects in the declared state, applies changes, and removes generated objects with `ownerReferences` and a finalizer.
6. The operator upgrades versions without losing data.

Today `RupturaInstance` deploys only the engine.

### CRD

```yaml
apiVersion: ruptura.io/v1alpha1
kind: RupturaInstance
metadata: {name: prod, namespace: ruptura}
spec:
  image: docker.io/<account>/ruptura:8.0.0
  deployment:
    mode: Deployment       # StatefulSet, DaemonSet, Sidecar
    replicas: 1
    resources: {}
  level:
    scope: namespace       # container, pod, workload, namespace, service,
                           # node, cluster, application, customResource
    selector: {matchLabels: {team: shop}}
    exclude: {namespaces: [kube-system]}
  input:
    profile: standard      # minimal, standard, full, deep
    sources:
      - prometheus: {url: https://thanos-querier.openshift-monitoring.svc:9091}
      - otlp: {}
  analysis:
    sensitivity: balanced  # conservative, balanced, aggressive
    horizon: 30m
  output:
    ui: {enabled: true}
    consolePlugin: {enabled: false}
    metrics: {prometheus: true}
    otlp: {endpoint: ""}
  alerting:
    routes:
      - severity: critical
        channels: [ops]
    channels:
      - name: ops
        alertmanager: {url: http://alertmanager:9093}  # webhook, slack, teams, pagerduty, email
  actions:
    mode: suggest          # disabled, suggest, auto (Autopilot)
  storage: {size: 10Gi, retention: 7d}
  security: {tls: {}, apiKeyRef: {name: ruptura-key}}
  logging: {level: info, format: json}
status:
  conditions: []
  inputs: []               # available and missing data
```

Generated objects:

| Block | Objects |
|---|---|
| `deployment` | `Deployment` or `StatefulSet`, `Service`, `ServiceAccount` |
| `input` | RBAC, source connections, `DaemonSet` for `deep` |
| `output` | UI, `Route` or `Ingress`, `ConsolePlugin`, `ServiceMonitor` |
| `alerting` | Configuration, `PrometheusRule` or `AlertmanagerConfig` |
| `storage` | `PersistentVolumeClaim` |
| `security` | `Secret`, `NetworkPolicy`, certificates |

Input is chosen by aggressiveness, not by metric name:

| Profile | Coverage | Cost |
|---|---|---|
| `minimal` | What prediction needs at the chosen level | Low |
| `standard` | Plus errors, latency, network | Medium |
| `full` | All metrics available at the level | High, to be measured |
| `deep` | Plus calls between containers, through eBPF | Very high, needs a privileged DaemonSet and an SCC on OpenShift |

### Multi-level signals

A signal keeps one meaning at every level. Only what it observes changes. Each level supplies its elements and its links, and the calculation is written once.

| Level | Elements | Contagion link |
|---|---|---|
| pod | Containers | Shared resources, internal calls |
| workload | Pods of one application | Load shared between replicas |
| namespace | Workloads | Calls, shared quotas |
| node | Pods on the node | Shared CPU, memory, disk, network |
| service | Services | Network calls |
| cluster | Nodes, components | Control plane, network, storage |

Constraints:

1. Common cause is not contagion. Real propagation has an order in time.
2. Data differs by level. Calls between containers need eBPF or a mesh, services need traces.
3. Evidence is per level. A level without proven prediction reports conditions and anomaly detection only, and `status` says so.

### Signals and recipes

Agreed with the owner, 2026-11-01.

Metrics are ingredients, signals are transformed ingredients, the engine is the pot, and the failure result is the dish. The user never sees a formula.

A shared vocabulary keeps one meaning at every level, and each level has its own recipe:

| Signal | Meaning |
|---|---|
| `stress` | Under strain now |
| `fatigue` | Under strain for too long |
| `pressure` | Quality of service is degrading |
| `entropy` | Behaviour became unstable |
| `contagion` | Spreads a problem or suffers from one |
| `resilience` | Recovers well, or less and less well |

Some signals exist at one level only, such as post-deploy regression at workload level or quota headroom at namespace level. `mood` and `humidity` are candidates to merge, `throughput` is a candidate workload signal. These are opinions until measured.

Each signal has one card per level: question, ingredients with the required minimum, baseline from the element's own history, failure it announces, measured score, verdict. A signal without proof does not exist at that level.

### Scope and release rule

Coverage of every failure is impossible, so the scope is closed per version.

1. Each level targets 5 failures. A version lists its failures in advance, and is done when they are proven, not when everything is covered.
2. Exit criteria are written in advance: detection rate, lead time, false alarms.
3. A new failure is a new recipe in the catalog, without changing the engine.
4. Failures are chosen by frequency in real clusters. Rare cases wait.
5. Fewer failures fully proven are better than many partly proven.

Three outcomes only:

| Case | Output |
|---|---|
| Known failure | Failure name, estimated time, evidence |
| Abnormal behaviour | Signal state and moving metrics, no invented cause |
| Out of scope | `unavailable` |

Each failure is labelled predicted or detected, because some can only be detected. Lead time is reported separately.

Candidates, ordered by assumed frequency, to confirm:

| Level | Failures |
|---|---|
| pod | Memory saturation, crash loop, CPU throttling, startup stuck, local disk full |
| workload | Replicas unavailable, rollout stuck, post-deploy regression, traffic drop, error rise |
| namespace | Memory quota, CPU quota, volume full, object quota, secret or certificate expiry |
| node | Memory pressure, disk pressure, noisy neighbour, `NotReady`, pod capacity |
| cluster | Capacity shortage, slow etcd, API server overload, certificate expiry, degraded operator |

Output is a structured result, not a sentence:

```yaml
level: workload
target: shop/orders
failureMode: memorySaturation
state: critical
etaSeconds: 720
confidence: 0.85
evidence:
  - metric: memory_working_set_bytes
    value: 91% of limit
    trend: +2%/min
recommendedAction: raise the memory limit
```

`confidence` must come from measurements. The UI and notifications are built from this result.

### Versions

| Version | Content | Done when |
|---|---|---|
| Summit, 2026-11-24 | CRD and operator, 5 pod failures, 2 or 3 workload failures | Exit criteria met on the lab, benchmark published |
| Next | Complete workload, add node | 5 proven per level |
| Later | Namespace, then cluster | 5 proven per level |

The schema and catalog cover all levels from the start. Node contagion moves to the next version.

### Logs and events

Metrics and Kubernetes Events are the base input. Logs are an optional ingredient, used to confirm and explain, and no recipe may depend on them.

Formats are not parsed per tool. OTLP is the single entry point, and the user's collector normalizes. Read adapters such as Loki or Elasticsearch can come later through `input.sources`.

Kubernetes Events come first: structured, identical across clusters, and they need no collector. Examples: `FailedMount`, `BackOff`, `FailedScheduling`, `Evicted`, `Unhealthy`.

Logs yield indicators, not text understanding. Order: error rate and bursts, then known patterns, then new patterns if they prove their value. A log without severity makes the indicator `unavailable`.

| Version | Inputs |
|---|---|
| Summit | Metrics and Kubernetes Events |
| Next | Logs through OTLP, error rate and bursts, gain measured |
| Later | Known and new patterns, if proven |

```yaml
input:
  profile: standard
  sources:
    - prometheus: {url: https://thanos-querier.openshift-monitoring.svc:9091}
    - kubernetesEvents: {enabled: true}
    - otlp: {signals: [metrics, logs, traces]}
```

### Benchmark

Agreed with the owner, 2026-11-01. A public benchmark is a mandatory exit criterion of each version.

| Item | Choice |
|---|---|
| Opponents | Alerts shipped with OpenShift, and well tuned `predict_linear` rules |
| Terrain | The same faults on the same lab, at least 3 runs per fault |
| Measures | Failures detected, lead time, false alarms on healthy workloads, resources used, configuration effort |
| Publication | Reproducible scripts and all results, losses included |

Win rule, fixed in advance: on a given failure, Ruptura warns earlier or with fewer false alarms than the opponent, at comparable configuration effort.

A tie on a steady memory leak is acceptable. On a loss, improve the recipe or state the result, and never present it as an advantage.

### Open

README message, final list of pod and workload failures, and CI access.

## Evidence protocol

1. Fault library: reproducible faults on the lab, recorded with labels, starting with the failures of the current version.
2. Replay: extend `internal/replay` so any signal is scored on recordings.
3. Score: detection rate, lead time, false alarms on healthy workloads, and gain against the simplest baseline.
4. Decision: keep, change or delete the signal. The result goes in this file.

Constraint: learned weights need labelled failures. Until the library exists, use only rules that the evidence supports.

## Plan to the Summit

Approved by the owner, 2026-11-01. Results freeze on 2026-11-17.

| Dates | Work | Exit |
|---|---|---|
| Nov 2 to 8 | Phase 1 closed, catalog and normalizer for pod and workload, Kubernetes Events, structured output, fault library, first benchmark bench | Memory saturation compared with `predict_linear` end to end |
| Nov 9 to 15 | `RupturaInstance` blocks, validation, status, object generation, pod failures, workload failures, Alertmanager and webhook | Install from zero through the operator, 7 failures recorded at least 3 times |
| Nov 16 to 17 | Full benchmark | Results frozen on Nov 17 |
| Nov 18 to 23 | Bug fixes only, README and install docs, minimal UI, published images, rehearsal, backup video | Demonstration ready |

Cut first if late, in order: `StatefulSet` mode, rollout stuck failure, `standard` profile, local disk full failure.

Never cut: benchmark, honest missing data, install through the operator.

After the Summit: full duplicate merge unless the catalog needs it, API surface reduction, unused package cleanup, tests on actions, `gofmt`, OpenShift Console plugin, eBPF, logs, levels node, namespace and cluster.

Images stay on Docker Hub. Quay.io can be a mirror later.

## Owner decisions, 2026-11-01

1. Signal names stay in the API for signals that pass the evidence test. Any rename needs a version plan.
2. Summit scope on 2026-11-24: see Versions. The rest is roadmap.
3. Traces are a recommended input.
4. Edition: basic configuration goes in community, automatic actions stay in Autopilot. This updates the frozen feature set note in `CLAUDE.md`.
5. Surface reduction is approved in principle, one component at a time, with each removal stated.
