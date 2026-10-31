curl -s -m 30 -H "Authorization: Bearer $(cat /var/tmp/.k)" http://172.30.10.124:80/api/v2/alerts -o /tmp/al3.json
python3 - <<'P'
import json,collections as C
a=json.load(open('/tmp/al3.json'))
a=[x for x in a if x['timestamp'][:19]>='2026-10-31T21:53:40']
print('alerts',len(a),'per min over 45',round(len(a)/45,2),'span',min(x['timestamp'] for x in a)[:19] if a else '',max(x['timestamp'] for x in a)[:19] if a else '')
print(C.Counter(x['metric'] for x in a).most_common(10)); print(C.Counter(x['method'] for x in a).most_common()); print(C.Counter(x['severity'] for x in a).most_common())
s=C.Counter((x['host'],x['metric']) for x in a); print('series',len(s),s.most_common(6))
print('by 5min',sorted(C.Counter(x['timestamp'][11:15] for x in a).items()))
P
python3 - <<'P'
import json
last=[json.loads(l) for l in open('/var/tmp/fleet.jsonl')][-1]
print(last['t'],{k:last['fleet'].get(k) for k in('total_hosts','healthy_hosts','degraded_hosts','critical_hosts')})
for h in last['fleet']['hosts']:
    if h['host'].split('/')[0]=='demo-shop': print(h['host'],h['state'],h.get('fused_rupture_index'),h.get('active_alerts'),h.get('etf_seconds'),h.get('failure_mode'))
P
tail -2 /var/tmp/pods.log | cut -c1-300
export KUBECONFIG=/etc/kubernetes/static-pod-resources/kube-apiserver-certs/secrets/node-kubeconfigs/localhost.kubeconfig; oc get pods -n ruptura-h --no-headers | grep '^ruptura-[0-9a-f]*-'
