package httpapi

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"math"
	"net/http"
	"net/mail"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type product struct {
	ID           int64   `json:"id"`
	SKU          string  `json:"sku"`
	Name         string  `json:"name"`
	Category     string  `json:"category"`
	Barcode      string  `json:"barcode"`
	Unit         string  `json:"unit"`
	SellingPrice float64 `json:"sellingPrice"`
	CostPrice    float64 `json:"costPrice"`
	Quantity     float64 `json:"quantity"`
	MinStock     float64 `json:"minStock"`
	Image        string  `json:"image"`
}

type productInput struct {
	SKU          string  `json:"sku"`
	Name         string  `json:"name"`
	Category     string  `json:"category"`
	Barcode      string  `json:"barcode"`
	Unit         string  `json:"unit"`
	SellingPrice float64 `json:"sellingPrice"`
	CostPrice    float64 `json:"costPrice"`
	Quantity     float64 `json:"quantity"`
	MinStock     float64 `json:"minStock"`
	Image        string  `json:"image"`
}

type categoryInput struct {
	Name    string `json:"name"`
	OldName string `json:"oldName"`
}

func (s *Server) listCategories(w http.ResponseWriter, r *http.Request) {
	claims := claimsFrom(r)
	rows, err := s.db.QueryContext(r.Context(), `
		SELECT name FROM categories WHERE shop_id = $1
		UNION
		SELECT DISTINCT category FROM products WHERE shop_id = $1 AND active = TRUE AND category <> ''
		ORDER BY name`,
		claims.ShopID,
	)
	if err != nil {
		log.Printf("list categories for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "Categories could not be loaded.")
		return
	}
	defer rows.Close()
	names := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			log.Printf("read category for shop %s: %v", claims.ShopID, err)
			writeError(w, http.StatusInternalServerError, "request_failed", "Categories could not be loaded.")
			return
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		log.Printf("iterate categories for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "Categories could not be loaded.")
		return
	}
	writeJSON(w, http.StatusOK, map[string][]string{"categories": names})
}

func (s *Server) createCategory(w http.ResponseWriter, r *http.Request) {
	if !requireOwnerOrManager(w, r) {
		return
	}
	var input categoryInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	if !validLength(input.Name, 1, 80) {
		writeError(w, http.StatusBadRequest, "invalid_category", "Category names must be between 1 and 80 characters.")
		return
	}
	claims := claimsFrom(r)
	var exists bool
	if err := s.db.QueryRowContext(r.Context(), `
		SELECT EXISTS (
			SELECT 1 FROM categories WHERE shop_id = $1 AND lower(name) = lower($2)
			UNION ALL
			SELECT 1 FROM products WHERE shop_id = $1 AND active = TRUE AND lower(category) = lower($2)
		)`,
		claims.ShopID, input.Name,
	).Scan(&exists); err != nil {
		log.Printf("check category for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "The category could not be added.")
		return
	}
	if exists {
		writeError(w, http.StatusConflict, "duplicate_category", "That category already exists in this shop.")
		return
	}
	_, err := s.db.ExecContext(r.Context(), `
		INSERT INTO categories (shop_id, name) VALUES ($1, $2)`,
		claims.ShopID, input.Name,
	)
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "duplicate_category", "That category already exists in this shop.")
			return
		}
		log.Printf("create category for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "The category could not be added.")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"name": input.Name})
}

func (s *Server) renameCategory(w http.ResponseWriter, r *http.Request) {
	if !requireOwnerOrManager(w, r) {
		return
	}
	var input categoryInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.OldName = strings.TrimSpace(input.OldName)
	input.Name = strings.TrimSpace(input.Name)
	if !validLength(input.OldName, 1, 80) || !validLength(input.Name, 1, 80) {
		writeError(w, http.StatusBadRequest, "invalid_category", "Category names must be between 1 and 80 characters.")
		return
	}
	if strings.EqualFold(input.OldName, input.Name) {
		writeJSON(w, http.StatusOK, map[string]string{"name": input.Name})
		return
	}
	claims := claimsFrom(r)
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		log.Printf("begin category rename for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "The category could not be renamed.")
		return
	}
	defer tx.Rollback()

	var sourceExists bool
	if err := tx.QueryRowContext(r.Context(), `
		SELECT EXISTS (
			SELECT 1 FROM categories WHERE shop_id = $1 AND lower(name) = lower($2)
			UNION ALL
			SELECT 1 FROM products WHERE shop_id = $1 AND active = TRUE AND lower(category) = lower($2)
		)`,
		claims.ShopID, input.OldName,
	).Scan(&sourceExists); err != nil {
		log.Printf("check category rename source for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "The category could not be renamed.")
		return
	}
	if !sourceExists {
		writeError(w, http.StatusNotFound, "category_not_found", "That category no longer exists.")
		return
	}
	var exists bool
	if err := tx.QueryRowContext(r.Context(), `
		SELECT EXISTS (
			SELECT 1 FROM categories WHERE shop_id = $1 AND lower(name) = lower($2)
			UNION ALL
			SELECT 1 FROM products WHERE shop_id = $1 AND active = TRUE AND lower(category) = lower($2)
		)`,
		claims.ShopID, input.Name,
	).Scan(&exists); err != nil {
		log.Printf("check category rename target for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "The category could not be renamed.")
		return
	}
	if exists {
		writeError(w, http.StatusConflict, "duplicate_category", "Another category with that name already exists.")
		return
	}
	productResult, err := tx.ExecContext(r.Context(), `
		UPDATE products SET category = $1, updated_at = now()
		WHERE shop_id = $2 AND active = TRUE AND lower(category) = lower($3)`,
		input.Name, claims.ShopID, input.OldName,
	)
	if err != nil {
		log.Printf("rename products category for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "The category could not be renamed.")
		return
	}
	result, err := tx.ExecContext(r.Context(), `
		UPDATE categories SET name = $1 WHERE shop_id = $2 AND lower(name) = lower($3)`,
		input.Name, claims.ShopID, input.OldName,
	)
	if err != nil {
		log.Printf("rename category for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "The category could not be renamed.")
		return
	}
	categoryRows, err := result.RowsAffected()
	if err != nil {
		log.Printf("read category rename result for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "The category could not be renamed.")
		return
	}
	productRows, err := productResult.RowsAffected()
	if err != nil {
		log.Printf("read product category rename result for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "The category could not be renamed.")
		return
	}
	if categoryRows == 0 && productRows == 0 {
		writeError(w, http.StatusNotFound, "category_not_found", "That category no longer exists.")
		return
	}
	if err := tx.Commit(); err != nil {
		log.Printf("commit category rename for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "The category could not be renamed.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"name": input.Name})
}

func (s *Server) deleteCategory(w http.ResponseWriter, r *http.Request) {
	if !requireOwnerOrManager(w, r) {
		return
	}
	var input categoryInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	if !validLength(input.Name, 1, 80) {
		writeError(w, http.StatusBadRequest, "invalid_category", "Category names must be between 1 and 80 characters.")
		return
	}
	claims := claimsFrom(r)
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		log.Printf("begin category deletion for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "The category could not be deleted.")
		return
	}
	defer tx.Rollback()
	var exists bool
	if err := tx.QueryRowContext(r.Context(), `
		SELECT EXISTS (
			SELECT 1 FROM categories WHERE shop_id = $1 AND lower(name) = lower($2)
			UNION ALL
			SELECT 1 FROM products WHERE shop_id = $1 AND active = TRUE AND lower(category) = lower($2)
		)`,
		claims.ShopID, input.Name,
	).Scan(&exists); err != nil {
		log.Printf("check category deletion for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "The category could not be deleted.")
		return
	}
	if !exists {
		writeError(w, http.StatusNotFound, "category_not_found", "That category no longer exists.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `
		UPDATE products SET category = '', updated_at = now()
		WHERE shop_id = $1 AND active = TRUE AND lower(category) = lower($2)`,
		claims.ShopID, input.Name,
	); err != nil {
		log.Printf("remove category from products for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "The category could not be deleted.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `
		DELETE FROM categories WHERE shop_id = $1 AND lower(name) = lower($2)`,
		claims.ShopID, input.Name,
	); err != nil {
		log.Printf("delete category for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "The category could not be deleted.")
		return
	}
	if err := tx.Commit(); err != nil {
		log.Printf("commit category deletion for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "The category could not be deleted.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	claims := claimsFrom(r)
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "Daily"
	}
	if period != "Daily" && period != "Weekly" && period != "Monthly" {
		writeError(w, http.StatusBadRequest, "invalid_period", "Choose a daily, weekly, or monthly dashboard period.")
		return
	}
	location := time.FixedZone("Africa/Kampala", 3*60*60)
	now := time.Now().In(location)
	start, end := dashboardPeriodBounds(period, now)
	previousStart, previousEnd := dashboardPreviousPeriodBounds(period, start)
	reportDate := strings.TrimSpace(r.URL.Query().Get("reportDate"))
	if reportDate != "" {
		day, parseErr := time.ParseInLocation("2006-01-02", reportDate, location)
		if parseErr != nil || day.Format("2006-01-02") != reportDate {
			writeError(w, http.StatusBadRequest, "invalid_date", "Choose a valid calendar date.")
			return
		}
		nextDay := day.AddDate(0, 0, 1)
		start, end = day, nextDay
		previousStart, previousEnd = day.AddDate(0, 0, -1), day
	}
	topDate := strings.TrimSpace(r.URL.Query().Get("date"))
	var topStart, topEnd *time.Time
	if reportDate != "" {
		topStart, topEnd = &start, &end
	}
	if topDate != "" {
		day, parseErr := time.ParseInLocation("2006-01-02", topDate, location)
		if parseErr != nil || day.Format("2006-01-02") != topDate {
			writeError(w, http.StatusBadRequest, "invalid_date", "Choose a valid calendar date.")
			return
		}
		nextDay := day.AddDate(0, 0, 1)
		topStart, topEnd = &day, &nextDay
	}
	var result struct {
		ProductCount     int64                     `json:"productCount"`
		LowStockCount    int64                     `json:"lowStockCount"`
		TodaySales       int64                     `json:"todaySales"`
		TodayRevenue     float64                   `json:"todayRevenue"`
		Currency         string                    `json:"currency"`
		InventoryValue   float64                   `json:"inventoryValue"`
		Revenue          float64                   `json:"revenue"`
		Profit           float64                   `json:"profit"`
		PreviousRevenue  float64                   `json:"previousRevenue"`
		PreviousProfit   float64                   `json:"previousProfit"`
		OrderCount       int64                     `json:"orderCount"`
		PreviousOrders   int64                     `json:"previousOrders"`
		UnitsSold        float64                   `json:"unitsSold"`
		Chart            []dashboardChartPoint     `json:"chart"`
		TopProducts      []dashboardTopProduct     `json:"topProducts"`
		ProfitByCategory []dashboardProfitCategory `json:"profitByCategory"`
	}
	err := s.db.QueryRowContext(r.Context(), `
		SELECT
			(SELECT count(*) FROM products WHERE shop_id = $1 AND active = TRUE),
			(SELECT count(*) FROM products WHERE shop_id = $1 AND active = TRUE AND quantity <= min_stock),
			(SELECT COALESCE(sum(cost_price * quantity), 0) FROM products WHERE shop_id = $1 AND active = TRUE),
			(SELECT count(*) FROM sales WHERE shop_id = $1
				AND completed_at >= (date_trunc('day', now() AT TIME ZONE 'Africa/Kampala') AT TIME ZONE 'Africa/Kampala')),
			COALESCE((SELECT sum(total) FROM sales WHERE shop_id = $1
				AND completed_at >= (date_trunc('day', now() AT TIME ZONE 'Africa/Kampala') AT TIME ZONE 'Africa/Kampala')), 0),
			(SELECT currency FROM shops WHERE id = $1)`,
		claims.ShopID,
	).Scan(&result.ProductCount, &result.LowStockCount, &result.InventoryValue, &result.TodaySales, &result.TodayRevenue, &result.Currency)
	if err != nil {
		log.Printf("load dashboard for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "The dashboard could not be loaded.")
		return
	}

	rows, err := s.db.QueryContext(r.Context(), `
		SELECT s.completed_at, s.total, COALESCE(sum(si.quantity), 0),
			COALESCE(sum(si.quantity * si.unit_cost), 0)
		FROM sales s
		LEFT JOIN sale_items si ON si.sale_id = s.id
		WHERE s.shop_id = $1 AND s.completed_at >= $2 AND s.completed_at < $3
		GROUP BY s.id
		ORDER BY s.completed_at`,
		claims.ShopID, start, end,
	)
	if err != nil {
		log.Printf("load dashboard sales for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "The dashboard could not be loaded.")
		return
	}
	chartPeriod := period
	if reportDate != "" {
		chartPeriod = "Daily"
	}
	result.Chart = dashboardChart(chartPeriod, now)
	for rows.Next() {
		var completedAt time.Time
		var revenue, units, cost float64
		if err := rows.Scan(&completedAt, &revenue, &units, &cost); err != nil {
			rows.Close()
			log.Printf("read dashboard sale for shop %s: %v", claims.ShopID, err)
			writeError(w, http.StatusInternalServerError, "request_failed", "The dashboard could not be loaded.")
			return
		}
		result.OrderCount++
		result.Revenue += revenue
		result.Profit += revenue - cost
		result.UnitsSold += units
		index := dashboardChartIndex(chartPeriod, completedAt.In(location))
		result.Chart[index].Sales += revenue
		result.Chart[index].Profit += revenue - cost
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		log.Printf("iterate dashboard sales for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "The dashboard could not be loaded.")
		return
	}
	if err := rows.Close(); err != nil {
		log.Printf("close dashboard sales for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "The dashboard could not be loaded.")
		return
	}

	rows, err = s.db.QueryContext(r.Context(), `
		SELECT COALESCE(NULLIF(p.category, ''), 'Uncategorized'),
			COALESCE(sum(si.line_total - si.quantity * si.unit_cost -
				CASE WHEN s.subtotal > 0 THEN s.discount * si.line_total / s.subtotal ELSE 0 END), 0)
		FROM sales s
		JOIN sale_items si ON si.sale_id = s.id
		JOIN products p ON p.id = si.product_id AND p.shop_id = s.shop_id
		WHERE s.shop_id = $1 AND s.completed_at >= $2 AND s.completed_at < $3
		GROUP BY COALESCE(NULLIF(p.category, ''), 'Uncategorized')
		ORDER BY sum(si.line_total - si.quantity * si.unit_cost -
			CASE WHEN s.subtotal > 0 THEN s.discount * si.line_total / s.subtotal ELSE 0 END) DESC
		LIMIT 5`,
		claims.ShopID, start, end,
	)
	if err != nil {
		log.Printf("load dashboard profit categories for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "The dashboard could not be loaded.")
		return
	}
	result.ProfitByCategory = make([]dashboardProfitCategory, 0, 5)
	for rows.Next() {
		var category dashboardProfitCategory
		if err := rows.Scan(&category.Name, &category.Value); err != nil {
			rows.Close()
			log.Printf("read dashboard profit category for shop %s: %v", claims.ShopID, err)
			writeError(w, http.StatusInternalServerError, "request_failed", "The dashboard could not be loaded.")
			return
		}
		result.ProfitByCategory = append(result.ProfitByCategory, category)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		log.Printf("iterate dashboard profit categories for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "The dashboard could not be loaded.")
		return
	}
	if err := rows.Close(); err != nil {
		log.Printf("close dashboard profit categories for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "The dashboard could not be loaded.")
		return
	}

	err = s.db.QueryRowContext(r.Context(), `
		SELECT count(*), COALESCE(sum(s.total), 0),
			COALESCE(sum(s.total - COALESCE((
				SELECT sum(si.quantity * si.unit_cost) FROM sale_items si WHERE si.sale_id = s.id
			), 0)), 0)
		FROM sales s
		WHERE s.shop_id = $1 AND s.completed_at >= $2 AND s.completed_at < $3`,
		claims.ShopID, previousStart, previousEnd,
	).Scan(&result.PreviousOrders, &result.PreviousRevenue, &result.PreviousProfit)
	if err != nil {
		log.Printf("load previous dashboard period for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "The dashboard could not be loaded.")
		return
	}

	result.TopProducts, err = loadTopProducts(r.Context(), s.db, claims.ShopID, topStart, topEnd)
	if err != nil {
		log.Printf("load dashboard top products for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "The dashboard could not be loaded.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) businessIntelligence(w http.ResponseWriter, r *http.Request) {
	if !requireOwnerOrManager(w, r) {
		return
	}
	claims := claimsFrom(r)
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "Daily"
	}
	if period != "Daily" && period != "Weekly" && period != "Monthly" {
		writeError(w, http.StatusBadRequest, "invalid_period", "Choose a daily, weekly, or monthly business intelligence period.")
		return
	}
	location := time.FixedZone("Africa/Kampala", 3*60*60)
	now := time.Now().In(location)
	start, end := dashboardPeriodBounds(period, now)
	if date := strings.TrimSpace(r.URL.Query().Get("date")); date != "" {
		start, err := time.ParseInLocation("2006-01-02", date, location)
		if err != nil || start.Format("2006-01-02") != date {
			writeError(w, http.StatusBadRequest, "invalid_date", "Choose a valid calendar date.")
			return
		}
		end = start.AddDate(0, 0, 1)
	}

	type peakHour struct {
		Hour  int `json:"hour"`
		Count int `json:"count"`
	}
	type profitableProduct struct {
		Name   string  `json:"name"`
		Profit float64 `json:"profit"`
	}
	type basketPair struct {
		Pair      []string `json:"pair"`
		Frequency int      `json:"frequency"`
	}
	result := struct {
		PeakSalesHours         []int               `json:"peakSalesHours"`
		MostProfitableProducts []profitableProduct `json:"mostProfitableProducts"`
		MarketBasket           []basketPair        `json:"marketBasket"`
	}{
		PeakSalesHours:         make([]int, 12),
		MostProfitableProducts: make([]profitableProduct, 0, 7),
		MarketBasket:           make([]basketPair, 0, 9),
	}

	rows, err := s.db.QueryContext(r.Context(), `
		SELECT EXTRACT(HOUR FROM completed_at AT TIME ZONE 'Africa/Kampala')::int, count(*)
		FROM sales
		WHERE shop_id = $1 AND completed_at >= $2 AND completed_at < $3
		GROUP BY 1`,
		claims.ShopID, start, end,
	)
	if err != nil {
		log.Printf("load business intelligence peak hours for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "Business Intelligence data could not be loaded.")
		return
	}
	for rows.Next() {
		var hour peakHour
		if err := rows.Scan(&hour.Hour, &hour.Count); err != nil {
			rows.Close()
			log.Printf("read business intelligence peak hour for shop %s: %v", claims.ShopID, err)
			writeError(w, http.StatusInternalServerError, "request_failed", "Business Intelligence data could not be loaded.")
			return
		}
		result.PeakSalesHours[(hour.Hour-6+24)%24/2] = hour.Count
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		log.Printf("iterate business intelligence peak hours for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "Business Intelligence data could not be loaded.")
		return
	}
	if err := rows.Close(); err != nil {
		log.Printf("close business intelligence peak hours for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "Business Intelligence data could not be loaded.")
		return
	}

	rows, err = s.db.QueryContext(r.Context(), `
		SELECT si.product_name, sum(si.line_total - si.quantity * si.unit_cost -
			CASE WHEN s.subtotal > 0 THEN s.discount * si.line_total / s.subtotal ELSE 0 END) AS profit
		FROM sales s
		JOIN sale_items si ON si.sale_id = s.id
		WHERE s.shop_id = $1 AND s.completed_at >= $2 AND s.completed_at < $3
		GROUP BY si.product_id, si.product_name
		ORDER BY profit DESC, si.product_name
		LIMIT 7`,
		claims.ShopID, start, end,
	)
	if err != nil {
		log.Printf("load business intelligence profitable products for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "Business Intelligence data could not be loaded.")
		return
	}
	for rows.Next() {
		var product profitableProduct
		if err := rows.Scan(&product.Name, &product.Profit); err != nil {
			rows.Close()
			log.Printf("read business intelligence profitable product for shop %s: %v", claims.ShopID, err)
			writeError(w, http.StatusInternalServerError, "request_failed", "Business Intelligence data could not be loaded.")
			return
		}
		result.MostProfitableProducts = append(result.MostProfitableProducts, product)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		log.Printf("iterate business intelligence profitable products for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "Business Intelligence data could not be loaded.")
		return
	}
	if err := rows.Close(); err != nil {
		log.Printf("close business intelligence profitable products for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "Business Intelligence data could not be loaded.")
		return
	}

	rows, err = s.db.QueryContext(r.Context(), `
		WITH sale_products AS (
			SELECT DISTINCT si.sale_id, si.product_id, si.product_name
			FROM sale_items si
			JOIN sales s ON s.id = si.sale_id
			WHERE s.shop_id = $1 AND s.completed_at >= $2 AND s.completed_at < $3
		)
		SELECT p1.product_name, p2.product_name, count(*) AS frequency
		FROM sale_products p1
		JOIN sale_products p2 ON p1.sale_id = p2.sale_id AND p1.product_id < p2.product_id
		GROUP BY p1.product_id, p1.product_name, p2.product_id, p2.product_name
		ORDER BY frequency DESC, p1.product_name, p2.product_name
		LIMIT 9`,
		claims.ShopID, start, end,
	)
	if err != nil {
		log.Printf("load business intelligence market basket for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "Business Intelligence data could not be loaded.")
		return
	}
	for rows.Next() {
		var first, second string
		var frequency int
		if err := rows.Scan(&first, &second, &frequency); err != nil {
			rows.Close()
			log.Printf("read business intelligence market basket for shop %s: %v", claims.ShopID, err)
			writeError(w, http.StatusInternalServerError, "request_failed", "Business Intelligence data could not be loaded.")
			return
		}
		result.MarketBasket = append(result.MarketBasket, basketPair{
			Pair: []string{first, second}, Frequency: frequency,
		})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		log.Printf("iterate business intelligence market basket for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "Business Intelligence data could not be loaded.")
		return
	}
	if err := rows.Close(); err != nil {
		log.Printf("close business intelligence market basket for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "Business Intelligence data could not be loaded.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) auditLogs(w http.ResponseWriter, r *http.Request) {
	if !requireOwnerOrManager(w, r) {
		return
	}
	search := strings.TrimSpace(r.URL.Query().Get("q"))
	if len([]rune(search)) > 100 {
		writeError(w, http.StatusBadRequest, "invalid_search", "Search text must be 100 characters or fewer.")
		return
	}
	claims := claimsFrom(r)
	rows, err := s.db.QueryContext(r.Context(), `
		WITH activity AS (
			SELECT s.id::text AS id, s.completed_at AS happened_at, s.cashier_name AS username,
				'Sale completed'::text AS action,
				'Sale ' || left(s.id::text, 8) || ' for ' || s.total::text || ' (' ||
					replace(s.payment_method, '_', ' ') || ', ' ||
					(SELECT count(*) FROM sale_items si WHERE si.sale_id = s.id)::text || ' items)' AS details,
				'INFO'::text AS level
			FROM sales s
			WHERE s.shop_id = $1
			UNION ALL
			SELECT 'stock-' || sm.id::text, sm.created_at, COALESCE(u.full_name, 'Unknown user'),
				CASE WHEN sm.reason = 'Opening stock' THEN 'Opening stock recorded' ELSE 'Stock adjusted' END,
				p.name || ': ' || CASE WHEN sm.quantity_delta > 0 THEN '+' ELSE '' END ||
					sm.quantity_delta::text || ' ' || p.unit || ' (' || sm.reason || ')',
				'AUDIT'::text
			FROM stock_movements sm
			JOIN products p ON p.id = sm.product_id AND p.shop_id = sm.shop_id
			LEFT JOIN users u ON u.id = sm.user_id AND u.shop_id = sm.shop_id
			WHERE sm.shop_id = $1 AND sm.sale_id IS NULL
		)
		SELECT id, happened_at, username, action, details, level
		FROM activity
		WHERE $2 = '' OR username ILIKE '%' || $2 || '%' OR action ILIKE '%' || $2 || '%'
			OR details ILIKE '%' || $2 || '%' OR level ILIKE '%' || $2 || '%'
		ORDER BY happened_at DESC, id DESC
		LIMIT 500`,
		claims.ShopID, search,
	)
	if err != nil {
		log.Printf("load audit activity for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "Audit activity could not be loaded.")
		return
	}
	defer rows.Close()
	type auditLog struct {
		ID        string    `json:"id"`
		Timestamp time.Time `json:"timestamp"`
		User      string    `json:"user"`
		Action    string    `json:"action"`
		Details   string    `json:"details"`
		Level     string    `json:"level"`
	}
	entries := make([]auditLog, 0)
	for rows.Next() {
		var entry auditLog
		if err := rows.Scan(&entry.ID, &entry.Timestamp, &entry.User, &entry.Action, &entry.Details, &entry.Level); err != nil {
			log.Printf("read audit activity for shop %s: %v", claims.ShopID, err)
			writeError(w, http.StatusInternalServerError, "request_failed", "Audit activity could not be loaded.")
			return
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		log.Printf("iterate audit activity for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "Audit activity could not be loaded.")
		return
	}
	writeJSON(w, http.StatusOK, map[string][]auditLog{"logs": entries})
}

type shopSettings struct {
	ShopName         string `json:"shopName"`
	Currency         string `json:"currency"`
	Address          string `json:"address"`
	Phone            string `json:"phone"`
	ReceiptHeader    string `json:"receiptHeader"`
	ReceiptFooter    string `json:"receiptFooter"`
	ReceiptShowPhone bool   `json:"receiptShowPhone"`
}

func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) {
	if !requireOwnerOrManager(w, r) {
		return
	}
	claims := claimsFrom(r)
	var settings shopSettings
	err := s.db.QueryRowContext(r.Context(), `
		SELECT name, currency, address, phone, receipt_header, receipt_footer, receipt_show_phone
		FROM shops WHERE id = $1`,
		claims.ShopID,
	).Scan(&settings.ShopName, &settings.Currency, &settings.Address, &settings.Phone,
		&settings.ReceiptHeader, &settings.ReceiptFooter, &settings.ReceiptShowPhone)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "shop_not_found", "The shop settings could not be found.")
		return
	}
	if err != nil {
		log.Printf("load settings for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "Shop settings could not be loaded.")
		return
	}
	settings.Currency = strings.TrimSpace(settings.Currency)
	writeJSON(w, http.StatusOK, map[string]shopSettings{"settings": settings})
}

func (s *Server) updateSettings(w http.ResponseWriter, r *http.Request) {
	if !requireOwnerOrManager(w, r) {
		return
	}
	var settings shopSettings
	if !decodeJSON(w, r, &settings) {
		return
	}
	settings.ShopName = strings.TrimSpace(settings.ShopName)
	settings.Currency = strings.ToUpper(strings.TrimSpace(settings.Currency))
	settings.Address = strings.TrimSpace(settings.Address)
	settings.Phone = strings.TrimSpace(settings.Phone)
	settings.ReceiptHeader = strings.TrimSpace(settings.ReceiptHeader)
	settings.ReceiptFooter = strings.TrimSpace(settings.ReceiptFooter)
	if !validLength(settings.ShopName, 1, 120) ||
		!isValidCurrency(settings.Currency) ||
		!validLength(settings.Address, 0, 200) ||
		!validLength(settings.Phone, 0, 32) ||
		!validLength(settings.ReceiptHeader, 0, 300) ||
		!validLength(settings.ReceiptFooter, 0, 300) {
		writeError(w, http.StatusBadRequest, "invalid_settings", "Check the shop name, three-letter currency code, address, phone, and receipt text lengths.")
		return
	}
	claims := claimsFrom(r)
	err := s.db.QueryRowContext(r.Context(), `
		UPDATE shops SET name = $1, currency = $2, address = $3, phone = $4,
			receipt_header = $5, receipt_footer = $6, receipt_show_phone = $7
		WHERE id = $8
		RETURNING name, currency, address, phone, receipt_header, receipt_footer, receipt_show_phone`,
		settings.ShopName, settings.Currency, settings.Address, settings.Phone,
		settings.ReceiptHeader, settings.ReceiptFooter, settings.ReceiptShowPhone, claims.ShopID,
	).Scan(&settings.ShopName, &settings.Currency, &settings.Address, &settings.Phone,
		&settings.ReceiptHeader, &settings.ReceiptFooter, &settings.ReceiptShowPhone)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "shop_not_found", "The shop settings could not be found.")
		return
	}
	if err != nil {
		log.Printf("update settings for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "Shop settings could not be saved.")
		return
	}
	settings.Currency = strings.TrimSpace(settings.Currency)
	writeJSON(w, http.StatusOK, map[string]shopSettings{"settings": settings})
}

var allowedStaffPermissions = map[string]bool{
	"page_pos":           true,
	"page_inventory":     true,
	"page_bi":            true,
	"page_sales_history": true,
	"page_audit":         true,
	"page_settings":      true,
}

func (s *Server) getSettingsPreferences(w http.ResponseWriter, r *http.Request) {
	if !requireOwnerOrManager(w, r) {
		return
	}
	claims := claimsFrom(r)
	var rawPrefs []byte
	var rawUserPerms []byte
	err := s.db.QueryRowContext(r.Context(), `
		SELECT COALESCE(sh.preferences, '{}'::jsonb), COALESCE(u.permissions, '[]'::jsonb)
		FROM shops sh
		JOIN users u ON u.shop_id = sh.id AND u.id = $2
		WHERE sh.id = $1`,
		claims.ShopID, claims.Subject,
	).Scan(&rawPrefs, &rawUserPerms)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "shop_not_found", "The shop preferences could not be found.")
		return
	}
	if err != nil {
		log.Printf("load settings preferences for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "Shop preferences could not be loaded.")
		return
	}
	prefs := make(map[string]any)
	_ = json.Unmarshal(rawPrefs, &prefs)
	userPerms := make([]string, 0)
	_ = json.Unmarshal(rawUserPerms, &userPerms)

	staffPerms := make(map[string][]string)
	if claims.Role == "owner" || claims.Role == "manager" {
		rows, qErr := s.db.QueryContext(r.Context(), `
			SELECT id::text, COALESCE(permissions, '[]'::jsonb)
			FROM users WHERE shop_id = $1`,
			claims.ShopID,
		)
		if qErr == nil {
			defer rows.Close()
			for rows.Next() {
				var memberID string
				var memberRaw []byte
				if scanErr := rows.Scan(&memberID, &memberRaw); scanErr == nil {
					var p []string
					if json.Unmarshal(memberRaw, &p) == nil && len(p) > 0 {
						staffPerms[memberID] = p
					}
				}
			}
			if rowsErr := rows.Err(); rowsErr != nil {
				log.Printf("load staff permissions for shop %s: %v", claims.ShopID, rowsErr)
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"preferences":      prefs,
		"permissions":      userPerms,
		"staffPermissions": staffPerms,
	})
}

func (s *Server) updateSettingsPreferences(w http.ResponseWriter, r *http.Request) {
	if !requireOwnerOrManager(w, r) {
		return
	}
	var input struct {
		Preferences map[string]any `json:"preferences"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.Preferences == nil {
		input.Preferences = make(map[string]any)
	}
	encoded, err := json.Marshal(input.Preferences)
	if err != nil || len(encoded) > 900_000 {
		writeError(w, http.StatusBadRequest, "invalid_preferences", "Preferences payload is invalid or too large.")
		return
	}
	claims := claimsFrom(r)
	var savedRaw []byte
	err = s.db.QueryRowContext(r.Context(), `
		UPDATE shops SET preferences = ($1::jsonb - '__businessProfileSetupComplete') ||
			CASE WHEN preferences->'__businessProfileSetupComplete' = 'true'::jsonb
				OR $1::jsonb->'__businessProfileSetupComplete' = 'true'::jsonb
				THEN jsonb_build_object('__businessProfileSetupComplete', TRUE)
				WHEN preferences ? '__businessProfileSetupComplete'
				THEN jsonb_build_object('__businessProfileSetupComplete', preferences->'__businessProfileSetupComplete')
				ELSE '{}'::jsonb
			END
		WHERE id = $2
		RETURNING preferences`,
		string(encoded), claims.ShopID,
	).Scan(&savedRaw)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "shop_not_found", "The shop preferences could not be found.")
		return
	}
	if err != nil {
		log.Printf("update settings preferences for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "Shop preferences could not be saved.")
		return
	}
	saved := make(map[string]any)
	_ = json.Unmarshal(savedRaw, &saved)
	writeJSON(w, http.StatusOK, map[string]any{"preferences": saved})
}

func (s *Server) updateTeamMemberPermissions(w http.ResponseWriter, r *http.Request) {
	if !requireShopOwner(w, r) {
		return
	}
	id := strings.ToLower(r.PathValue("id"))
	compactID := strings.ReplaceAll(id, "-", "")
	_, idErr := hex.DecodeString(compactID)
	if len(id) != 36 || id[8] != '-' || id[13] != '-' || id[18] != '-' || id[23] != '-' || idErr != nil {
		writeError(w, http.StatusBadRequest, "invalid_user_id", "Enter a valid team account.")
		return
	}
	var input struct {
		FullName    string   `json:"fullName"`
		Permissions []string `json:"permissions"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	input.FullName = strings.TrimSpace(input.FullName)
	if input.FullName != "" && !validLength(input.FullName, 1, 100) {
		writeError(w, http.StatusBadRequest, "invalid_profile", "Enter a staff name between 1 and 100 characters.")
		return
	}
	cleanPerms := make([]string, 0, len(input.Permissions))
	seen := make(map[string]bool)
	for _, perm := range input.Permissions {
		perm = strings.TrimSpace(perm)
		if !allowedStaffPermissions[perm] {
			writeError(w, http.StatusBadRequest, "invalid_permission", "One or more selected permissions are not valid.")
			return
		}
		if !seen[perm] {
			seen[perm] = true
			cleanPerms = append(cleanPerms, perm)
		}
	}
	encodedPerms, err := json.Marshal(cleanPerms)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_permission", "Permissions could not be encoded.")
		return
	}
	claims := claimsFrom(r)
	var updatedID string
	err = s.db.QueryRowContext(r.Context(), `
		UPDATE users
		SET permissions = $1::jsonb,
			full_name = CASE WHEN $2 <> '' THEN $2 ELSE full_name END
		WHERE id = $3 AND shop_id = $4
		RETURNING id::text`,
		string(encodedPerms), input.FullName, id, claims.ShopID,
	).Scan(&updatedID)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "team_member_not_found", "That team account could not be found.")
		return
	}
	if err != nil {
		log.Printf("update team member permissions for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "The team account permissions could not be updated.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":          updatedID,
		"permissions": cleanPerms,
	})
}

func isValidCurrency(currency string) bool {
	if len(currency) != 3 {
		return false
	}
	for _, character := range currency {
		if character < 'A' || character > 'Z' {
			return false
		}
	}
	return true
}

type teamMember struct {
	ID        string    `json:"id"`
	FullName  string    `json:"fullName"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"createdAt"`
}

func requireShopOwner(w http.ResponseWriter, r *http.Request) bool {
	if claimsFrom(r).Role != "owner" {
		writeError(w, http.StatusForbidden, "permission_denied", "Only the shop owner can manage team accounts.")
		return false
	}
	return true
}

func (s *Server) listTeamMembers(w http.ResponseWriter, r *http.Request) {
	if !requireShopOwner(w, r) {
		return
	}
	claims := claimsFrom(r)
	rows, err := s.db.QueryContext(r.Context(), `
		SELECT id::text, full_name, email, role, active, created_at
		FROM users
		WHERE shop_id = $1
		ORDER BY created_at ASC, id ASC`,
		claims.ShopID,
	)
	if err != nil {
		log.Printf("list team members for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "Team accounts could not be loaded.")
		return
	}
	defer rows.Close()
	members := make([]teamMember, 0)
	for rows.Next() {
		var member teamMember
		if err := rows.Scan(&member.ID, &member.FullName, &member.Email, &member.Role, &member.Active, &member.CreatedAt); err != nil {
			log.Printf("read team member for shop %s: %v", claims.ShopID, err)
			writeError(w, http.StatusInternalServerError, "request_failed", "Team accounts could not be loaded.")
			return
		}
		members = append(members, member)
	}
	if err := rows.Err(); err != nil {
		log.Printf("iterate team members for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "Team accounts could not be loaded.")
		return
	}
	writeJSON(w, http.StatusOK, map[string][]teamMember{"members": members})
}

func (s *Server) createTeamMember(w http.ResponseWriter, r *http.Request) {
	if !requireShopOwner(w, r) {
		return
	}
	var input struct {
		FullName string `json:"fullName"`
		Email    string `json:"email"`
		Role     string `json:"role"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	input.FullName = strings.TrimSpace(input.FullName)
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.Role = strings.TrimSpace(input.Role)
	if !validLength(input.FullName, 1, 100) {
		writeError(w, http.StatusBadRequest, "invalid_profile", "Enter a staff name between 1 and 100 characters.")
		return
	}
	if len(input.Email) > 254 {
		writeError(w, http.StatusBadRequest, "invalid_email", "Enter a valid staff email address.")
		return
	}
	parsedEmail, err := mail.ParseAddress(input.Email)
	if err != nil || parsedEmail.Address != input.Email {
		writeError(w, http.StatusBadRequest, "invalid_email", "Enter a valid staff email address.")
		return
	}
	if input.Role != "manager" && input.Role != "cashier" {
		writeError(w, http.StatusBadRequest, "invalid_role", "New team members must be a manager or cashier.")
		return
	}
	if len(input.Password) < 8 || len(input.Password) > 72 {
		writeError(w, http.StatusBadRequest, "invalid_password", "Use a password between 8 and 72 bytes.")
		return
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("hash team member password for shop %s: %v", claimsFrom(r).ShopID, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "The team account could not be created.")
		return
	}
	claims := claimsFrom(r)
	var member teamMember
	err = s.db.QueryRowContext(r.Context(), `
		INSERT INTO users (shop_id, full_name, email, password_hash, role)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id::text, full_name, email, role, active, created_at`,
		claims.ShopID, input.FullName, input.Email, passwordHash, input.Role,
	).Scan(&member.ID, &member.FullName, &member.Email, &member.Role, &member.Active, &member.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "email_in_use", "An account with that email already exists.")
			return
		}
		log.Printf("create team member for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "The team account could not be created.")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]teamMember{"member": member})
}

func (s *Server) updateTeamMemberStatus(w http.ResponseWriter, r *http.Request) {
	if !requireShopOwner(w, r) {
		return
	}
	id := strings.ToLower(r.PathValue("id"))
	compactID := strings.ReplaceAll(id, "-", "")
	_, idErr := hex.DecodeString(compactID)
	if len(id) != 36 || id[8] != '-' || id[13] != '-' || id[18] != '-' || id[23] != '-' || idErr != nil {
		writeError(w, http.StatusBadRequest, "invalid_user_id", "Enter a valid team account.")
		return
	}
	var input struct {
		Active bool `json:"active"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	claims := claimsFrom(r)
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		log.Printf("begin team status update for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "The team account could not be updated.")
		return
	}
	defer tx.Rollback()

	var lockedShop string
	if err := tx.QueryRowContext(r.Context(), `SELECT id::text FROM shops WHERE id = $1 FOR UPDATE`, claims.ShopID).Scan(&lockedShop); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "shop_not_found", "The shop could not be found.")
			return
		}
		log.Printf("lock shop %s for team status update: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "The team account could not be updated.")
		return
	}
	var member teamMember
	err = tx.QueryRowContext(r.Context(), `
		SELECT id::text, full_name, email, role, active, created_at
		FROM users WHERE id = $1 AND shop_id = $2 FOR UPDATE`,
		id, claims.ShopID,
	).Scan(&member.ID, &member.FullName, &member.Email, &member.Role, &member.Active, &member.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "team_member_not_found", "That team account could not be found.")
		return
	}
	if err != nil {
		log.Printf("load team member %s for status update: %v", id, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "The team account could not be updated.")
		return
	}
	if id == claims.Subject && !input.Active {
		writeError(w, http.StatusConflict, "cannot_deactivate_self", "You cannot deactivate your own account.")
		return
	}
	if member.Role == "owner" && member.Active && !input.Active {
		var activeOwners int
		if err := tx.QueryRowContext(r.Context(), `
			SELECT count(*) FROM users WHERE shop_id = $1 AND role = 'owner' AND active = TRUE`,
			claims.ShopID,
		).Scan(&activeOwners); err != nil {
			log.Printf("count active owners for shop %s: %v", claims.ShopID, err)
			writeError(w, http.StatusInternalServerError, "request_failed", "The team account could not be updated.")
			return
		}
		if activeOwners <= 1 {
			writeError(w, http.StatusConflict, "last_active_owner", "The last active owner account cannot be deactivated.")
			return
		}
	}
	if member.Active != input.Active {
		if _, err := tx.ExecContext(r.Context(),
			`UPDATE users SET active = $1 WHERE id = $2 AND shop_id = $3`,
			input.Active, id, claims.ShopID,
		); err != nil {
			log.Printf("update team member status for shop %s: %v", claims.ShopID, err)
			writeError(w, http.StatusInternalServerError, "request_failed", "The team account could not be updated.")
			return
		}
		member.Active = input.Active
	}
	if err := tx.Commit(); err != nil {
		log.Printf("commit team member status update for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "The team account could not be updated.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]teamMember{"member": member})
}

type dashboardChartPoint struct {
	Label  string  `json:"label"`
	Sales  float64 `json:"sales"`
	Profit float64 `json:"profit"`
}

type dashboardTopProduct struct {
	Name         string  `json:"name"`
	QuantitySold float64 `json:"quantitySold"`
	Revenue      float64 `json:"revenue"`
}

type dashboardProfitCategory struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

func dashboardPeriodBounds(period string, now time.Time) (time.Time, time.Time) {
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	switch period {
	case "Weekly":
		daysSinceMonday := (int(day.Weekday()) + 6) % 7
		start := day.AddDate(0, 0, -daysSinceMonday)
		return start, start.AddDate(0, 0, 7)
	case "Monthly":
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		return start, start.AddDate(0, 1, 0)
	default:
		return day, day.AddDate(0, 0, 1)
	}
}

func dashboardPreviousPeriodBounds(period string, currentStart time.Time) (time.Time, time.Time) {
	switch period {
	case "Weekly":
		start := currentStart.AddDate(0, 0, -7)
		return start, currentStart
	case "Monthly":
		start := currentStart.AddDate(0, -1, 0)
		return start, currentStart
	default:
		start := currentStart.AddDate(0, 0, -1)
		return start, currentStart
	}
}

func dashboardChart(period string, _ time.Time) []dashboardChartPoint {
	switch period {
	case "Weekly":
		labels := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
		points := make([]dashboardChartPoint, len(labels))
		for i, label := range labels {
			points[i].Label = label
		}
		return points
	case "Monthly":
		points := make([]dashboardChartPoint, 4)
		for i := range points {
			points[i].Label = "Week " + strconv.Itoa(i+1)
		}
		return points
	default:
		points := make([]dashboardChartPoint, 24)
		for i := range points {
			hour := (i + 6) % 24
			displayHour := hour % 12
			if displayHour == 0 {
				displayHour = 12
			}
			meridiem := "am"
			if hour >= 12 {
				meridiem = "pm"
			}
			points[i].Label = strconv.Itoa(displayHour) + meridiem
		}
		return points
	}
}

func dashboardChartIndex(period string, completedAt time.Time) int {
	switch period {
	case "Weekly":
		return (int(completedAt.Weekday()) + 6) % 7
	case "Monthly":
		index := (completedAt.Day() - 1) / 7
		if index > 3 {
			return 3
		}
		return index
	default:
		return (completedAt.Hour() - 6 + 24) % 24
	}
}

func loadTopProducts(ctx context.Context, db *sql.DB, shopID string, start, end *time.Time) ([]dashboardTopProduct, error) {
	query := `
		SELECT si.product_name, sum(si.quantity),
			sum(si.line_total - CASE WHEN s.subtotal > 0 THEN s.discount * si.line_total / s.subtotal ELSE 0 END)
		FROM sales s
		JOIN sale_items si ON si.sale_id = s.id
		WHERE s.shop_id = $1
		GROUP BY si.product_id, si.product_name
		ORDER BY sum(si.quantity) DESC, sum(si.line_total) DESC
		LIMIT 5`
	args := []any{shopID}
	if start != nil && end != nil {
		query = `
			SELECT si.product_name, sum(si.quantity),
				sum(si.line_total - CASE WHEN s.subtotal > 0 THEN s.discount * si.line_total / s.subtotal ELSE 0 END)
			FROM sales s
			JOIN sale_items si ON si.sale_id = s.id
			WHERE s.shop_id = $1 AND s.completed_at >= $2 AND s.completed_at < $3
			GROUP BY si.product_id, si.product_name
			ORDER BY sum(si.quantity) DESC, sum(si.line_total) DESC
			LIMIT 5`
		args = append(args, *start, *end)
	}
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	products := make([]dashboardTopProduct, 0, 5)
	for rows.Next() {
		var item dashboardTopProduct
		if err := rows.Scan(&item.Name, &item.QuantitySold, &item.Revenue); err != nil {
			return nil, err
		}
		products = append(products, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return products, nil
}

func (s *Server) listProducts(w http.ResponseWriter, r *http.Request) {
	claims := claimsFrom(r)
	search := strings.TrimSpace(r.URL.Query().Get("q"))
	if len([]rune(search)) > 100 {
		writeError(w, http.StatusBadRequest, "invalid_search", "Search text must be 100 characters or fewer.")
		return
	}
	rows, err := s.db.QueryContext(r.Context(), `
		SELECT id, sku, name, category, barcode, unit, selling_price, cost_price, quantity, min_stock, image
		FROM products
		WHERE shop_id = $1 AND active = TRUE
			AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR sku ILIKE '%' || $2 || '%' OR barcode ILIKE '%' || $2 || '%' OR category ILIKE '%' || $2 || '%')
		ORDER BY name ASC
		LIMIT 500`,
		claims.ShopID, search,
	)
	if err != nil {
		log.Printf("list products for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "Products could not be loaded.")
		return
	}
	defer rows.Close()
	products := make([]product, 0)
	for rows.Next() {
		var item product
		if err := rows.Scan(&item.ID, &item.SKU, &item.Name, &item.Category, &item.Barcode, &item.Unit, &item.SellingPrice, &item.CostPrice, &item.Quantity, &item.MinStock, &item.Image); err != nil {
			log.Printf("read product for shop %s: %v", claims.ShopID, err)
			writeError(w, http.StatusInternalServerError, "request_failed", "Products could not be loaded.")
			return
		}
		products = append(products, item)
	}
	if err := rows.Err(); err != nil {
		log.Printf("iterate products for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "Products could not be loaded.")
		return
	}
	writeJSON(w, http.StatusOK, map[string][]product{"products": products})
}

func (s *Server) createProduct(w http.ResponseWriter, r *http.Request) {
	if !requireOwnerOrManager(w, r) {
		return
	}
	var input productInput
	if !decodeJSON(w, r, &input) || !normalizeProductInput(w, &input, true) {
		return
	}
	claims := claimsFrom(r)
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		log.Printf("begin product creation for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "The product could not be saved.")
		return
	}
	defer tx.Rollback()

	var created product
	err = tx.QueryRowContext(r.Context(), `
		INSERT INTO products (shop_id, sku, name, category, barcode, unit, selling_price, cost_price, quantity, min_stock, image)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, sku, name, category, barcode, unit, selling_price, cost_price, quantity, min_stock, image`,
		claims.ShopID, input.SKU, input.Name, input.Category, input.Barcode, input.Unit,
		input.SellingPrice, input.CostPrice, input.Quantity, input.MinStock, input.Image,
	).Scan(&created.ID, &created.SKU, &created.Name, &created.Category, &created.Barcode, &created.Unit,
		&created.SellingPrice, &created.CostPrice, &created.Quantity, &created.MinStock, &created.Image)
	if err != nil {
		writeProductDatabaseError(w, err, "The product could not be saved.")
		return
	}
	if input.Quantity > 0 {
		if _, err := tx.ExecContext(r.Context(), `
			INSERT INTO stock_movements (shop_id, product_id, user_id, quantity_delta, reason)
			VALUES ($1, $2, $3, $4, 'Opening stock')`,
			claims.ShopID, created.ID, claims.Subject, input.Quantity,
		); err != nil {
			log.Printf("record opening stock for product %d: %v", created.ID, err)
			writeError(w, http.StatusInternalServerError, "request_failed", "The product could not be saved.")
			return
		}
	}
	if err := tx.Commit(); err != nil {
		log.Printf("commit product creation for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "The product could not be saved.")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]product{"product": created})
}

func (s *Server) updateProduct(w http.ResponseWriter, r *http.Request) {
	if !requireOwnerOrManager(w, r) {
		return
	}
	id, ok := productID(w, r)
	if !ok {
		return
	}
	var input productInput
	if !decodeJSON(w, r, &input) || !normalizeProductInput(w, &input, false) {
		return
	}
	claims := claimsFrom(r)
	var updated product
	err := s.db.QueryRowContext(r.Context(), `
		UPDATE products SET sku = $1, name = $2, category = $3, barcode = $4, unit = $5,
			selling_price = $6, cost_price = $7, min_stock = $8, image = $9, updated_at = now()
		WHERE id = $10 AND shop_id = $11 AND active = TRUE
		RETURNING id, sku, name, category, barcode, unit, selling_price, cost_price, quantity, min_stock, image`,
		input.SKU, input.Name, input.Category, input.Barcode, input.Unit,
		input.SellingPrice, input.CostPrice, input.MinStock, input.Image, id, claims.ShopID,
	).Scan(&updated.ID, &updated.SKU, &updated.Name, &updated.Category, &updated.Barcode, &updated.Unit,
		&updated.SellingPrice, &updated.CostPrice, &updated.Quantity, &updated.MinStock, &updated.Image)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "product_not_found", "That product is no longer available.")
		return
	}
	if err != nil {
		writeProductDatabaseError(w, err, "The product could not be updated.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]product{"product": updated})
}

func (s *Server) adjustStock(w http.ResponseWriter, r *http.Request) {
	if !requireOwnerOrManager(w, r) {
		return
	}
	id, ok := productID(w, r)
	if !ok {
		return
	}
	var input struct {
		Delta  float64 `json:"delta"`
		Reason string  `json:"reason"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Reason = strings.TrimSpace(input.Reason)
	if !isValidQuantityDelta(input.Delta) || input.Delta == 0 || !validLength(input.Reason, 1, 200) {
		writeError(w, http.StatusBadRequest, "invalid_adjustment", "Enter a non-zero stock change with a reason of 1 to 200 characters.")
		return
	}
	claims := claimsFrom(r)
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		log.Printf("begin stock adjustment for product %d: %v", id, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "Stock could not be adjusted.")
		return
	}
	defer tx.Rollback()

	var item product
	err = tx.QueryRowContext(r.Context(), `
		UPDATE products SET quantity = quantity + $1::numeric, updated_at = now()
		WHERE id = $2 AND shop_id = $3 AND active = TRUE AND quantity + $1::numeric >= 0
		RETURNING id, sku, name, category, barcode, unit, selling_price, cost_price, quantity, min_stock, image`,
		input.Delta, id, claims.ShopID,
	).Scan(&item.ID, &item.SKU, &item.Name, &item.Category, &item.Barcode, &item.Unit,
		&item.SellingPrice, &item.CostPrice, &item.Quantity, &item.MinStock, &item.Image)
	if errors.Is(err, sql.ErrNoRows) {
		var exists bool
		if checkErr := tx.QueryRowContext(r.Context(), `
			SELECT EXISTS (SELECT 1 FROM products WHERE id = $1 AND shop_id = $2 AND active = TRUE)`,
			id, claims.ShopID,
		).Scan(&exists); checkErr != nil {
			log.Printf("check product %d for stock adjustment: %v", id, checkErr)
			writeError(w, http.StatusInternalServerError, "request_failed", "Stock could not be adjusted.")
			return
		}
		if !exists {
			writeError(w, http.StatusNotFound, "product_not_found", "That product is no longer available.")
		} else {
			writeError(w, http.StatusConflict, "insufficient_stock", "Stock cannot be adjusted below zero.")
		}
		return
	}
	if err != nil {
		log.Printf("adjust stock for product %d: %v", id, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "Stock could not be adjusted.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `
		INSERT INTO stock_movements (shop_id, product_id, user_id, quantity_delta, reason)
		VALUES ($1, $2, $3, $4, $5)`,
		claims.ShopID, id, claims.Subject, input.Delta, input.Reason,
	); err != nil {
		log.Printf("record stock adjustment for product %d: %v", id, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "Stock could not be adjusted.")
		return
	}
	if err := tx.Commit(); err != nil {
		log.Printf("commit stock adjustment for product %d: %v", id, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "Stock could not be adjusted.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]product{"product": item})
}

func (s *Server) archiveProduct(w http.ResponseWriter, r *http.Request) {
	if !requireOwnerOrManager(w, r) {
		return
	}
	id, ok := productID(w, r)
	if !ok {
		return
	}
	claims := claimsFrom(r)
	result, err := s.db.ExecContext(r.Context(), `
		UPDATE products SET active = FALSE, updated_at = now()
		WHERE id = $1 AND shop_id = $2 AND active = TRUE`,
		id, claims.ShopID,
	)
	if err != nil {
		log.Printf("archive product %d for shop %s: %v", id, claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "The product could not be archived.")
		return
	}
	affected, err := result.RowsAffected()
	if err != nil {
		log.Printf("read archive result for product %d: %v", id, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "The product could not be archived.")
		return
	}
	if affected == 0 {
		writeError(w, http.StatusNotFound, "product_not_found", "That product is no longer available.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func normalizeProductInput(w http.ResponseWriter, input *productInput, includeQuantity bool) bool {
	input.SKU = strings.TrimSpace(input.SKU)
	input.Name = strings.TrimSpace(input.Name)
	input.Category = strings.TrimSpace(input.Category)
	input.Barcode = strings.TrimSpace(input.Barcode)
	input.Unit = strings.TrimSpace(input.Unit)
	if input.Unit == "" {
		input.Unit = "piece"
	}
	input.Image = strings.TrimSpace(input.Image)
	if len(input.Image) > 2*1024*1024 {
		writeError(w, http.StatusBadRequest, "invalid_product_image", "Product image must be 2MB or smaller.")
		return false
	}
	if !validLength(input.Name, 1, 160) ||
		!validLength(input.SKU, 0, 80) ||
		!validLength(input.Category, 0, 80) ||
		!validLength(input.Barcode, 0, 120) ||
		!validLength(input.Unit, 1, 32) ||
		!isValidAmount(input.SellingPrice) ||
		input.SellingPrice == 0 ||
		!isValidAmount(input.CostPrice) ||
		!isValidQuantity(input.MinStock) ||
		(includeQuantity && !isValidQuantity(input.Quantity)) {
		writeError(w, http.StatusBadRequest, "invalid_product", "Check the product name, price, unit, and stock values.")
		return false
	}
	return true
}

func isValidAmount(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0 &&
		value <= 999999999999.99 && math.Abs(value*100-math.Round(value*100)) < 0.000001
}

func isValidQuantity(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0 &&
		value <= 99999999999.999 && math.Abs(value*1000-math.Round(value*1000)) < 0.000001
}

func isValidQuantityDelta(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) &&
		math.Abs(value) <= 99999999999.999 &&
		math.Abs(value*1000-math.Round(value*1000)) < 0.000001
}

func productID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		writeError(w, http.StatusBadRequest, "invalid_product_id", "Enter a valid product.")
		return 0, false
	}
	return id, true
}

func requireOwnerOrManager(w http.ResponseWriter, r *http.Request) bool {
	claims := claimsFrom(r)
	if claims.Role != "owner" && claims.Role != "manager" {
		writeError(w, http.StatusForbidden, "permission_denied", "Your account cannot change shop settings, products, or stock.")
		return false
	}
	return true
}

func claimsFrom(r *http.Request) *sessionClaims {
	val := r.Context().Value(sessionClaimsKey{})
	if val == nil {
		return nil
	}
	claims, ok := val.(*sessionClaims)
	if !ok {
		return nil
	}
	return claims
}

func writeProductDatabaseError(w http.ResponseWriter, err error, message string) {
	if isUniqueViolation(err) {
		writeError(w, http.StatusConflict, "duplicate_product_code", "That SKU or barcode is already used by another product in this shop.")
		return
	}
	log.Printf("save product: %v", err)
	writeError(w, http.StatusInternalServerError, "request_failed", message)
}

type checkoutRequest struct {
	CustomerName  string         `json:"customerName"`
	PaymentMethod string         `json:"paymentMethod"`
	Discount      float64        `json:"discount"`
	Items         []checkoutItem `json:"items"`
}

type checkoutItem struct {
	ProductID int64   `json:"productId"`
	Quantity  float64 `json:"quantity"`
}

type saleItem struct {
	ProductID   int64   `json:"productId"`
	ProductName string  `json:"productName"`
	Quantity    float64 `json:"quantity"`
	UnitPrice   float64 `json:"unitPrice"`
	UnitCost    float64 `json:"unitCost"`
	LineTotal   float64 `json:"lineTotal"`
}

type sale struct {
	ID            string     `json:"id"`
	CashierName   string     `json:"cashierName"`
	CustomerName  string     `json:"customerName"`
	Subtotal      float64    `json:"subtotal"`
	Discount      float64    `json:"discount"`
	Total         float64    `json:"total"`
	PaymentMethod string     `json:"paymentMethod"`
	CompletedAt   time.Time  `json:"completedAt"`
	Items         []saleItem `json:"items"`
}

func (s *Server) checkout(w http.ResponseWriter, r *http.Request) {
	var input checkoutRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	input.CustomerName = strings.TrimSpace(input.CustomerName)
	if !validLength(input.CustomerName, 0, 120) {
		writeError(w, http.StatusBadRequest, "invalid_customer", "Customer name must be 120 characters or fewer.")
		return
	}
	if input.PaymentMethod != "cash" && input.PaymentMethod != "mobile_money" && input.PaymentMethod != "card" {
		writeError(w, http.StatusBadRequest, "invalid_payment_method", "Choose cash, mobile money, or card.")
		return
	}
	if !isValidAmount(input.Discount) {
		writeError(w, http.StatusBadRequest, "invalid_discount", "Discount must be a non-negative amount with at most two decimal places.")
		return
	}
	if input.Discount > 0 && claimsFrom(r).Role != "owner" && claimsFrom(r).Role != "manager" {
		writeError(w, http.StatusForbidden, "discount_not_allowed", "Only an owner or manager can apply a discount.")
		return
	}
	if len(input.Items) == 0 || len(input.Items) > 100 {
		writeError(w, http.StatusBadRequest, "invalid_cart", "A sale must contain between 1 and 100 products.")
		return
	}

	quantities := make(map[int64]float64, len(input.Items))
	for _, line := range input.Items {
		if line.ProductID < 1 || !isValidQuantity(line.Quantity) || line.Quantity == 0 {
			writeError(w, http.StatusBadRequest, "invalid_cart", "Check the products and quantities in the sale.")
			return
		}
		quantities[line.ProductID] += line.Quantity
		if !isValidQuantity(quantities[line.ProductID]) {
			writeError(w, http.StatusBadRequest, "invalid_cart", "A product quantity is too large.")
			return
		}
	}
	productIDs := make([]int64, 0, len(quantities))
	for id := range quantities {
		productIDs = append(productIDs, id)
	}
	sort.Slice(productIDs, func(i, j int) bool { return productIDs[i] < productIDs[j] })

	claims := claimsFrom(r)
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		log.Printf("begin checkout for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "checkout_failed", "The sale could not be completed.")
		return
	}
	defer tx.Rollback()

	var cashierName string
	if err := tx.QueryRowContext(r.Context(), `
		SELECT full_name FROM users WHERE id = $1 AND shop_id = $2 AND active = TRUE`,
		claims.Subject, claims.ShopID,
	).Scan(&cashierName); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			clearSession(w, s.config.CookieSecure)
			writeError(w, http.StatusUnauthorized, "session_expired", "Please sign in again.")
			return
		}
		log.Printf("load cashier for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "checkout_failed", "The sale could not be completed.")
		return
	}

	items := make([]saleItem, 0, len(productIDs))
	var subtotal float64
	for _, id := range productIDs {
		quantity := quantities[id]
		var line saleItem
		line.ProductID = id
		line.Quantity = quantity
		err := tx.QueryRowContext(r.Context(), `
			UPDATE products SET quantity = quantity - $1::numeric, updated_at = now()
			WHERE id = $2 AND shop_id = $3 AND active = TRUE AND quantity >= $1::numeric
			RETURNING name, selling_price, cost_price`,
			quantity, id, claims.ShopID,
		).Scan(&line.ProductName, &line.UnitPrice, &line.UnitCost)
		if errors.Is(err, sql.ErrNoRows) {
			var exists bool
			if checkErr := tx.QueryRowContext(r.Context(), `
				SELECT EXISTS (SELECT 1 FROM products WHERE id = $1 AND shop_id = $2 AND active = TRUE)`,
				id, claims.ShopID,
			).Scan(&exists); checkErr != nil {
				log.Printf("check product %d during checkout: %v", id, checkErr)
				writeError(w, http.StatusInternalServerError, "checkout_failed", "The sale could not be completed.")
				return
			}
			if !exists {
				writeError(w, http.StatusConflict, "product_unavailable", "A product in this cart is no longer available.")
			} else {
				writeError(w, http.StatusConflict, "insufficient_stock", "There is not enough stock to complete this sale.")
			}
			return
		}
		if err != nil {
			log.Printf("reserve product %d for checkout: %v", id, err)
			writeError(w, http.StatusInternalServerError, "checkout_failed", "The sale could not be completed.")
			return
		}
		line.LineTotal = math.Round(line.UnitPrice*quantity*100) / 100
		if !isValidAmount(line.LineTotal) || line.LineTotal == 0 {
			writeError(w, http.StatusBadRequest, "invalid_sale_total", "Each product must have a sale value of at least 0.01.")
			return
		}
		subtotal += line.LineTotal
		if !isValidAmount(subtotal) {
			writeError(w, http.StatusBadRequest, "invalid_sale_total", "The sale total is too large to process.")
			return
		}
		items = append(items, line)
	}
	subtotal = math.Round(subtotal*100) / 100
	if input.Discount > subtotal {
		writeError(w, http.StatusBadRequest, "invalid_discount", "Discount cannot be greater than the sale subtotal.")
		return
	}
	total := math.Round((subtotal-input.Discount)*100) / 100

	var completed sale
	err = tx.QueryRowContext(r.Context(), `
		INSERT INTO sales (shop_id, cashier_id, cashier_name, customer_name, subtotal, discount, total, payment_method)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id::text, cashier_name, customer_name, subtotal, discount, total, payment_method, completed_at`,
		claims.ShopID, claims.Subject, cashierName, input.CustomerName, subtotal, input.Discount, total, input.PaymentMethod,
	).Scan(&completed.ID, &completed.CashierName, &completed.CustomerName, &completed.Subtotal, &completed.Discount, &completed.Total,
		&completed.PaymentMethod, &completed.CompletedAt)
	if err != nil {
		log.Printf("create sale for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "checkout_failed", "The sale could not be completed.")
		return
	}
	for _, item := range items {
		if _, err := tx.ExecContext(r.Context(), `
			INSERT INTO sale_items (sale_id, product_id, product_name, quantity, unit_price, unit_cost, line_total)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			completed.ID, item.ProductID, item.ProductName, item.Quantity, item.UnitPrice, item.UnitCost, item.LineTotal,
		); err != nil {
			log.Printf("record item on sale %s: %v", completed.ID, err)
			writeError(w, http.StatusInternalServerError, "checkout_failed", "The sale could not be completed.")
			return
		}
		if _, err := tx.ExecContext(r.Context(), `
			INSERT INTO stock_movements (shop_id, product_id, user_id, quantity_delta, reason, sale_id)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			claims.ShopID, item.ProductID, claims.Subject, -item.Quantity, "Sale "+completed.ID, completed.ID,
		); err != nil {
			log.Printf("record stock movement for sale %s: %v", completed.ID, err)
			writeError(w, http.StatusInternalServerError, "checkout_failed", "The sale could not be completed.")
			return
		}
	}
	if err := tx.Commit(); err != nil {
		log.Printf("commit sale %s: %v", completed.ID, err)
		writeError(w, http.StatusInternalServerError, "checkout_failed", "The sale could not be completed.")
		return
	}
	completed.Items = items
	writeJSON(w, http.StatusCreated, map[string]sale{"sale": completed})
}

func (s *Server) listSales(w http.ResponseWriter, r *http.Request) {
	claims := claimsFrom(r)
	rows, err := s.db.QueryContext(r.Context(), `
		WITH shop_sales AS (
			SELECT id, cashier_name, customer_name, subtotal, discount, total, payment_method, completed_at
			FROM sales WHERE shop_id = $1
		)
		SELECT ss.id::text, ss.cashier_name, ss.customer_name, ss.subtotal, ss.discount, ss.total,
			ss.payment_method, ss.completed_at, si.product_id, si.product_name,
			si.quantity, si.unit_price, si.unit_cost, si.line_total
		FROM shop_sales ss
		LEFT JOIN sale_items si ON si.sale_id = ss.id
		ORDER BY ss.completed_at DESC, si.id ASC`,
		claims.ShopID,
	)
	if err != nil {
		log.Printf("list sales for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "Sales could not be loaded.")
		return
	}
	defer rows.Close()

	sales := make([]sale, 0)
	indexByID := make(map[string]int)
	for rows.Next() {
		var item sale
		var productID sql.NullInt64
		var productName sql.NullString
		var quantity, unitPrice, unitCost, lineTotal sql.NullFloat64
		if err := rows.Scan(&item.ID, &item.CashierName, &item.CustomerName, &item.Subtotal, &item.Discount, &item.Total,
			&item.PaymentMethod, &item.CompletedAt, &productID, &productName, &quantity, &unitPrice, &unitCost, &lineTotal); err != nil {
			log.Printf("read sale for shop %s: %v", claims.ShopID, err)
			writeError(w, http.StatusInternalServerError, "request_failed", "Sales could not be loaded.")
			return
		}
		index, exists := indexByID[item.ID]
		if !exists {
			item.Items = make([]saleItem, 0)
			index = len(sales)
			indexByID[item.ID] = index
			sales = append(sales, item)
		}
		if productID.Valid {
			sales[index].Items = append(sales[index].Items, saleItem{
				ProductID: productID.Int64, ProductName: productName.String, Quantity: quantity.Float64,
				UnitPrice: unitPrice.Float64, UnitCost: unitCost.Float64, LineTotal: lineTotal.Float64,
			})
		}
	}
	if err := rows.Err(); err != nil {
		log.Printf("iterate sales for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "request_failed", "Sales could not be loaded.")
		return
	}
	writeJSON(w, http.StatusOK, map[string][]sale{"sales": sales})
}
