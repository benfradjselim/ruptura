package main

import (
	"fmt"
	"net/url"
	"strings"
)

// maxReasonLength bounds the text copied from a pod into the instance status.
const maxReasonLength = 300

type stateDetail struct {
	Reason   string `json:"reason"`
	Message  string `json:"message"`
	ExitCode int32  `json:"exitCode"`
}

type containerState struct {
	Waiting    *stateDetail `json:"waiting,omitempty"`
	Terminated *stateDetail `json:"terminated,omitempty"`
}

type containerStatus struct {
	Name      string         `json:"name"`
	State     containerState `json:"state"`
	LastState containerState `json:"lastState"`
}

type condition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

type podInfo struct {
	Metadata ObjectMeta `json:"metadata"`
	Status   struct {
		Phase             string            `json:"phase"`
		Conditions        []condition       `json:"conditions"`
		ContainerStatuses []containerStatus `json:"containerStatuses"`
	} `json:"status"`
}

// startingReasons are waiting reasons of a container that is on its way up.
var startingReasons = map[string]bool{"": true, "ContainerCreating": true, "PodInitializing": true}

// notReadyReason reads what Kubernetes reports about an instance that is not
// ready, so the status names the cause instead of only "Pending".
func notReadyReason(c *k8sClient, ns, name string) string {
	var pods struct {
		Items []podInfo `json:"items"`
	}
	query := url.Values{"labelSelector": {instanceLabel + "=" + name}}.Encode()
	_ = c.get(fmt.Sprintf("/api/v1/namespaces/%s/pods?%s", ns, query), &pods)
	var pvc struct {
		Status struct {
			Phase string `json:"phase"`
		} `json:"status"`
	}
	_ = c.get(fmt.Sprintf("/api/v1/namespaces/%s/persistentvolumeclaims/%s-data", ns, name), &pvc)
	var dep struct {
		Status struct {
			Conditions []condition `json:"conditions"`
		} `json:"status"`
	}
	_ = c.get(fmt.Sprintf("/apis/apps/v1/namespaces/%s/deployments/%s", ns, name), &dep)
	failed := ""
	for _, cond := range dep.Status.Conditions {
		if cond.Type == "ReplicaFailure" && cond.Status == "True" {
			failed = cond.Message
		}
	}
	return explainNotReady(name, pods.Items, pvc.Status.Phase, failed)
}

// explainNotReady picks the most specific cause: a failing container, then a
// pod the scheduler cannot place, then an unbound volume, then a pod that was
// never created.
func explainNotReady(name string, pods []podInfo, pvcPhase, replicaFailure string) string {
	for _, pod := range pods {
		for _, cs := range pod.Status.ContainerStatuses {
			if w := cs.State.Waiting; w != nil && !startingReasons[w.Reason] {
				return bounded(containerProblem(w, cs.LastState.Terminated))
			}
		}
	}
	for _, pod := range pods {
		for _, cond := range pod.Status.Conditions {
			if cond.Type == "PodScheduled" && cond.Status == "False" {
				return bounded("engine pod cannot be scheduled: " + cond.Message)
			}
		}
	}
	if pvcPhase != "" && pvcPhase != "Bound" {
		return fmt.Sprintf("volume claim %s-data is %s", name, pvcPhase)
	}
	if replicaFailure != "" {
		return bounded("engine pod not created: " + replicaFailure)
	}
	if len(pods) == 0 {
		return "no engine pod yet"
	}
	return "engine starting"
}

func containerProblem(waiting, last *stateDetail) string {
	text := "engine container " + waiting.Reason
	if last != nil && last.Message != "" {
		return fmt.Sprintf("%s, exit code %d: %s", text, last.ExitCode, lastLine(last.Message))
	}
	if waiting.Message != "" {
		return text + ": " + waiting.Message
	}
	return text
}

func lastLine(text string) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

func bounded(text string) string {
	if len(text) > maxReasonLength {
		return text[:maxReasonLength] + "..."
	}
	return text
}
