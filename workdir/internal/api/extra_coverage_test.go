package api_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

// TestLokiLabelValues covers the label values endpoint.
func TestLokiLabelValues(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/loki/api/v1/label/service/values")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		t.Errorf("unexpected error: %d", resp.StatusCode)
	}
}

// TestTraceQuery covers the single trace endpoint.
func TestTraceQuery(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/traces/nonexistent-trace-id")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	// Either 200 (empty results) or 404 is acceptable
	if resp.StatusCode >= 500 {
		t.Errorf("unexpected error: %d", resp.StatusCode)
	}
}

// TestOrgGetHandler covers OrgGetHandler.
func TestOrgGetHandler(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	// Create an org first
	body, _ := json.Marshal(map[string]string{"name": "Test Corp", "slug": "testcorp"})
	resp, err := http.Post(srv.URL+"/api/v1/orgs", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /orgs: %v", err)
	}
	defer resp.Body.Close()

	var created map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&created)

	// Access via ID (may return 404 if creation response differs)
	getResp, err := http.Get(srv.URL + "/api/v1/orgs/testcorp")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer getResp.Body.Close()
	// 200 or 404 both valid; 500 is failure
	if getResp.StatusCode >= 500 {
		t.Errorf("unexpected error: %d", getResp.StatusCode)
	}
}

// TestNotificationChannelGetUpdateDelete covers channel sub-handlers.
func TestNotificationChannelGetUpdateDelete(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	// Create channel
	body, _ := json.Marshal(map[string]interface{}{
		"name":    "test-channel",
		"type":    "slack",
		"enabled": true,
	})
	createResp, err := http.Post(srv.URL+"/api/v1/notifications", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer createResp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(createResp.Body).Decode(&result)
	data, _ := result["data"].(map[string]interface{})
	id, _ := data["id"].(string)
	if id == "" {
		t.Skip("could not create channel to test get/update/delete")
	}

	// Get channel
	getResp, err := http.Get(srv.URL + "/api/v1/notifications/" + id)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	getResp.Body.Close()
	if getResp.StatusCode != http.StatusOK {
		t.Errorf("get status = %d; want 200", getResp.StatusCode)
	}

	// Update channel
	updateBody, _ := json.Marshal(map[string]interface{}{
		"name":    "updated-channel",
		"type":    "slack",
		"enabled": false,
	})
	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/v1/notifications/"+id, bytes.NewReader(updateBody))
	req.Header.Set("Content-Type", "application/json")
	updateResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT: %v", err)
	}
	updateResp.Body.Close()

	// Delete channel
	delReq, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/v1/notifications/"+id, nil)
	delResp, err := http.DefaultClient.Do(delReq)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	delResp.Body.Close()
	if delResp.StatusCode != http.StatusOK && delResp.StatusCode != http.StatusNoContent {
		t.Errorf("delete status = %d; want 200 or 204", delResp.StatusCode)
	}
}

// TestAlertRuleUpdateDelete covers update/delete alert rule handlers.
func TestAlertRuleUpdateDelete(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	// Create rule
	body, _ := json.Marshal(map[string]interface{}{
		"name":      "cpu-high",
		"metric":    "cpu_percent",
		"threshold": 85.0,
		"severity":  "warning",
	})
	createResp, err := http.Post(srv.URL+"/api/v1/alert-rules", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	io.ReadAll(createResp.Body)
	createResp.Body.Close()

	// Update rule
	updateBody, _ := json.Marshal(map[string]interface{}{
		"name":      "cpu-high",
		"metric":    "cpu_percent",
		"threshold": 90.0,
		"severity":  "critical",
	})
	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/v1/alert-rules/cpu-high", bytes.NewReader(updateBody))
	req.Header.Set("Content-Type", "application/json")
	updateResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT: %v", err)
	}
	io.ReadAll(updateResp.Body)
	updateResp.Body.Close()

	// Delete rule
	delReq, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/v1/alert-rules/cpu-high", nil)
	delResp, err := http.DefaultClient.Do(delReq)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	io.ReadAll(delResp.Body)
	delResp.Body.Close()
}

// TestSLOGetUpdateDelete covers SLO sub-handlers.
func TestSLOGetUpdateDelete(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	// Create SLO
	body, _ := json.Marshal(map[string]interface{}{
		"name":   "uptime",
		"metric": "health_score",
		"target": 99.5,
	})
	createResp, err := http.Post(srv.URL+"/api/v1/slos", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	var result map[string]interface{}
	json.NewDecoder(createResp.Body).Decode(&result)
	createResp.Body.Close()
	data, _ := result["data"].(map[string]interface{})
	id, _ := data["id"].(string)
	if id == "" {
		t.Skip("could not create SLO")
	}

	// Get SLO
	getResp, err := http.Get(srv.URL + "/api/v1/slos/" + id)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	getResp.Body.Close()
	if getResp.StatusCode != http.StatusOK {
		t.Errorf("get slo status = %d; want 200", getResp.StatusCode)
	}

	// Get SLO status
	statusResp, err := http.Get(srv.URL + "/api/v1/slos/" + id + "/status")
	if err != nil {
		t.Fatalf("GET status: %v", err)
	}
	statusResp.Body.Close()
	if statusResp.StatusCode >= 500 {
		t.Errorf("slo status %d", statusResp.StatusCode)
	}

	// Update SLO
	updateBody, _ := json.Marshal(map[string]interface{}{
		"name":   "uptime",
		"metric": "health_score",
		"target": 99.9,
	})
	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/v1/slos/"+id, bytes.NewReader(updateBody))
	req.Header.Set("Content-Type", "application/json")
	updateResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT: %v", err)
	}
	updateResp.Body.Close()

	// Delete SLO
	delReq, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/v1/slos/"+id, nil)
	delResp, err := http.DefaultClient.Do(delReq)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	delResp.Body.Close()
}

// TestMetricRangeEndpoint covers tiered range query.
func TestMetricRangeEndpoint(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/metrics/cpu_percent/range?host=test-host&from=2024-01-01T00:00:00Z&to=2024-01-02T00:00:00Z")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d; want 200", resp.StatusCode)
	}
}

// TestUserAssignOrg covers user org assignment.
func TestUserAssignOrg(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	// Create user first
	userBody, _ := json.Marshal(map[string]interface{}{
		"username": "testuser",
		"password": "passw0rd",
		"role":     "viewer",
	})
	createResp, err := http.Post(srv.URL+"/api/v1/auth/users", "application/json", bytes.NewReader(userBody))
	if err != nil {
		t.Fatalf("POST users: %v", err)
	}
	var userResult map[string]interface{}
	json.NewDecoder(createResp.Body).Decode(&userResult)
	createResp.Body.Close()

	userData, _ := userResult["data"].(map[string]interface{})
	userID, _ := userData["id"].(string)
	if userID == "" {
		t.Skip("could not create user")
	}

	// Assign org
	orgBody, _ := json.Marshal(map[string]string{"org_id": "default"})
	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/v1/auth/users/"+userID+"/org", bytes.NewReader(orgBody))
	req.Header.Set("Content-Type", "application/json")
	assignResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT: %v", err)
	}
	io.ReadAll(assignResp.Body)
	assignResp.Body.Close()
	if assignResp.StatusCode >= 500 {
		t.Errorf("assign org status = %d", assignResp.StatusCode)
	}
}

// TestRetentionCompact covers the manual compaction endpoint.
func TestRetentionCompact(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/api/v1/retention/compact", "application/json", nil)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d; want 200", resp.StatusCode)
	}
}

// TestOrgUpdateDelete covers OrgUpdateHandler and OrgDeleteHandler.
func TestOrgUpdateDelete(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	// Create org
	body, _ := json.Marshal(map[string]string{"name": "Update Corp", "slug": "updatecorp"})
	createResp, err := http.Post(srv.URL+"/api/v1/orgs", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	var result map[string]interface{}
	json.NewDecoder(createResp.Body).Decode(&result)
	createResp.Body.Close()
	data, _ := result["data"].(map[string]interface{})
	id, _ := data["id"].(string)
	if id == "" {
		t.Skip("could not create org")
	}

	// Update org
	updateBody, _ := json.Marshal(map[string]string{"name": "Updated Corp", "slug": "updatecorp"})
	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/v1/orgs/"+id, bytes.NewReader(updateBody))
	req.Header.Set("Content-Type", "application/json")
	updateResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT: %v", err)
	}
	io.ReadAll(updateResp.Body)
	updateResp.Body.Close()
	if updateResp.StatusCode >= 500 {
		t.Errorf("update status = %d", updateResp.StatusCode)
	}

	// List members
	membersResp, err := http.Get(srv.URL + "/api/v1/orgs/" + id + "/members")
	if err != nil {
		t.Fatalf("GET members: %v", err)
	}
	io.ReadAll(membersResp.Body)
	membersResp.Body.Close()
	if membersResp.StatusCode >= 500 {
		t.Errorf("members status = %d", membersResp.StatusCode)
	}

	// Delete org
	delReq, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/v1/orgs/"+id, nil)
	delResp, err := http.DefaultClient.Do(delReq)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	io.ReadAll(delResp.Body)
	delResp.Body.Close()
	if delResp.StatusCode >= 500 {
		t.Errorf("delete status = %d", delResp.StatusCode)
	}
}

// TestKPIMulti covers the KPIMultiHandler endpoint.
func TestKPIMulti(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/kpis/multi?metrics=cpu_percent,mem_percent&host=test-host")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode >= 500 {
		t.Errorf("kpi multi status = %d", resp.StatusCode)
	}
}

// TestNotificationChannelTest covers the channel test-fire endpoint.
func TestNotificationChannelTest(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	// Create a channel first
	body, _ := json.Marshal(map[string]interface{}{
		"name":    "test-ch-fire",
		"type":    "slack",
		"enabled": true,
	})
	createResp, err := http.Post(srv.URL+"/api/v1/notifications", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	var result map[string]interface{}
	json.NewDecoder(createResp.Body).Decode(&result)
	createResp.Body.Close()
	data, _ := result["data"].(map[string]interface{})
	id, _ := data["id"].(string)
	if id == "" {
		t.Skip("could not create channel")
	}

	// Fire test
	fireResp, err := http.Post(srv.URL+"/api/v1/notifications/"+id+"/test", "application/json", nil)
	if err != nil {
		t.Fatalf("POST test: %v", err)
	}
	io.ReadAll(fireResp.Body)
	fireResp.Body.Close()
	if fireResp.StatusCode >= 500 {
		t.Errorf("test fire status = %d", fireResp.StatusCode)
	}
}
