package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestPublicWorkspaceReturnsConfiguredShopWithoutSession(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT name, currency FROM shops WHERE id = \$1`).
		WithArgs("shop-id").
		WillReturnRows(sqlmock.NewRows([]string{"name", "currency"}).AddRow("Public Shop", "UGX"))

	server := New(db, Config{PublicShopID: "shop-id"})
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
	if workspace.ShopName != "Public Shop" || workspace.Currency != "UGX" {
		t.Fatalf("workspace = %+v, want configured shop metadata", workspace)
	}
	if strings.Contains(response.Body.String(), "email") || strings.Contains(response.Body.String(), "shopId") {
		t.Fatalf("public workspace leaked account or tenant identifiers: %s", response.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("workspace query expectations: %v", err)
	}
}

func TestPublicWorkspaceRequiresConfiguredShop(t *testing.T) {
	server := New(nil, Config{})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/public/workspace", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), "public_workspace_unavailable") {
		t.Fatalf("status = %d, body = %s; want unconfigured public workspace response", response.Code, response.Body.String())
	}
}

func TestPublicDashboardCanBeRequestedWithoutSession(t *testing.T) {
	server := New(nil, Config{PublicShopID: "shop-id"})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/public/dashboard?period=Yearly", nil)
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "invalid_period") {
		t.Fatalf("status = %d, body = %s; want dashboard validation response without session", response.Code, response.Body.String())
	}
}

func TestPublicProductsOnlyReturnDashboardStockFields(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create SQL mock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(`SELECT id, name, unit, quantity, min_stock FROM products WHERE shop_id = \$1 AND active = TRUE ORDER BY name ASC LIMIT 500`).
		WithArgs("shop-id").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "unit", "quantity", "min_stock"}).
			AddRow(int64(7), "Coffee", "pcs", 12.0, 3.0))

	server := New(db, Config{PublicShopID: "shop-id"})
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
	if !strings.Contains(response.Body.String(), `"name":"Coffee"`) {
		t.Fatalf("public products response omitted dashboard product: %s", response.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("public products query expectations: %v", err)
	}
}
