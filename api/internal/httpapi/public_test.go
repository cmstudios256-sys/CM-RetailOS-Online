package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPublicWorkspaceIsClearlyLabeledDemo(t *testing.T) {
	server := New(nil, Config{})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/public/workspace", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusOK, response.Body.String())
	}
	var workspace publicWorkspaceResponse
	if err := json.Unmarshal(response.Body.Bytes(), &workspace); err != nil {
		t.Fatalf("decode public workspace: %v", err)
	}
	if workspace.ShopName != "Demo Shop" || workspace.Currency != "UGX" || !workspace.Demo {
		t.Fatalf("workspace = %+v, want the seeded demo shop", workspace)
	}
	if strings.Contains(response.Body.String(), "email") || strings.Contains(response.Body.String(), "shopId") {
		t.Fatalf("public workspace leaked account or tenant identifiers: %s", response.Body.String())
	}
}

func TestPublicDashboardIsSeededDemoData(t *testing.T) {
	server := New(nil, Config{})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/public/dashboard", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusOK, response.Body.String())
	}
	for _, sample := range []string{"Demo Coffee Beans", "Sample Notebook", `"todayRevenue":21000`} {
		if !strings.Contains(response.Body.String(), sample) {
			t.Errorf("demo dashboard response omitted %q: %s", sample, response.Body.String())
		}
	}
}

func TestPublicProductsAreStaticDemoDataWithoutPrivateFields(t *testing.T) {
	server := New(nil, Config{})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/public/products", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", response.Code, http.StatusOK, response.Body.String())
	}
	for _, privateField := range []string{"costPrice", "sku", "barcode", "sellingPrice"} {
		if strings.Contains(response.Body.String(), `"`+privateField+`"`) {
			t.Errorf("public products response contains %q: %s", privateField, response.Body.String())
		}
	}
	if !strings.Contains(response.Body.String(), `"name":"Demo Coffee Beans"`) {
		t.Fatalf("public products response omitted seeded demo product: %s", response.Body.String())
	}
}
