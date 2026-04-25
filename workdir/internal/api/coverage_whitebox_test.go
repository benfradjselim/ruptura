package api

// White-box coverage tests for unexported helpers that are still at 0%.
// Lives in package api (not api_test) to access unexported symbols.

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/benfradjselim/kairo-core/pkg/models"
	"golang.org/x/crypto/bcrypt"
)

// --- widgetsToPrediction ---

func TestWidgetsToPrediction_ConvertsTimeseriesAndGauge(t *testing.T) {
	widgets := []models.Widget{
		{Title: "CPU", Type: "timeseries"},
		{Title: "Mem", Type: "gauge"},
		{Title: "Stat", Type: "stat"},
		{Title: "Text", Type: "text"},
	}
	out := widgetsToPrediction(widgets)
	if len(out) != len(widgets) {
		t.Fatalf("got %d widgets, want %d", len(out), len(widgets))
	}
	for i, w := range out[:3] {
		if w.Type != "prediction" {
			t.Errorf("widget[%d] type = %q, want prediction", i, w.Type)
		}
		if w.Options["horizon"] != "60" {
			t.Errorf("widget[%d] horizon = %q, want 60", i, w.Options["horizon"])
		}
	}
	if out[3].Type != "text" {
		t.Errorf("text widget type mutated, got %q", out[3].Type)
	}
}

func TestWidgetsToPrediction_NilOptionsPopulated(t *testing.T) {
	widgets := []models.Widget{{Type: "gauge"}} // Options is nil
	out := widgetsToPrediction(widgets)
	if out[0].Options == nil {
		t.Error("Options should not be nil after conversion")
	}
	if out[0].Options["horizon"] != "60" {
		t.Errorf("horizon = %q, want 60", out[0].Options["horizon"])
	}
}

func TestWidgetsToPrediction_EmptySlice(t *testing.T) {
	out := widgetsToPrediction(nil)
	if len(out) != 0 {
		t.Errorf("expected empty, got %d", len(out))
	}
}

// --- writePrometheusKPI ---

func TestWritePrometheusKPI_Output(t *testing.T) {
	var buf strings.Builder
	ts := time.Unix(1700000000, 0)
	writePrometheusKPI(&buf, "ohe_stress", 0.42, "elevated", "web-01", ts)
	got := buf.String()

	if !strings.Contains(got, "# HELP ohe_stress") {
		t.Errorf("missing HELP line, got:\n%s", got)
	}
	if !strings.Contains(got, "# TYPE ohe_stress gauge") {
		t.Errorf("missing TYPE line")
	}
	if !strings.Contains(got, `ohe_stress{host="web-01"`) {
		t.Errorf("missing metric line with host label")
	}
}

func TestWritePrometheusKPI_MultipleCallsAppend(t *testing.T) {
	var buf strings.Builder
	ts := time.Now()
	writePrometheusKPI(&buf, "ohe_stress", 0.1, "low", "h1", ts)
	writePrometheusKPI(&buf, "ohe_fatigue", 0.2, "low", "h1", ts)
	got := buf.String()
	if !strings.Contains(got, "ohe_stress") || !strings.Contains(got, "ohe_fatigue") {
		t.Errorf("both KPIs should be present, got:\n%s", got)
	}
}

// --- slugify ---

func TestSlugify(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"My Service", "my-service"},
		{"test_name", "test-name"},
		{"Hello-World", "hello-world"},
		{"  trim  ", "trim"},
		{"Café", "caf"},      // non-ASCII dropped
		{"a1b2c3", "a1b2c3"}, // all valid chars
	}
	for _, tc := range cases {
		got := slugify(tc.in)
		if got != tc.want {
			t.Errorf("slugify(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// --- skipField ---

func TestSkipField_Varint(t *testing.T) {
	// encode varint 300 = 0xAC 0x02
	b := []byte{0xAC, 0x02, 0xFF}
	rest, err := skipField(b, 0)
	if err != nil {
		t.Fatalf("varint skip: %v", err)
	}
	if !bytes.Equal(rest, []byte{0xFF}) {
		t.Errorf("rest = %v, want [0xFF]", rest)
	}
}

func TestSkipField_64bit(t *testing.T) {
	b := make([]byte, 9)
	b[8] = 0xAB
	rest, err := skipField(b, 1)
	if err != nil {
		t.Fatalf("64-bit skip: %v", err)
	}
	if len(rest) != 1 || rest[0] != 0xAB {
		t.Errorf("rest = %v", rest)
	}
}

func TestSkipField_64bit_TooShort(t *testing.T) {
	_, err := skipField([]byte{1, 2, 3}, 1)
	if err != io.ErrUnexpectedEOF {
		t.Errorf("expected ErrUnexpectedEOF, got %v", err)
	}
}

func TestSkipField_LengthDelimited(t *testing.T) {
	// length=3, data=[1,2,3], trailing [0xFF]
	b := []byte{0x03, 0x01, 0x02, 0x03, 0xFF}
	rest, err := skipField(b, 2)
	if err != nil {
		t.Fatalf("length-delimited skip: %v", err)
	}
	if !bytes.Equal(rest, []byte{0xFF}) {
		t.Errorf("rest = %v", rest)
	}
}

func TestSkipField_32bit(t *testing.T) {
	b := make([]byte, 5)
	b[4] = 0xCC
	rest, err := skipField(b, 5)
	if err != nil {
		t.Fatalf("32-bit skip: %v", err)
	}
	if len(rest) != 1 || rest[0] != 0xCC {
		t.Errorf("rest = %v", rest)
	}
}

func TestSkipField_32bit_TooShort(t *testing.T) {
	_, err := skipField([]byte{1, 2}, 5)
	if err != io.ErrUnexpectedEOF {
		t.Errorf("expected ErrUnexpectedEOF, got %v", err)
	}
}

func TestSkipField_UnknownWireType(t *testing.T) {
	_, err := skipField([]byte{0x01}, 7)
	if err == nil {
		t.Error("expected error for unknown wire type")
	}
}

// --- SetAPIKeyLookup / SetTokenRevokedChecker ---

func TestSetAPIKeyLookup_Callable(t *testing.T) {
	called := false
	SetAPIKeyLookup(func(orgID, rawKey string) (*JWTClaims, bool) {
		called = true
		return nil, false
	})
	// Verify it was stored without panicking; reset to nil.
	authMiddlewareMu.Lock()
	fn := apiKeyLookupFn
	authMiddlewareMu.Unlock()
	if fn == nil {
		t.Error("apiKeyLookupFn should not be nil after SetAPIKeyLookup")
	}
	_ = called
	SetAPIKeyLookup(nil)
}

func TestSetTokenRevokedChecker_Callable(t *testing.T) {
	SetTokenRevokedChecker(func(jti string) bool { return false })
	authMiddlewareMu.Lock()
	fn := tokenRevokedFn
	authMiddlewareMu.Unlock()
	if fn == nil {
		t.Error("tokenRevokedFn should not be nil after SetTokenRevokedChecker")
	}
	SetTokenRevokedChecker(nil)
}

// --- evictStaleBuckets ---

func TestEvictStaleBuckets_RemovesStaleKeepsRecent(t *testing.T) {
	loginLimiter.mu.Lock()
	// seed two buckets: one stale, one fresh
	loginLimiter.buckets["stale-ip"] = &bucket{tokens: 5, lastSeen: time.Now().Add(-20 * time.Minute)}
	loginLimiter.buckets["fresh-ip"] = &bucket{tokens: 5, lastSeen: time.Now()}
	evictStaleBuckets()
	_, staleOK := loginLimiter.buckets["stale-ip"]
	_, freshOK := loginLimiter.buckets["fresh-ip"]
	delete(loginLimiter.buckets, "stale-ip")
	delete(loginLimiter.buckets, "fresh-ip")
	loginLimiter.mu.Unlock()

	if staleOK {
		t.Error("stale-ip should have been evicted")
	}
	if !freshOK {
		t.Error("fresh-ip should NOT have been evicted")
	}
}

// --- ValidateAPIKey ---

type mockAPIKeyStore struct {
	keys map[string]models.APIKey
	err  error
}

func (m *mockAPIKeyStore) LookupAPIKeyByPrefix(prefix string, dest interface{}) error {
	if m.err != nil {
		return m.err
	}
	k, ok := m.keys[prefix]
	if !ok {
		return fmt.Errorf("not found")
	}
	*dest.(*models.APIKey) = k
	return nil
}

func TestValidateAPIKey_TooShort(t *testing.T) {
	_, ok := ValidateAPIKey(&mockAPIKeyStore{}, "ohe_short")
	if ok {
		t.Error("too-short key should be rejected")
	}
}

func TestValidateAPIKey_WrongPrefix(t *testing.T) {
	_, ok := ValidateAPIKey(&mockAPIKeyStore{}, "xyz_abcdefghijklmnop")
	if ok {
		t.Error("wrong prefix should be rejected")
	}
}

func TestValidateAPIKey_StoreError(t *testing.T) {
	store := &mockAPIKeyStore{err: fmt.Errorf("db error")}
	_, ok := ValidateAPIKey(store, "ohe_abcdefghijklmnop")
	if ok {
		t.Error("store error should reject key")
	}
}

func TestValidateAPIKey_NotActive(t *testing.T) {
	rawKey := "ohe_abcdefgh12345678"
	prefix := rawKey[:12]
	store := &mockAPIKeyStore{keys: map[string]models.APIKey{
		prefix: {ID: "k1", Active: false, OrgID: "default", Role: "viewer"},
	}}
	_, ok := ValidateAPIKey(store, rawKey)
	if ok {
		t.Error("inactive key should be rejected")
	}
}

func TestValidateAPIKey_Expired(t *testing.T) {
	rawKey := "ohe_abcdefgh12345678"
	prefix := rawKey[:12]
	store := &mockAPIKeyStore{keys: map[string]models.APIKey{
		prefix: {ID: "k1", Active: true, ExpiresAt: time.Now().Add(-time.Hour), OrgID: "default", Role: "viewer"},
	}}
	_, ok := ValidateAPIKey(store, rawKey)
	if ok {
		t.Error("expired key should be rejected")
	}
}

func TestValidateAPIKey_BCryptMismatch(t *testing.T) {
	rawKey := "ohe_abcdefgh12345678"
	prefix := rawKey[:12]
	wrongHash, _ := bcrypt.GenerateFromPassword([]byte("different_key"), 4)
	store := &mockAPIKeyStore{keys: map[string]models.APIKey{
		prefix: {ID: "k1", Active: true, KeyHash: string(wrongHash), OrgID: "default", Role: "viewer"},
	}}
	_, ok := ValidateAPIKey(store, rawKey)
	if ok {
		t.Error("bcrypt mismatch should reject key")
	}
}

func TestValidateAPIKey_Valid(t *testing.T) {
	rawKey := "ohe_abcdefgh12345678"
	prefix := rawKey[:12]
	hash, _ := bcrypt.GenerateFromPassword([]byte(rawKey), 4)
	store := &mockAPIKeyStore{keys: map[string]models.APIKey{
		prefix: {ID: "k1", Active: true, KeyHash: string(hash), OrgID: "default", Role: "operator", Name: "ci"},
	}}
	claims, ok := ValidateAPIKey(store, rawKey)
	if !ok {
		t.Error("valid key should be accepted")
	}
	if claims == nil || claims.Role != "operator" {
		t.Errorf("unexpected claims: %+v", claims)
	}
}

// --- Threshold label helpers (handlers_explain.go) ---

func TestStressThresholdLabel(t *testing.T) {
	cases := []struct{ v float64; want string }{
		{0.9, "≥0.8 (Panic)"},
		{0.7, "≥0.6 (Stressed)"},
		{0.5, "≥0.3 (Nervous)"},
		{0.1, "<0.3 (Calm)"},
	}
	for _, tc := range cases {
		if got := stressThresholdLabel(tc.v); got != tc.want {
			t.Errorf("stressThresholdLabel(%v) = %q, want %q", tc.v, got, tc.want)
		}
	}
}

func TestFatigueThresholdLabel(t *testing.T) {
	cases := []struct{ v float64; want string }{
		{0.9, "≥0.8 (Burnout imminent)"},
		{0.7, "≥0.6 (Exhausted)"},
		{0.4, "≥0.3 (Tired)"},
		{0.1, "<0.3 (Rested)"},
	}
	for _, tc := range cases {
		if got := fatigueThresholdLabel(tc.v); got != tc.want {
			t.Errorf("fatigueThresholdLabel(%v) = %q, want %q", tc.v, got, tc.want)
		}
	}
}

func TestFatigueRecommendation(t *testing.T) {
	cases := []struct{ v float64; wantContains string }{
		{0.9, "Burnout imminent"},
		{0.7, "exhausted"},
		{0.4, "Monitor"},
		{0.1, "well-rested"},
	}
	for _, tc := range cases {
		got := fatigueRecommendation(tc.v)
		if !strings.Contains(strings.ToLower(got), strings.ToLower(tc.wantContains)) {
			t.Errorf("fatigueRecommendation(%v) = %q, want contains %q", tc.v, got, tc.wantContains)
		}
	}
}

func TestMoodThresholdLabel(t *testing.T) {
	cases := []struct{ v float64; want string }{
		{0.9, ">0.75 (Happy)"},
		{0.6, ">0.5 (Content)"},
		{0.4, ">0.25 (Neutral)"},
		{0.1, "≤0.25 (Sad/Depressed)"},
	}
	for _, tc := range cases {
		if got := moodThresholdLabel(tc.v); got != tc.want {
			t.Errorf("moodThresholdLabel(%v) = %q, want %q", tc.v, got, tc.want)
		}
	}
}

func TestPressureThresholdLabel(t *testing.T) {
	cases := []struct{ v float64; want string }{
		{0.8, ">0.7 (Storm approaching)"},
		{0.6, ">0.55 (Rising)"},
		{0.3, "≤0.55 (Stable/Improving)"},
	}
	for _, tc := range cases {
		if got := pressureThresholdLabel(tc.v); got != tc.want {
			t.Errorf("pressureThresholdLabel(%v) = %q, want %q", tc.v, got, tc.want)
		}
	}
}

func TestPressureRecommendation(t *testing.T) {
	cases := []struct{ v float64; wantContains string }{
		{0.8, "Storm"},
		{0.6, "rising"},
		{0.3, "stable"},
	}
	for _, tc := range cases {
		got := pressureRecommendation(tc.v)
		if !strings.Contains(strings.ToLower(got), strings.ToLower(tc.wantContains)) {
			t.Errorf("pressureRecommendation(%v) = %q, want contains %q", tc.v, got, tc.wantContains)
		}
	}
}

func TestHumidityThresholdLabel(t *testing.T) {
	cases := []struct{ v float64; want string }{
		{0.6, "≥0.5 (Storm)"},
		{0.4, "≥0.3 (Very humid)"},
		{0.2, "≥0.1 (Humid)"},
		{0.05, "<0.1 (Dry)"},
	}
	for _, tc := range cases {
		if got := humidityThresholdLabel(tc.v); got != tc.want {
			t.Errorf("humidityThresholdLabel(%v) = %q, want %q", tc.v, got, tc.want)
		}
	}
}

func TestContagionThresholdLabel(t *testing.T) {
	cases := []struct{ v float64; want string }{
		{0.9, "≥0.8 (Pandemic)"},
		{0.7, "≥0.6 (Epidemic)"},
		{0.4, "≥0.3 (Moderate)"},
		{0.1, "<0.3 (Low)"},
	}
	for _, tc := range cases {
		if got := contagionThresholdLabel(tc.v); got != tc.want {
			t.Errorf("contagionThresholdLabel(%v) = %q, want %q", tc.v, got, tc.want)
		}
	}
}

func TestHealthScoreThresholdLabel(t *testing.T) {
	cases := []struct{ v float64; want string }{
		{90, ">80 (Excellent)"},
		{70, ">60 (Good)"},
		{50, ">40 (Fair)"},
		{30, ">20 (Poor)"},
		{10, "≤20 (Critical)"},
	}
	for _, tc := range cases {
		if got := healthScoreThresholdLabel(tc.v); got != tc.want {
			t.Errorf("healthScoreThresholdLabel(%v) = %q, want %q", tc.v, got, tc.want)
		}
	}
}

// --- normalizeDDStatus (compat.go) ---

func TestNormalizeDDStatus(t *testing.T) {
	cases := []struct{ in, want string }{
		{"error", "error"},
		{"ERR", "error"},
		{"CRITICAL", "error"},
		{"emerg", "error"},
		{"alert", "error"},
		{"warn", "warn"},
		{"WARNING", "warn"},
		{"debug", "debug"},
		{"TRACE", "debug"},
		{"info", "info"},
		{"", "info"},
		{"unknown", "info"},
	}
	for _, tc := range cases {
		if got := normalizeDDStatus(tc.in); got != tc.want {
			t.Errorf("normalizeDDStatus(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
