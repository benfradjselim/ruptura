package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/benfradjselim/ohe/internal/alerter"
	"github.com/benfradjselim/ohe/internal/analyzer"
	"github.com/benfradjselim/ohe/internal/api"
	"github.com/benfradjselim/ohe/internal/predictor"
	"github.com/benfradjselim/ohe/internal/processor"
	"github.com/benfradjselim/ohe/internal/storage"
	"github.com/benfradjselim/ohe/pkg/models"
)

// setupServerWithAuth creates an httptest.Server with authentication enabled.
func setupServerWithAuth(t *testing.T) *httptest.Server {
	t.Helper()
	dir, err := os.MkdirTemp("", "ohe-api-auth-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	store, err := storage.Open(dir)
	if err != nil {
		t.Fatalf("Open storage: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	proc := processor.NewProcessor(1000)
	ana := analyzer.NewAnalyzer()
	pred := predictor.NewPredictor()
	alrt := alerter.NewAlerter(100)

	handlers := api.NewHandlers(store, proc, ana, pred, alrt, "test-host", "test-secret-key", true)
	router := api.NewRouter(handlers, "test-secret-key", true, nil)
	return httptest.NewServer(router)
}

// loginHelper creates an admin via setup then logs in, returning the JWT token.
func loginHelper(t *testing.T, srv *httptest.Server, username, password string) string {
	t.Helper()
	payload := map[string]string{"username": username, "password": password}
	body, _ := json.Marshal(payload)
	resp, err := http.Post(srv.URL+"/api/v1/auth/setup", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	resp.Body.Close()

	loginBody, _ := json.Marshal(models.LoginRequest{Username: username, Password: password})
	resp2, err := http.Post(srv.URL+"/api/v1/auth/login", "application/json", bytes.NewReader(loginBody))
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d; want 200", resp2.StatusCode)
	}
	var result map[string]interface{}
	json.NewDecoder(resp2.Body).Decode(&result)
	data, ok := result["data"].(map[string]interface{})
	if !ok || data == nil {
		t.Fatalf("login response data is nil or wrong type: %+v", result)
	}
	token, _ := data["token"].(string)
	if token == "" {
		t.Fatalf("login returned empty token, response: %+v", result)
	}
	return token
}

func authGet(t *testing.T, srv *httptest.Server, path, token string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, srv.URL+path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	return resp
}

func authPost(t *testing.T, srv *httptest.Server, path, token string, body interface{}) *http.Response {
	t.Helper()
	var buf *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		buf = bytes.NewReader(b)
	} else {
		buf = bytes.NewReader(nil)
	}
	req, _ := http.NewRequest(http.MethodPost, srv.URL+path, buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	return resp
}

func authDo(t *testing.T, method, url, token string, body interface{}) *http.Response {
	t.Helper()
	var buf *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		buf = bytes.NewReader(b)
	} else {
		buf = bytes.NewReader(nil)
	}
	req, _ := http.NewRequest(method, url, buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	return resp
}

// mustGet is a test helper that fatalf's on HTTP error.
func mustGet(t *testing.T, url string) *http.Response {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	return resp
}

// mustPost is a test helper that fatalf's on HTTP error.
func mustPost(t *testing.T, url string, body []byte) *http.Response {
	t.Helper()
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	return resp
}

// mustDo is a test helper that fatalf's on HTTP error.
func mustDo(t *testing.T, req *http.Request) *http.Response {
	t.Helper()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", req.Method, req.URL, err)
	}
	return resp
}

// --- MetricGetHandler ---

func TestMetricGetHandler(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	batch := models.MetricBatch{
		Host: "mhost", Timestamp: time.Now(),
		Metrics: []models.Metric{
			{Name: "cpu_percent", Value: 55, Host: "mhost", Timestamp: time.Now()},
		},
	}
	body, _ := json.Marshal(batch)
	http.Post(srv.URL+"/api/v1/ingest", "application/json", bytes.NewReader(body))

	resp := mustGet(t, srv.URL+"/api/v1/metrics/cpu_percent?host=mhost&from=-1h")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d; want 200", resp.StatusCode)
	}
}

// --- AlertGetHandler / AlertDeleteHandler ---

func TestAlertGetAndDeleteExtended(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	batch := models.MetricBatch{
		Host: "alhost", Timestamp: time.Now(),
		Metrics: []models.Metric{
			{Name: "cpu_percent", Value: 95, Host: "alhost", Timestamp: time.Now()},
			{Name: "memory_percent", Value: 95, Host: "alhost", Timestamp: time.Now()},
		},
	}
	body, _ := json.Marshal(batch)
	http.Post(srv.URL+"/api/v1/ingest", "application/json", bytes.NewReader(body))

	resp := mustGet(t, srv.URL+"/api/v1/alerts")
	defer resp.Body.Close()
	var listResult map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&listResult)
	alerts, _ := listResult["data"].([]interface{})
	if len(alerts) == 0 {
		t.Skip("no alerts fired — skipping get/delete test")
	}
	id := alerts[0].(map[string]interface{})["id"].(string)

	resp2 := mustGet(t, srv.URL+"/api/v1/alerts/"+id)
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("get alert status = %d; want 200", resp2.StatusCode)
	}

	resp3 := mustGet(t, srv.URL+"/api/v1/alerts/nonexistent")
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusNotFound {
		t.Errorf("get nonexistent = %d; want 404", resp3.StatusCode)
	}

	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/v1/alerts/"+id, nil)
	resp4 := mustDo(t, req)
	defer resp4.Body.Close()
	if resp4.StatusCode != http.StatusNoContent {
		t.Errorf("delete alert = %d; want 204", resp4.StatusCode)
	}

	req5, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/v1/alerts/nonexistent", nil)
	resp5 := mustDo(t, req5)
	defer resp5.Body.Close()
	if resp5.StatusCode != http.StatusNotFound {
		t.Errorf("delete nonexistent = %d; want 404", resp5.StatusCode)
	}
}

// --- DashboardUpdateHandler / ExportHandler / ImportHandler ---

func TestDashboardUpdateExportImport(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	d := models.Dashboard{Name: "Original", Refresh: 30}
	body, _ := json.Marshal(d)
	resp := mustPost(t, srv.URL+"/api/v1/dashboards", body)
	defer resp.Body.Close()
	var created map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&created)
	id := created["data"].(map[string]interface{})["id"].(string)

	update := models.Dashboard{Name: "Updated", Refresh: 60}
	updateBody, _ := json.Marshal(update)
	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/v1/dashboards/"+id, bytes.NewReader(updateBody))
	req.Header.Set("Content-Type", "application/json")
	resp2 := mustDo(t, req)
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("update status = %d; want 200", resp2.StatusCode)
	}

	req3, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/v1/dashboards/nope", bytes.NewReader(updateBody))
	req3.Header.Set("Content-Type", "application/json")
	resp3 := mustDo(t, req3)
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusNotFound {
		t.Errorf("update nonexistent = %d; want 404", resp3.StatusCode)
	}

	resp4 := mustGet(t, srv.URL+"/api/v1/dashboards/"+id+"/export")
	defer resp4.Body.Close()
	if resp4.StatusCode != http.StatusOK {
		t.Errorf("export status = %d; want 200", resp4.StatusCode)
	}
	if !strings.Contains(resp4.Header.Get("Content-Disposition"), "attachment") {
		t.Error("export should set Content-Disposition: attachment")
	}

	resp5 := mustGet(t, srv.URL+"/api/v1/dashboards/nope/export")
	defer resp5.Body.Close()
	if resp5.StatusCode != http.StatusNotFound {
		t.Errorf("export nonexistent = %d; want 404", resp5.StatusCode)
	}

	imp := models.Dashboard{Name: "Imported", Refresh: 15}
	impBody, _ := json.Marshal(imp)
	resp6 := mustPost(t, srv.URL+"/api/v1/dashboards/import", impBody)
	defer resp6.Body.Close()
	if resp6.StatusCode != http.StatusCreated {
		t.Errorf("import status = %d; want 201", resp6.StatusCode)
	}
}

// --- LoginHandler / LogoutHandler / RefreshHandler ---

func TestLoginLogoutRefresh(t *testing.T) {
	srv := setupServerWithAuth(t)
	defer srv.Close()

	setupBody, _ := json.Marshal(map[string]string{"username": "admin2", "password": "adminpass1"})
	setupResp := mustPost(t, srv.URL+"/api/v1/auth/setup", setupBody)
	setupResp.Body.Close()

	loginBody, _ := json.Marshal(models.LoginRequest{Username: "admin2", Password: "adminpass1"})
	resp := mustPost(t, srv.URL+"/api/v1/auth/login", loginBody)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d; want 200", resp.StatusCode)
	}
	var loginResult map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&loginResult)
	dataField, ok := loginResult["data"].(map[string]interface{})
	if !ok || dataField == nil {
		t.Fatalf("login response data is nil or wrong type: %+v", loginResult)
	}
	token, _ := dataField["token"].(string)
	if token == "" {
		t.Fatal("login returned empty token")
	}

	wrongBody, _ := json.Marshal(models.LoginRequest{Username: "admin2", Password: "wrongpass"})
	resp2 := mustPost(t, srv.URL+"/api/v1/auth/login", wrongBody)
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Errorf("wrong password status = %d; want 401", resp2.StatusCode)
	}

	unknownBody, _ := json.Marshal(models.LoginRequest{Username: "nobody", Password: "pass"})
	resp3 := mustPost(t, srv.URL+"/api/v1/auth/login", unknownBody)
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusUnauthorized {
		t.Errorf("unknown user status = %d; want 401", resp3.StatusCode)
	}

	logoutReq, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/auth/logout", nil)
	logoutReq.Header.Set("Authorization", "Bearer "+token)
	resp4 := mustDo(t, logoutReq)
	defer resp4.Body.Close()
	if resp4.StatusCode != http.StatusOK {
		t.Errorf("logout status = %d; want 200", resp4.StatusCode)
	}

	refreshReq, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/auth/refresh", nil)
	refreshReq.Header.Set("Authorization", "Bearer "+token)
	resp5 := mustDo(t, refreshReq)
	defer resp5.Body.Close()
	if resp5.StatusCode != http.StatusOK {
		t.Errorf("refresh status = %d; want 200", resp5.StatusCode)
	}
}

// --- ReloadHandler ---

func TestReloadHandler(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/reload", nil)
	resp := mustDo(t, req)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("reload status = %d; want 200", resp.StatusCode)
	}
}

// --- DataSource full CRUD + test endpoint ---

func TestDataSourceFullCRUD(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	ds := map[string]interface{}{"name": "local", "type": "ohe"}
	body, _ := json.Marshal(ds)
	resp := mustPost(t, srv.URL+"/api/v1/datasources", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create datasource = %d; want 201", resp.StatusCode)
	}
	var created map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&created)
	id := created["data"].(map[string]interface{})["id"].(string)

	resp2 := mustGet(t, srv.URL+"/api/v1/datasources/"+id)
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("get datasource = %d; want 200", resp2.StatusCode)
	}

	resp3 := mustGet(t, srv.URL+"/api/v1/datasources/nope")
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusNotFound {
		t.Errorf("get nonexistent = %d; want 404", resp3.StatusCode)
	}

	updDS := map[string]interface{}{"name": "local-updated", "type": "ohe"}
	updBody, _ := json.Marshal(updDS)
	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/v1/datasources/"+id, bytes.NewReader(updBody))
	req.Header.Set("Content-Type", "application/json")
	resp4 := mustDo(t, req)
	defer resp4.Body.Close()
	if resp4.StatusCode != http.StatusOK {
		t.Errorf("update datasource = %d; want 200", resp4.StatusCode)
	}

	ssrfDS := map[string]interface{}{"name": "ssrf", "type": "prometheus", "url": "http://169.254.169.254/"}
	ssrfBody, _ := json.Marshal(ssrfDS)
	req5, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/v1/datasources/"+id, bytes.NewReader(ssrfBody))
	req5.Header.Set("Content-Type", "application/json")
	resp5 := mustDo(t, req5)
	defer resp5.Body.Close()
	if resp5.StatusCode == http.StatusOK {
		t.Error("SSRF URL in update should be rejected")
	}

	testReq, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/datasources/"+id+"/test", nil)
	resp6 := mustDo(t, testReq)
	defer resp6.Body.Close()
	_ = resp6.StatusCode

	testReq2, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/datasources/nope/test", nil)
	resp7 := mustDo(t, testReq2)
	defer resp7.Body.Close()
	if resp7.StatusCode != http.StatusNotFound {
		t.Errorf("test nonexistent datasource = %d; want 404", resp7.StatusCode)
	}

	delReq, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/v1/datasources/"+id, nil)
	resp8 := mustDo(t, delReq)
	defer resp8.Body.Close()
	if resp8.StatusCode != http.StatusNoContent {
		t.Errorf("delete datasource = %d; want 204", resp8.StatusCode)
	}

	delReq2, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/v1/datasources/nope", nil)
	resp9 := mustDo(t, delReq2)
	defer resp9.Body.Close()
	if resp9.StatusCode != http.StatusNoContent {
		t.Errorf("delete nonexistent datasource = %d; want 204", resp9.StatusCode)
	}
}

// --- User management ---

func TestUserCRUD(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	userPayload := map[string]string{"username": "alice", "password": "alicepass1", "role": "viewer"}
	body, _ := json.Marshal(userPayload)
	resp := mustPost(t, srv.URL+"/api/v1/auth/users", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("create user = %d; want 201", resp.StatusCode)
	}
	var created map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&created)
	data := created["data"].(map[string]interface{})
	if data["password"] != nil && data["password"] != "" {
		t.Error("password hash must not be returned in response")
	}

	badPayload := map[string]string{"username": "a:b", "password": "goodpass1"}
	badBody, _ := json.Marshal(badPayload)
	resp2 := mustPost(t, srv.URL+"/api/v1/auth/users", badBody)
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusBadRequest {
		t.Errorf("invalid username = %d; want 400", resp2.StatusCode)
	}

	shortPayload := map[string]string{"username": "bob", "password": "short"}
	shortBody, _ := json.Marshal(shortPayload)
	resp3 := mustPost(t, srv.URL+"/api/v1/auth/users", shortBody)
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusBadRequest {
		t.Errorf("short password = %d; want 400", resp3.StatusCode)
	}

	resp4 := mustGet(t, srv.URL+"/api/v1/auth/users")
	defer resp4.Body.Close()
	if resp4.StatusCode != http.StatusOK {
		t.Errorf("list users = %d; want 200", resp4.StatusCode)
	}

	resp5 := mustGet(t, srv.URL+"/api/v1/auth/users/alice")
	defer resp5.Body.Close()
	if resp5.StatusCode != http.StatusOK {
		t.Errorf("get user = %d; want 200", resp5.StatusCode)
	}

	resp6 := mustGet(t, srv.URL+"/api/v1/auth/users/nope")
	defer resp6.Body.Close()
	if resp6.StatusCode != http.StatusNotFound {
		t.Errorf("get nonexistent user = %d; want 404", resp6.StatusCode)
	}

	delReq, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/v1/auth/users/alice", nil)
	resp7 := mustDo(t, delReq)
	defer resp7.Body.Close()
	if resp7.StatusCode != http.StatusNoContent {
		t.Errorf("delete user = %d; want 204", resp7.StatusCode)
	}

	delReq2, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/v1/auth/users/nope", nil)
	resp8 := mustDo(t, delReq2)
	defer resp8.Body.Close()
	if resp8.StatusCode != http.StatusNoContent {
		t.Errorf("delete nonexistent user = %d; want 204", resp8.StatusCode)
	}
}

// --- AuthMiddleware ---

func TestAuthMiddlewareProtectsEndpoints(t *testing.T) {
	srv := setupServerWithAuth(t)
	defer srv.Close()

	resp := authGet(t, srv, "/api/v1/metrics", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("no token status = %d; want 401", resp.StatusCode)
	}

	resp2 := authGet(t, srv, "/api/v1/metrics", "not-a-valid-token")
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Errorf("invalid token status = %d; want 401", resp2.StatusCode)
	}

	token := loginHelper(t, srv, "sysadmin", "adminpass99")
	resp3 := authGet(t, srv, "/api/v1/metrics", token)
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusOK {
		t.Errorf("valid token status = %d; want 200", resp3.StatusCode)
	}
}

// --- DataSourceTestHandler error-path coverage ---

func TestDataSourceTestHandlerLive(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	ssrfDS := map[string]interface{}{"name": "bad", "type": "prometheus", "url": "http://169.254.169.254/"}
	ssrfBody, _ := json.Marshal(ssrfDS)
	resp0 := mustPost(t, srv.URL+"/api/v1/datasources", ssrfBody)
	resp0.Body.Close()
	if resp0.StatusCode != http.StatusBadRequest {
		t.Errorf("SSRF create = %d; want 400", resp0.StatusCode)
	}

	ds := map[string]interface{}{"name": "local-src", "type": "ohe"}
	body, _ := json.Marshal(ds)
	resp := mustPost(t, srv.URL+"/api/v1/datasources", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create datasource = %d; want 201", resp.StatusCode)
	}
	var created map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&created)
	id := created["data"].(map[string]interface{})["id"].(string)

	testReq, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/datasources/"+id+"/test", nil)
	resp2 := mustDo(t, testReq)
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusBadRequest {
		t.Errorf("test empty-URL datasource = %d; want 400", resp2.StatusCode)
	}

	testReq2, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/datasources/nope/test", nil)
	resp3 := mustDo(t, testReq2)
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusNotFound {
		t.Errorf("test nonexistent datasource = %d; want 404", resp3.StatusCode)
	}
}

// --- KPIListHandler with host param ---

func TestKPIListHandlerWithHost(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	resp := mustGet(t, srv.URL+"/api/v1/kpis?host=no-such-host")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("kpi list status = %d; want 404", resp.StatusCode)
	}
}

// --- Ensure password hash is never exposed on user endpoints ---

func TestUserPasswordNeverExposed(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	payload := map[string]string{"username": "charlie", "password": "charliepass1"}
	body, _ := json.Marshal(payload)
	http.Post(srv.URL+"/api/v1/auth/setup", "application/json", bytes.NewReader(body))

	resp := mustGet(t, srv.URL+"/api/v1/auth/users/charlie")
	defer resp.Body.Close()
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	user := result["data"].(map[string]interface{})
	if pw, ok := user["password"]; ok && fmt.Sprintf("%v", pw) != "" {
		t.Errorf("password hash exposed in response: %v", pw)
	}
}
