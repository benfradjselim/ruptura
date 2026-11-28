// Command ruptura-agent runs on every node when a RupturaInstance asks for the
// deep input profile. It reads basic node signals the cluster Prometheus does
// not have, PID use of the node and of each pod and the network policy drops
// of the OVN-Kubernetes ACL audit log, and sends them to the engine over OTLP.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/benfradjselim/ruptura/internal/agent"
	"github.com/benfradjselim/ruptura/internal/discovery"
	"github.com/benfradjselim/ruptura/pkg/logger"
)

const (
	interval    = 30 * time.Second
	sendTimeout = 10 * time.Second
	aclLog      = "var/log/ovn/acl-audit-log.log"
)

func main() {
	node := os.Getenv("NODE_NAME")
	endpoint := os.Getenv("RUPTURA_OTLP_ENDPOINT")
	if node == "" || endpoint == "" {
		fmt.Fprintln(os.Stderr, "ruptura-agent: NODE_NAME and RUPTURA_OTLP_ENDPOINT are required")
		os.Exit(2)
	}
	root := os.Getenv("RUPTURA_HOST_ROOT")
	if root == "" {
		root = "/host"
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	pods := newPodNames(node)
	tail := agent.NewACLTail(filepath.Join(root, aclLog))
	client := &http.Client{Timeout: sendTimeout}
	logger.Default.Info("ruptura-agent started", "node", node, "endpoint", endpoint)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		report := read(node, root, pods, tail)
		if err := send(ctx, client, endpoint, report); err != nil {
			logger.Default.Warn("ruptura-agent: send failed", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// read takes one reading of the node. A reader that fails is logged and
// left out of the report; the others still go.
func read(node, root string, pods *podNames, tail *agent.ACLTail) agent.Report {
	report := agent.Report{Node: node, At: time.Now()}
	if used, limit, err := agent.NodePIDs(filepath.Join(root, "proc")); err == nil {
		report.NodeUsed, report.NodeMax = used, limit
	} else {
		logger.Default.Warn("ruptura-agent: node PIDs", "err", err)
	}
	if cgroups, err := agent.ScanPodPIDs(filepath.Join(root, "sys/fs/cgroup")); err == nil {
		names := pods.lookup()
		for _, c := range cgroups {
			if name, ok := names[c.UID]; ok {
				report.Pods = append(report.Pods, agent.Pod{Namespace: name[0], Name: name[1], PIDs: c})
			}
		}
	} else {
		logger.Default.Warn("ruptura-agent: pod PIDs", "err", err)
	}
	if drops, err := tail.Drops(); err == nil {
		report.Drops = drops
	} else {
		logger.Default.Warn("ruptura-agent: ACL audit log", "err", err)
	}
	return report
}

func send(ctx context.Context, client *http.Client, endpoint string, report agent.Report) error {
	body, err := json.Marshal(report.Request())
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+"/v1/metrics", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("engine answered %s", resp.Status)
	}
	return nil
}

// podNames maps the pod UIDs of this node to namespace and name, listed from
// the Kubernetes API.
type podNames struct {
	node   string
	api    string
	token  string
	client *http.Client
}

func newPodNames(node string) *podNames {
	api, token, client, err := discovery.InClusterCreds()
	if err != nil {
		logger.Default.Warn("ruptura-agent: no Kubernetes API, pod PIDs are not reported", "err", err)
		return &podNames{node: node}
	}
	return &podNames{node: node, api: api, token: token, client: client}
}

func (p *podNames) lookup() map[string][2]string {
	out := map[string][2]string{}
	if p.client == nil {
		return out
	}
	req, err := http.NewRequest(http.MethodGet, p.api+"/api/v1/pods?fieldSelector=spec.nodeName%3D"+p.node, nil)
	if err != nil {
		return out
	}
	req.Header.Set("Authorization", "Bearer "+p.token)
	resp, err := p.client.Do(req)
	if err != nil {
		logger.Default.Warn("ruptura-agent: list pods", "err", err)
		return out
	}
	defer resp.Body.Close()
	var list struct {
		Items []struct {
			Metadata struct {
				UID       string `json:"uid"`
				Namespace string `json:"namespace"`
				Name      string `json:"name"`
			} `json:"metadata"`
		} `json:"items"`
	}
	if resp.StatusCode != http.StatusOK || json.NewDecoder(resp.Body).Decode(&list) != nil {
		logger.Default.Warn("ruptura-agent: list pods", "status", resp.Status)
		return out
	}
	for _, item := range list.Items {
		out[item.Metadata.UID] = [2]string{item.Metadata.Namespace, item.Metadata.Name}
	}
	return out
}
