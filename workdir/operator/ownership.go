package main

import (
	"fmt"
	"time"
)

// leaseDuration is how long an operator keeps an instance without renewing.
// Another operator takes the instance over only after it expires, so two
// operators watching the same cluster never rewrite the same Deployment.
const leaseDuration = 2 * time.Minute

// LeaseSpec mirrors coordination.k8s.io/v1 LeaseSpec.
type LeaseSpec struct {
	HolderIdentity       string `json:"holderIdentity,omitempty"`
	LeaseDurationSeconds int32  `json:"leaseDurationSeconds,omitempty"`
	RenewTime            string `json:"renewTime,omitempty"`
}

// Lease mirrors coordination.k8s.io/v1 Lease.
type Lease struct {
	APIVersion string     `json:"apiVersion"`
	Kind       string     `json:"kind"`
	Metadata   ObjectMeta `json:"metadata"`
	Spec       LeaseSpec  `json:"spec"`
}

func leaseName(name string) string { return name + "-operator" }

func leasesPath(ns string) string {
	return fmt.Sprintf("/apis/coordination.k8s.io/v1/namespaces/%s/leases", ns)
}

func leasePath(ns, name string) string { return leasesPath(ns) + "/" + leaseName(name) }

// claim takes or renews this operator's hold on an instance. It returns "" when
// this operator holds it, or the identity of the operator that does.
func claim(c *k8sClient, inst RupturaInstance, now time.Time) (string, error) {
	ns, name := inst.Metadata.Namespace, inst.Metadata.Name
	var lease Lease
	err := c.get(leasePath(ns, name), &lease)
	if isNotFound(err) {
		lease = Lease{APIVersion: "coordination.k8s.io/v1", Kind: "Lease",
			Metadata: ObjectMeta{Name: leaseName(name), Namespace: ns, Labels: map[string]string{managedByLabel: "ruptura-operator", instanceLabel: name}}}
		return claimResult(c.send("POST", leasesPath(ns), held(lease, c.ns, now)))
	}
	if err != nil {
		return "", err
	}
	if holder := lease.Spec.HolderIdentity; holder != "" && holder != c.ns && !expired(lease.Spec, now) {
		return holder, nil
	}
	return claimResult(c.send("PUT", leasePath(ns, name), held(lease, c.ns, now)))
}

func held(lease Lease, identity string, now time.Time) Lease {
	lease.Spec = LeaseSpec{HolderIdentity: identity, LeaseDurationSeconds: int32(leaseDuration.Seconds()),
		RenewTime: now.UTC().Format("2006-01-02T15:04:05.000000Z07:00")}
	return lease
}

// claimResult turns a lost write race into "held elsewhere": the winner is
// read on the next loop.
func claimResult(err error) (string, error) {
	if err == errConflict {
		return "another operator", nil
	}
	return "", err
}

func expired(spec LeaseSpec, now time.Time) bool {
	renewed, err := time.Parse(time.RFC3339Nano, spec.RenewTime)
	if err != nil {
		return true
	}
	duration := time.Duration(spec.LeaseDurationSeconds) * time.Second
	if duration <= 0 {
		duration = leaseDuration
	}
	return now.After(renewed.Add(duration))
}

// skipNotices remembers which holder was last reported per instance, so a
// skipped instance is logged once, not every loop.
var skipNotices = map[string]string{}

func noteSkipped(inst RupturaInstance, holder string) {
	key := inst.Metadata.Namespace + "/" + inst.Metadata.Name
	if skipNotices[key] == holder {
		return
	}
	skipNotices[key] = holder
	logInfo("instance managed by another operator, skipped",
		"namespace", inst.Metadata.Namespace, "name", inst.Metadata.Name, "holder", holder)
}
