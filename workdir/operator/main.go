// ruptura-operator — lightweight Kubernetes operator for RupturaInstance CRDs.
//
// Build:  go build -o ruptura-operator ./
// Deploy: kubectl apply -f ../deploy/crd/rupturainstances.ruptura.io.yaml
//         kubectl apply -f ../deploy/operator.yaml
//
// The operator polls the API server every --interval seconds, lists all
// RupturaInstance resources cluster-wide, and reconciles each to the desired
// Deployment + Service + (on OpenShift) Route state.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const operatorVersion = "0.6.9"

var logger = log.New(os.Stdout, "", 0)

func logInfo(msg string, kvs ...interface{}) {
	logger.Print(fmtKVs("INFO", msg, kvs...))
}

func logError(msg string, kvs ...interface{}) {
	logger.Print(fmtKVs("ERROR", msg, kvs...))
}

func fmtKVs(level, msg string, kvs ...interface{}) string {
	s := fmt.Sprintf(`level=%s msg=%q`, level, msg)
	for i := 0; i+1 < len(kvs); i += 2 {
		s += fmt.Sprintf(" %v=%q", kvs[i], fmt.Sprintf("%v", kvs[i+1]))
	}
	return s
}

func main() {
	interval    := flag.Duration("interval", 30*time.Second, "reconcile poll interval")
	metricsAddr := flag.String("metrics-addr", ":9090", "address for the Prometheus metrics server")
	flag.Parse()

	logInfo("ruptura-operator starting", "version", operatorVersion, "interval", interval.String())

	metricsSrv := startMetricsServer(*metricsAddr)
	logInfo("metrics server listening", "addr", *metricsAddr)

	c, err := newK8sClient()
	if err != nil {
		logError("failed to init K8s client", "err", err)
		os.Exit(1)
	}
	logInfo("in-cluster client ready", "namespace", c.ns)

	isOCP := c.routeAPIAvailable()
	if isOCP {
		logInfo("OpenShift detected — Route reconciliation enabled")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	defer metricsSrv.Close()

	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	runLoop(ctx, c, isOCP)
	for {
		select {
		case <-ctx.Done():
			logInfo("shutting down")
			return
		case <-ticker.C:
			runLoop(ctx, c, isOCP)
		}
	}
}

func runLoop(ctx context.Context, c *k8sClient, isOCP bool) {
	var list RupturaInstanceList
	if err := c.get("/apis/ruptura.io/v1alpha1/rupturainstances", &list); err != nil {
		logError("list RupturaInstances failed", "err", err)
		return
	}
	setInstanceCount(len(list.Items))
	logInfo("reconcile loop", "count", len(list.Items))
	for _, inst := range list.Items {
		if err := reconcile(ctx, c, inst, isOCP); err != nil {
			recordReconcileError()
			logError("reconcile failed",
				"name", inst.Metadata.Name,
				"namespace", inst.Metadata.Namespace,
				"err", err)
		} else {
			recordReconcileSuccess()
		}
	}
}
