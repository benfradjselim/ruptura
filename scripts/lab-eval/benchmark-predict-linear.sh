#!/bin/bash
# Scores predict_linear rules on the failures recorded by the lab Prometheus.
# Usage: /root/lab.sh < scripts/lab-eval/benchmark-predict-linear.sh
export KUBECONFIG=/etc/kubernetes/static-pod-resources/kube-apiserver-certs/secrets/node-kubeconfigs/localhost.kubeconfig
cat > /tmp/bench.py <<'P'
import json,subprocess,time,urllib.parse as U
HOURS,STEP=6,60
now=int(time.time()); start=now-HOURS*3600
def q(expr):
    url='http://localhost:9090/api/v1/query_range?'+U.urlencode(dict(query=expr,start=start,end=now,step=STEP))
    o=subprocess.check_output(['oc','exec','-n','openshift-monitoring','prometheus-k8s-0','-c','prometheus','--','curl','-s','-m','50',url],stderr=subprocess.DEVNULL)
    d=json.loads(o)
    if 'data' not in d: raise SystemExit(d)
    return d['data']['result']
NS='demo-shop'
rst=q(f'sum by (pod)(kube_pod_container_status_restarts_total{{namespace="{NS}",pod=~"leak-orders.*"}})')
ooms=[]
for s in rst:
    v=s['values']
    ooms+=[int(b[0]) for a,b in zip(v,v[1:]) if float(b[1])>float(a[1])]
ooms=sorted(ooms); print('leak OOMs in window',len(ooms))
USE='sum by (namespace,pod)(container_memory_working_set_bytes{container!="",container!="POD"})'
LIM='sum by (namespace,pod)(kube_pod_container_resource_limits{resource="memory"})'
print('window horizon | caught leads(min) | leak fires total | other pods firing')
for w in ('5m','10m','15m','30m'):
  for h in (600,1200,1800):
    r=q(f'(predict_linear(({USE})[{w}:30s],{h}) > {LIM}) and ({USE} > 0)')
    leak=sorted(int(t) for s in r if s['metric'].get('pod','').startswith('leak-orders') for t,_ in s['values'])
    other=sorted({s['metric'].get('namespace','')+'/'+s['metric'].get('pod','') for s in r if not s['metric'].get('pod','').startswith('leak-orders')})
    leads=[]; prev=start
    for o in ooms:
        f=[t for t in leak if prev<t<o]
        leads.append(round((o-f[0])/60,1) if f else None); prev=o
    ok=sum(1 for x in leads if x and x>=10)
    print(f'{w:>4} {h//60:>3}m | {ok}/{len(ooms)} {leads} | {len(leak)} | {len(other)} {other[:6]}')
P
python3 /tmp/bench.py
