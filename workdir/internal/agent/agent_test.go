package agent

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/benfradjselim/ruptura/pkg/models"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestNodePIDs(t *testing.T) {
	proc := t.TempDir()
	write(t, filepath.Join(proc, "loadavg"), "7.30 6.23 5.63 2/2816 3755309\n")
	write(t, filepath.Join(proc, "sys/kernel/pid_max"), "4194304\n")
	used, limit, err := NodePIDs(proc)
	if err != nil || used != 2816 || limit != 4194304 {
		t.Fatalf("NodePIDs = %v %v %v", used, limit, err)
	}
	write(t, filepath.Join(proc, "loadavg"), "garbage\n")
	if _, _, err := NodePIDs(proc); err == nil {
		t.Fatal("read a malformed loadavg")
	}
}

func TestScanPodPIDs(t *testing.T) {
	cg := t.TempDir()
	burst := filepath.Join(cg, "kubepods.slice/kubepods-burstable.slice/kubepods-burstable-poda41bef8a_9c53_4755_b627_574e282db2ce.slice")
	write(t, filepath.Join(burst, "pids.current"), "77\n")
	write(t, filepath.Join(burst, "pids.max"), "4096\n")
	write(t, filepath.Join(burst, "crio-abc.scope/pids.current"), "70\n")
	guaranteed := filepath.Join(cg, "kubepods.slice/kubepods-pod11111111_2222_3333_4444_555555555555.slice")
	write(t, filepath.Join(guaranteed, "pids.current"), "5\n")
	write(t, filepath.Join(guaranteed, "pids.max"), "max\n")
	got, err := ScanPodPIDs(cg)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].UID != "a41bef8a-9c53-4755-b627-574e282db2ce" || got[0].Used != 77 || got[0].Limit != 4096 {
		t.Fatalf("pods = %+v, want only the limited burstable pod", got)
	}
}

const (
	dropLine  = `2026-11-28T10:00:00.000Z|00001|acl_log(ovn_pinctrl0)|INFO|name="NP:shop:allow-web:Ingress:0", verdict=drop, severity=alert, direction=to-lport: tcp` + "\n"
	denyLine  = `2026-11-28T10:00:01.000Z|00002|acl_log(ovn_pinctrl0)|INFO|name="NP:shop:Ingress", verdict=drop, severity=alert, direction=to-lport: tcp` + "\n"
	allowLine = `2026-11-28T10:00:02.000Z|00003|acl_log(ovn_pinctrl0)|INFO|name="NP:shop:allow-web:Ingress:0", verdict=allow, severity=alert, direction=to-lport: tcp` + "\n"
	otherLine = `2026-11-28T10:00:03.000Z|00004|acl_log(ovn_pinctrl0)|INFO|name="NP:batch:Egress", verdict=drop, severity=alert, direction=from-lport: udp` + "\n"
)

func TestACLTail(t *testing.T) {
	path := filepath.Join(t.TempDir(), "acl-audit-log.log")
	write(t, path, dropLine) // before the agent starts: not counted
	tail := NewACLTail(path)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(dropLine + denyLine + allowLine + otherLine + `partial name="NP:shop:Ingress", verdict=drop`)
	f.Close()
	got, err := tail.Drops()
	if err != nil || got["shop"] != 2 || got["batch"] != 1 || len(got) != 2 {
		t.Fatalf("drops = %v %v, want shop 2 and batch 1", got, err)
	}
	if got, _ := tail.Drops(); len(got) != 0 {
		t.Fatalf("drops read twice: %v", got)
	}
	// Rotation: a new, shorter file is read from its start.
	os.Remove(path)
	write(t, path, otherLine)
	if got, _ := tail.Drops(); got["batch"] != 1 || len(got) != 1 {
		t.Fatalf("after rotation drops = %v", got)
	}
	os.Remove(path)
	if got, err := tail.Drops(); err != nil || got != nil {
		t.Fatalf("missing log = %v %v", got, err)
	}
}

func TestReportToStore(t *testing.T) {
	at := time.Date(2026, 11, 28, 10, 0, 0, 0, time.UTC)
	report := Report{Node: "n1", At: at, NodeUsed: 2816, NodeMax: 4194304,
		Pods:  []Pod{{Namespace: "shop", Name: "api-1", PIDs: PodPIDs{Used: 4000, Limit: 4096}}},
		Drops: map[string]int{"shop": 3}}
	other := Report{Node: "n2", At: at, Drops: map[string]int{"shop": 2}}
	store := NewStore()
	for _, r := range []Report{report, other} {
		for _, rm := range r.Request().ResourceMetrics {
			if !store.Accept(rm) {
				t.Fatal("agent resource not accepted")
			}
		}
	}
	now := at.Add(30 * time.Second)
	if got := store.Readings(KindNodePIDs, now); len(got) != 1 || got[0].Target != "n1" || got[0].Value != 2816 || got[0].Capacity != 4194304 {
		t.Fatalf("node readings = %+v", got)
	}
	if got := store.Readings(KindPodPIDs, now); len(got) != 1 || got[0].Target != "shop/api-1" || got[0].Value != 4000 || got[0].Capacity != 4096 {
		t.Fatalf("pod readings = %+v", got)
	}
	if got := store.Readings(KindPolicyDrops, now); len(got) != 1 || got[0].Target != "shop" || got[0].Value != 5 {
		t.Fatalf("drop readings = %+v, want 5 summed over both nodes", got)
	}
	if store.Accept(models.OTLPResourceMetrics{Resource: models.OTLPResource{Attributes: []models.OTLPAttribute{attr("k8s.node.name", "n1")}}}) {
		t.Fatal("accepted a resource the agent did not send")
	}
	if got := store.Readings(KindNodePIDs, at.Add(5*time.Minute)); len(got) != 0 {
		t.Fatalf("stale readings kept: %+v", got)
	}
}
