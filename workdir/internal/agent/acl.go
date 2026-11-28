package agent

import (
	"bufio"
	"io"
	"os"
	"regexp"
	"strings"
)

// aclName finds the namespace in the ACL name of an OVN-Kubernetes network
// policy, such as name="NP:shop:allow-web:Ingress:0" or the namespace's
// default deny name="NP:shop:Ingress".
var aclName = regexp.MustCompile(`name="NP:([^:"]+)`)

// ACLTail follows the OVN-Kubernetes ACL audit log of a node. The log only
// holds the namespaces that turned audit logging on with the
// k8s.ovn.org/acl-logging annotation.
type ACLTail struct {
	path   string
	offset int64
	file   os.FileInfo
}

// NewACLTail follows path from its current end, so old drops are not counted.
func NewACLTail(path string) *ACLTail {
	t := &ACLTail{path: path}
	if info, err := os.Stat(path); err == nil {
		t.offset, t.file = info.Size(), info
	}
	return t
}

// Drops returns the packets dropped by network policies per namespace since
// the last call. A rotated or truncated log is read from its start. A missing
// log returns nothing.
func (t *ACLTail) Drops() (map[string]int, error) {
	info, err := os.Stat(t.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if t.file == nil || !os.SameFile(t.file, info) || info.Size() < t.offset {
		t.offset = 0
	}
	t.file = info
	f, err := os.Open(t.path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if _, err := f.Seek(t.offset, io.SeekStart); err != nil {
		return nil, err
	}
	drops := map[string]int{}
	reader := bufio.NewReader(f)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break // a partial last line is read at the next call
		}
		t.offset += int64(len(line))
		if !strings.Contains(line, "verdict=drop") {
			continue
		}
		if m := aclName.FindStringSubmatch(line); m != nil {
			drops[m[1]]++
		}
	}
	return drops, nil
}
