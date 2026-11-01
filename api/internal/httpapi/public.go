package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strings"
)

type publicWorkspaceResponse struct {
	ShopName string `json:"shopName"`
	Currency string `json:"currency"`
}

type publicDashboardProduct struct {
	ID       int64   `json:"id"`
	Name     string  `json:"name"`
	Unit     string  `json:"unit"`
	Quantity float64 `json:"quantity"`
	MinStock float64 `json:"minStock"`
}

func (s *Server) publicWorkspace(w http.ResponseWriter, r *http.Request) {
	if !s.requirePublicShop(w) {
		return
	}

	var workspace publicWorkspaceResponse
	err := s.db.QueryRowContext(r.Context(), `
		SELECT name, currency FROM shops WHERE id = $1`,
		s.config.PublicShopID,
	).Scan(&workspace.ShopName, &workspace.Currency)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusServiceUnavailable, "public_workspace_unavailable", "The public dashboard is not configured.")
		return
	}
	if err != nil {
		log.Printf("load public workspace: %v", err)
		writeError(w, http.StatusInternalServerError, "request_failed", "The public dashboard could not be loaded.")
		return
	}
	writeJSON(w, http.StatusOK, workspace)
}

func (s *Server) publicDashboard(w http.ResponseWriter, r *http.Request) {
	if !s.requirePublicShop(w) {
		return
	}
	claims := &sessionClaims{ShopID: s.config.PublicShopID}
	ctx := context.WithValue(r.Context(), sessionClaimsKey{}, claims)
	s.dashboard(w, r.WithContext(ctx))
}

func (s *Server) publicProducts(w http.ResponseWriter, r *http.Request) {
	if !s.requirePublicShop(w) {
		return
	}

	rows, err := s.db.QueryContext(r.Context(), `
		SELECT id, name, unit, quantity, min_stock
		FROM products
		WHERE shop_id = $1 AND active = TRUE
		ORDER BY name ASC
		LIMIT 500`,
		s.config.PublicShopID,
	)
	if err != nil {
		log.Printf("list public dashboard products: %v", err)
		writeError(w, http.StatusInternalServerError, "request_failed", "Public stock alerts could not be loaded.")
		return
	}
	defer rows.Close()

	products := make([]publicDashboardProduct, 0)
	for rows.Next() {
		var item publicDashboardProduct
		if err := rows.Scan(&item.ID, &item.Name, &item.Unit, &item.Quantity, &item.MinStock); err != nil {
			log.Printf("read public dashboard product: %v", err)
			writeError(w, http.StatusInternalServerError, "request_failed", "Public stock alerts could not be loaded.")
			return
		}
		products = append(products, item)
	}
	if err := rows.Err(); err != nil {
		log.Printf("iterate public dashboard products: %v", err)
		writeError(w, http.StatusInternalServerError, "request_failed", "Public stock alerts could not be loaded.")
		return
	}
	writeJSON(w, http.StatusOK, map[string][]publicDashboardProduct{"products": products})
}

func (s *Server) requirePublicShop(w http.ResponseWriter) bool {
	if strings.TrimSpace(s.config.PublicShopID) == "" {
		writeError(w, http.StatusServiceUnavailable, "public_workspace_unavailable", "The public dashboard is not configured.")
		return false
	}
	return true
}
