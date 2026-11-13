package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func waitingPod(reason, message, lastLog string) podInfo {
	var p podInfo
	p.Status.Phase = "Running"
	cs := containerStatus{Name: "ruptura"}
	cs.State.Waiting = &stateDetail{Reason: reason, Message: message}
	if lastLog != "" {
		cs.LastState.Terminated = &stateDetail{Reason: "Error", Message: lastLog, ExitCode: 1}
	}
	p.Status.ContainerStatuses = []containerStatus{cs}
	return p
}

func unschedulablePod(message string) podInfo {
	var p podInfo
	p.Status.Phase = "Pending"
	p.Status.Conditions = []condition{{Type: "PodScheduled", Status: "False", Reason: "Unschedulable", Message: message}}
	return p
}

func TestExplainNotReady(t *testing.T) {
	crashLog := "ts=1 level=info msg=starting\nts=2 level=error msg=\"server error\" err=\"open badger: permission denied\"\n"
	cases := []struct {
		name      string
		pods      []podInfo
		pvcPhase  string
		depFailed string
		want      []string
	}{
		{"crash loop shows the last log line", []podInfo{waitingPod("CrashLoopBackOff", "back-off 40s", crashLog)}, "Bound", "",
			[]string{"CrashLoopBackOff", "exit code 1", "open badger: permission denied"}},
		{"image pull names the reason and message", []podInfo{waitingPod("ImagePullBackOff", "Back-off pulling image \"docker.io/x\"", "")}, "Bound", "",
			[]string{"ImagePullBackOff", "Back-off pulling image"}},
		{"unschedulable pod gives the scheduler message", []podInfo{unschedulablePod("didn't find available persistent volumes to bind")}, "Pending", "",
			[]string{"cannot be scheduled", "persistent volumes"}},
		{"unbound claim without pod", nil, "Pending", "",
			[]string{"volume claim ruptura-data is Pending"}},
		{"pod refused by admission", nil, "Bound", "pods \"ruptura-1\" is forbidden: unable to validate against any security context constraint",
			[]string{"not created", "security context constraint"}},
		{"nothing known yet", nil, "Bound", "", []string{"no engine pod yet"}},
		{"container creating is not a failure", []podInfo{waitingPod("ContainerCreating", "", "")}, "Bound", "", []string{"starting"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := explainNotReady("ruptura", c.pods, c.pvcPhase, c.depFailed)
			for _, w := range c.want {
				if !strings.Contains(got, w) {
					t.Errorf("explainNotReady() = %q, want it to contain %q", got, w)
				}
			}
		})
	}
}

func TestExplainNotReady_BoundsLongMessages(t *testing.T) {
	got := explainNotReady("ruptura", []podInfo{waitingPod("CrashLoopBackOff", "", strings.Repeat("x", 2000))}, "Bound", "")
	if len(got) > maxReasonLength+100 {
		t.Errorf("message length %d, want it bounded near %d", len(got), maxReasonLength)
	}
}

func TestUpdateStatus_ExplainsPending(t *testing.T) {
	var patched map[string]RupturaInstanceStatus
	pods := map[string][]podInfo{"items": {unschedulablePod("0/1 nodes are available: didn't find available persistent volumes to bind")}}
	c := fakeServer(t, map[string]http.HandlerFunc{
		"/api/v1/namespaces/ns/pods": func(w http.ResponseWriter, r *http.Request) {
			if got := r.URL.Query().Get("labelSelector"); got != instanceLabel+"=ri" {
				t.Errorf("labelSelector = %q", got)
			}
			_ = json.NewEncoder(w).Encode(pods)
		},
		"/api/v1/namespaces/ns/persistentvolumeclaims/ri-data": func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"status":{"phase":"Pending"}}`))
		},
		"/apis/ruptura.io/v1alpha1/namespaces/ns/rupturainstances/ri/status": func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&patched)
			_, _ = w.Write([]byte(`{}`))
		},
	})
	if err := updateStatus(c, makeInstance("ri", "ns"), ""); err != nil {
		t.Fatal(err)
	}
	st := patched["status"]
	if st.Phase != "Pending" || !strings.Contains(st.Message, "persistent volumes") || st.Operator != c.ns {
		t.Errorf("status = %+v", st)
	}
}

func TestUpdateStatus_RunningClearsMessage(t *testing.T) {
	var raw map[string]map[string]interface{}
	c := fakeServer(t, map[string]http.HandlerFunc{
		"/apis/apps/v1/namespaces/ns/deployments/ri": func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"status":{"readyReplicas":1,"availableReplicas":1}}`))
		},
		"/apis/ruptura.io/v1alpha1/namespaces/ns/rupturainstances/ri/status": func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&raw)
			_, _ = w.Write([]byte(`{}`))
		},
	})
	if err := updateStatus(c, makeInstance("ri", "ns"), ""); err != nil {
		t.Fatal(err)
	}
	msg, present := raw["status"]["message"]
	if raw["status"]["phase"] != "Running" || !present || msg != "" {
		t.Errorf("status = %+v, want Running with an empty message that clears an old one", raw["status"])
	}
}
