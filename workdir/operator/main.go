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
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	interval := flag.Duration("interval", 30*time.Second, "reconcile poll interval")
	flag.Parse()

	log.SetFlags(log.Ldate | log.Ltime | log.Lmsgprefix)
	log.SetPrefix("[ohe-operator] ")
	log.Printf("starting — reconcile every %s", *interval)

	c, err := newK8sClient()
	if err != nil {
		log.Fatalf("init K8s client: %v", err)
	}
	log.Printf("in-cluster client ready (namespace: %s)", c.ns)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	// Run once immediately, then on each tick.
	runReconcileLoop(c)
	for {
		select {
		case <-ctx.Done():
			log.Println("shutting down")
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
		log.Printf("list OHEClusters: %v", err)
		return
	}
	log.Printf("found %d OHECluster(s)", len(list.Items))
	for _, cluster := range list.Items {
		reconcile(c, cluster)
	}
}
