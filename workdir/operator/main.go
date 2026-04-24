// OHE Operator — minimal K8s controller for OHECluster CRDs.
//
// Build:
//   go build -o ohe-operator ./
//
// Deploy:
//   kubectl apply -f ../deploy/crd/oheclusters.yaml
//   kubectl apply -f operator-deployment.yaml
//
// The operator runs a poll loop (default every 30s) that lists all OHECluster
// resources and reconciles each one to the desired Deployment state.
package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"
	"github.com/benfradjselim/kairo-core/pkg/logger"
)

func main() {
	interval := flag.Duration("interval", 30*time.Second, "reconcile poll interval")
	flag.Parse()

	log.SetFlags(log.Ldate | log.Ltime | log.Lmsgprefix)
	log.SetPrefix("[ohe-operator] ")
	logger.Default.Info("operator starting", "interval", *interval)

	c, err := newK8sClient()
	if err != nil {
		logger.Default.Error("init K8s client failed", "err", err)
	os.Exit(1)
	}
	logger.Default.Info("in-cluster client ready", "namespace", c.ns)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	// Run once immediately, then on each tick.
	runReconcileLoop(c)
	for {
		select {
		case <-ctx.Done():
			logger.Default.Info("shutting down")
			return
		case <-ticker.C:
			runReconcileLoop(c)
		}
	}
}

// runReconcileLoop lists all OHECluster resources and reconciles each.
func runReconcileLoop(c *k8sClient) {
	var list OHEClusterList
	// List across all namespaces
	if err := c.get("/apis/ohe.io/v1alpha1/oheclusters", &list); err != nil {
		logger.Default.Error("list OHEClusters error", "err", err)
		return
	}
	logger.Default.Info("OHEClusters found", "count", len(list.Items))
	for _, cluster := range list.Items {
		reconcile(c, cluster)
	}
}
