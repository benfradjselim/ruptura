package callgraph

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/benfradjselim/ruptura/internal/discovery"
	"github.com/benfradjselim/ruptura/pkg/logger"
)

const (
	refreshInterval = time.Minute
	requestTimeout  = 30 * time.Second
	pageSize        = 500
)

var workloadPaths = []struct{ path, kind string }{
	{"apis/apps/v1/deployments", "Deployment"},
	{"apis/apps/v1/statefulsets", "StatefulSet"},
	{"apis/apps/v1/daemonsets", "DaemonSet"},
}

type meta struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type workloadItem struct {
	Metadata meta `json:"metadata"`
	Spec     struct {
		Template struct {
			Metadata struct {
				Labels map[string]string `json:"labels"`
			} `json:"metadata"`
			Spec struct {
				Containers []struct {
					Env []struct {
						Name  string `json:"name"`
						Value string `json:"value"`
					} `json:"env"`
				} `json:"containers"`
			} `json:"spec"`
		} `json:"template"`
	} `json:"spec"`
}

type serviceItem struct {
	Metadata meta `json:"metadata"`
	Spec     struct {
		Selector map[string]string `json:"selector"`
	} `json:"spec"`
}

type routeItem struct {
	Metadata meta `json:"metadata"`
	Spec     struct {
		Host string `json:"host"`
		To   struct {
			Name string `json:"name"`
		} `json:"to"`
	} `json:"spec"`
}

type ingressItem struct {
	Metadata meta `json:"metadata"`
	Spec     struct {
		Rules []struct {
			Host string `json:"host"`
			HTTP struct {
				Paths []struct {
					Backend struct {
						Service struct {
							Name string `json:"name"`
						} `json:"service"`
					} `json:"backend"`
				} `json:"paths"`
			} `json:"http"`
		} `json:"rules"`
	} `json:"spec"`
}

// Collector refreshes the call graph from the Kubernetes API. It is safe for
// concurrent use.
type Collector struct {
	apiBase string
	token   string
	client  *http.Client
	mu      sync.RWMutex
	graph   Graph
}

// New returns a collector with the pod's in-cluster credentials.
func New() (*Collector, error) {
	apiBase, token, client, err := discovery.InClusterCreds()
	if err != nil {
		return nil, fmt.Errorf("callgraph: %w", err)
	}
	return &Collector{apiBase: apiBase, token: token, client: client}, nil
}

// Graph returns the last built graph.
func (c *Collector) Graph() Graph {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.graph
}

// Run refreshes the graph until ctx ends.
func (c *Collector) Run(ctx context.Context) {
	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()
	for {
		if err := c.refresh(ctx); err != nil {
			logger.Default.Warn("call graph refresh failed", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (c *Collector) refresh(ctx context.Context) error {
	var workloads []Workload
	for _, wp := range workloadPaths {
		err := list(ctx, c, wp.path, func(raw json.RawMessage) error {
			var item workloadItem
			if err := json.Unmarshal(raw, &item); err != nil {
				return err
			}
			workloads = append(workloads, toWorkload(item, wp.kind))
			return nil
		})
		if err != nil {
			return fmt.Errorf("list %s: %w", wp.kind, err)
		}
	}
	var services []Service
	if err := list(ctx, c, "api/v1/services", func(raw json.RawMessage) error {
		var item serviceItem
		if err := json.Unmarshal(raw, &item); err != nil {
			return err
		}
		services = append(services, Service{Namespace: item.Metadata.Namespace, Name: item.Metadata.Name, Selector: item.Spec.Selector})
		return nil
	}); err != nil {
		return fmt.Errorf("list services: %w", err)
	}
	var entries []Entry
	_ = list(ctx, c, "apis/route.openshift.io/v1/routes", func(raw json.RawMessage) error {
		var item routeItem
		if err := json.Unmarshal(raw, &item); err != nil {
			return err
		}
		entries = append(entries, Entry{Kind: "Route", Namespace: item.Metadata.Namespace, Name: item.Metadata.Name, Host: item.Spec.Host, Service: item.Spec.To.Name})
		return nil
	})
	_ = list(ctx, c, "apis/networking.k8s.io/v1/ingresses", func(raw json.RawMessage) error {
		var item ingressItem
		if err := json.Unmarshal(raw, &item); err != nil {
			return err
		}
		entries = append(entries, ingressEntries(item)...)
		return nil
	})

	graph := Build(workloads, services, entries)
	c.mu.Lock()
	c.graph = graph
	c.mu.Unlock()
	return nil
}

func toWorkload(item workloadItem, kind string) Workload {
	w := Workload{Namespace: item.Metadata.Namespace, Kind: kind, Name: item.Metadata.Name, Labels: item.Spec.Template.Metadata.Labels}
	for _, container := range item.Spec.Template.Spec.Containers {
		for _, env := range container.Env {
			if env.Value != "" {
				w.Env = append(w.Env, EnvVar{Name: env.Name, Value: env.Value})
			}
		}
	}
	return w
}

func ingressEntries(item ingressItem) []Entry {
	var entries []Entry
	for _, rule := range item.Spec.Rules {
		for _, path := range rule.HTTP.Paths {
			if name := path.Backend.Service.Name; name != "" {
				entries = append(entries, Entry{Kind: "Ingress", Namespace: item.Metadata.Namespace, Name: item.Metadata.Name, Host: rule.Host, Service: name})
			}
		}
	}
	return entries
}

// list reads every page of a cluster wide list. An absent API, such as Routes
// outside OpenShift, returns an error the caller may ignore.
func list(ctx context.Context, c *Collector, path string, each func(json.RawMessage) error) error {
	next := ""
	for {
		query := url.Values{"limit": {fmt.Sprint(pageSize)}}
		if next != "" {
			query.Set("continue", next)
		}
		page, err := c.get(ctx, path+"?"+query.Encode())
		if err != nil {
			return err
		}
		for _, raw := range page.Items {
			if err := each(raw); err != nil {
				return err
			}
		}
		if page.Metadata.Continue == "" {
			return nil
		}
		next = page.Metadata.Continue
	}
}

type listPage struct {
	Metadata struct {
		Continue string `json:"continue"`
	} `json:"metadata"`
	Items []json.RawMessage `json:"items"`
}

func (c *Collector) get(ctx context.Context, path string) (listPage, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiBase+"/"+path, nil)
	if err != nil {
		return listPage{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := c.client.Do(req)
	if err != nil {
		return listPage{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return listPage{}, fmt.Errorf("%s: status %d", path, resp.StatusCode)
	}
	var page listPage
	if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
		return listPage{}, fmt.Errorf("%s: decode: %w", path, err)
	}
	return page, nil
}
