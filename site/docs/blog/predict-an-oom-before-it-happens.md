# Predict an OOM kill before it happens

*November 2026 · Selim Benfradj*

Most memory alerts fire too late. A rule like "memory above 90% of the limit" tells you a pod is in trouble, but not whether it will be killed in two minutes or never. By the time the alert is read, the container has often already exited with `OOMKilled`.

Ruptura takes a different approach: instead of a threshold, it gives you a **time left**.

## One conclusion instead of one more chart

Ruptura runs in your cluster, on top of the Prometheus you already have. For each workload it watches, it returns one structured result per failure:

```json
{
  "level": "workload",
  "target": "demo-shop/Deployment/step-leak",
  "failureMode": "saturation",
  "kind": "predicted",
  "state": "degraded",
  "etaSeconds": 626,
  "recommendedAction": "Review the memory limit or restart the pod before it is killed",
  "evidence": ["Memory at 75% of its limit"]
}
```

That reads as: this Deployment is on course to reach its memory limit in about ten minutes, here is why, and here is what to do. The result goes to your Alertmanager, so it follows the routes, silences and receivers you already use.

## Predicted versus detected

Every result says whether it is **predicted** (it has not happened yet, and carries `etaSeconds`) or **detected** (it is happening now). Ruptura never dresses up a detection as a prediction.

| Predicted | Detected |
|---|---|
| Memory saturation | OOM kill |
| A release that leaks faster than the one it replaced | Crash loop |
| One replica leaking while its siblings stay flat | Pod stuck at startup |
| Failures recurring on a regular rhythm | Unavailable replicas, stuck rollout, CPU throttling |

The full rules are on the [failure modes](../failures.md) page.

## Each workload against its own history

A single global threshold is wrong for most workloads: a cache is supposed to sit near its limit, a stateless API is not. Ruptura compares every workload with its own history. Two of the newest checks show why that matters:

- **Release regression.** When a new version of a Deployment grows toward its memory limit at least 3 times faster than the version it replaced, Ruptura warns up to 2 hours ahead and suggests a roll back.
- **Replica divergence.** When one replica out of three or more grows 3 times faster than the others, Ruptura names that pod. An average across replicas would never show it.

## Honest about what it cannot see

A container without a memory limit cannot be forecast for saturation, and a new workload needs about 10 minutes of samples first. Until then its state is `calibrating`. `GET /api/v2/coverage` lists what each instance can and cannot measure on your cluster.

## Try it in two minutes

Locally, no cluster needed:

```bash
git clone https://github.com/benfradjselim/ruptura
cd ruptura
docker compose up --build
```

Open <http://localhost:8081> and sign in with `ruptura-local-dev`.

On OpenShift or Kubernetes, follow the [installation guide](../install.md): one operator, one `RupturaInstance` resource.

## Help shape it

Ruptura is open source (Apache 2.0) and still early: it is not yet recommended for production. I am looking for teams running Kubernetes or OpenShift with Prometheus who will try it on a test cluster and tell me what is missing.

- Tell me which failure pages you most often in [Discussions](https://github.com/benfradjselim/ruptura/discussions)
- Report bugs in [Issues](https://github.com/benfradjselim/ruptura/issues)
- If it is useful to you, a star on [GitHub](https://github.com/benfradjselim/ruptura) helps others find it
