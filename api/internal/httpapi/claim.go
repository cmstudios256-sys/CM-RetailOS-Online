package httpapi

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var guestUUIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-8][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)

type guestClaimRequest struct {
	SchemaVersion  int                  `json:"schemaVersion"`
	GuestDeviceID  string               `json:"guestDeviceId"`
	IdempotencyKey string               `json:"idempotencyKey"`
	Products       []guestClaimProduct  `json:"products"`
	Categories     []string             `json:"categories"`
	Sales          []guestClaimSale     `json:"sales"`
	StockMovements []guestClaimMovement `json:"stockMovements"`
	Settings       guestClaimSettings   `json:"settings"`
}

type guestClaimProduct struct {
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
	Active       *bool   `json:"active,omitempty"`
}

type guestClaimSale struct {
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

type guestClaimMovement struct {
	ID            string    `json:"id"`
	ProductID     int64     `json:"productId"`
	QuantityDelta float64   `json:"quantityDelta"`
	Reason        string    `json:"reason"`
	SaleID        string    `json:"saleId,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
}

type guestClaimSettings struct {
	ShopName         string         `json:"shopName"`
	Currency         string         `json:"currency"`
	Address          string         `json:"address"`
	Phone            string         `json:"phone"`
	ReceiptHeader    string         `json:"receiptHeader"`
	ReceiptFooter    string         `json:"receiptFooter"`
	ReceiptShowPhone bool           `json:"receiptShowPhone"`
	Preferences      map[string]any `json:"preferences"`
}

type guestClaimCounts struct {
	Products       int `json:"products"`
	Categories     int `json:"categories"`
	Sales          int `json:"sales"`
	StockMovements int `json:"stockMovements"`
}

type guestClaimResponse struct {
	Imported   guestClaimCounts  `json:"imported"`
	ProductIDs map[string]int64  `json:"productIds"`
	SaleIDs    map[string]string `json:"saleIds"`
}

func (s *Server) claimGuestData(w http.ResponseWriter, r *http.Request) {
	if !requireOwnerOrManager(w, r) {
		return
	}
	var input guestClaimRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := validateGuestClaim(input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_guest_claim", err.Error())
		return
	}
	payload, err := json.Marshal(input)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_guest_claim", "Guest data could not be validated.")
		return
	}
	payloadHash := sha256.Sum256(payload)
	claims := claimsFrom(r)
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		log.Printf("begin guest data claim for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "guest_claim_failed", "Guest data could not be saved to the cloud.")
		return
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(r.Context(), `
		INSERT INTO guest_imports (shop_id, guest_device_id, idempotency_key, payload_hash)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (shop_id, guest_device_id, idempotency_key) DO NOTHING`,
		claims.ShopID, input.GuestDeviceID, input.IdempotencyKey, payloadHash[:],
	)
	if err != nil {
		log.Printf("reserve guest data claim for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "guest_claim_failed", "Guest data could not be saved to the cloud.")
		return
	}
	var storedHash, storedResponse []byte
	err = tx.QueryRowContext(r.Context(), `
		SELECT payload_hash, response
		FROM guest_imports
		WHERE shop_id = $1 AND guest_device_id = $2 AND idempotency_key = $3
		FOR UPDATE`,
		claims.ShopID, input.GuestDeviceID, input.IdempotencyKey,
	).Scan(&storedHash, &storedResponse)
	if err != nil {
		log.Printf("load guest data claim for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "guest_claim_failed", "Guest data could not be saved to the cloud.")
		return
	}
	if !bytes.Equal(storedHash, payloadHash[:]) {
		writeError(w, http.StatusConflict, "guest_claim_key_reused", "This save-to-cloud request key was already used for different guest data.")
		return
	}
	if len(storedResponse) > 0 {
		var result guestClaimResponse
		if err := json.Unmarshal(storedResponse, &result); err != nil {
			log.Printf("decode saved guest claim result for shop %s: %v", claims.ShopID, err)
			writeError(w, http.StatusInternalServerError, "guest_claim_failed", "The previous guest data save could not be verified.")
			return
		}
		if err := tx.Commit(); err != nil {
			log.Printf("finish idempotent guest claim for shop %s: %v", claims.ShopID, err)
			writeError(w, http.StatusInternalServerError, "guest_claim_failed", "Guest data could not be saved to the cloud.")
			return
		}
		writeJSON(w, http.StatusOK, result)
		return
	}

	result, err := importGuestData(r, tx, claims, input)
	if err != nil {
		var uniqueErr interface{ SQLState() string }
		if errors.As(err, &uniqueErr) && uniqueErr.SQLState() == "23505" {
			writeError(w, http.StatusConflict, "guest_import_conflict", "Some guest products conflict with data already in this shop. Resolve the duplicate SKU or barcode and retry.")
			return
		}
		log.Printf("import guest data for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "guest_claim_failed", "Guest data could not be saved to the cloud.")
		return
	}
	responseBytes, err := json.Marshal(result)
	if err != nil {
		log.Printf("encode guest claim result for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "guest_claim_failed", "Guest data could not be saved to the cloud.")
		return
	}
	if _, err := tx.ExecContext(r.Context(), `
		UPDATE guest_imports SET response = $1
		WHERE shop_id = $2 AND guest_device_id = $3 AND idempotency_key = $4`,
		responseBytes, claims.ShopID, input.GuestDeviceID, input.IdempotencyKey,
	); err != nil {
		log.Printf("store guest claim result for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "guest_claim_failed", "Guest data could not be saved to the cloud.")
		return
	}
	if err := tx.Commit(); err != nil {
		log.Printf("commit guest data claim for shop %s: %v", claims.ShopID, err)
		writeError(w, http.StatusInternalServerError, "guest_claim_failed", "Guest data could not be saved to the cloud.")
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func validateGuestClaim(input guestClaimRequest) error {
	if input.SchemaVersion != 1 {
		return errors.New("schemaVersion must be 1.")
	}
	if !guestUUIDPattern.MatchString(input.GuestDeviceID) || !guestUUIDPattern.MatchString(input.IdempotencyKey) {
		return errors.New("guestDeviceId and idempotencyKey must be UUIDs.")
	}
	if len(input.Products) > 500 || len(input.Categories) > 200 || len(input.Sales) > 10000 || len(input.StockMovements) > 50000 {
		return errors.New("The guest data bundle exceeds the supported record limits.")
	}
	if !validLength(strings.TrimSpace(input.Settings.ShopName), 1, 120) ||
		!isValidCurrency(strings.ToUpper(strings.TrimSpace(input.Settings.Currency))) ||
		len([]rune(input.Settings.Address)) > 200 || len([]rune(input.Settings.Phone)) > 32 ||
		len([]rune(input.Settings.ReceiptHeader)) > 300 || len([]rune(input.Settings.ReceiptFooter)) > 300 ||
		input.Settings.Preferences == nil {
		return errors.New("Shop settings are invalid.")
	}
	productIDs := make(map[int64]bool, len(input.Products))
	for _, item := range input.Products {
		if item.ID < 1 || productIDs[item.ID] || !validLength(strings.TrimSpace(item.Name), 1, 160) ||
			!isValidAmount(item.SellingPrice) || item.SellingPrice == 0 ||
			!isValidAmount(item.CostPrice) || !isValidQuantity(item.Quantity) ||
			!isValidQuantity(item.MinStock) || len([]rune(item.SKU)) > 80 ||
			len([]rune(item.Category)) > 80 || len([]rune(item.Barcode)) > 100 ||
			len([]rune(item.Unit)) > 40 || len(item.Image) > 500_000 {
			return errors.New("A guest product has invalid or duplicate fields.")
		}
		productIDs[item.ID] = true
	}
	categoryNames := make(map[string]bool, len(input.Categories))
	for _, name := range input.Categories {
		normalized := strings.ToLower(strings.TrimSpace(name))
		if !validLength(strings.TrimSpace(name), 1, 80) || categoryNames[normalized] {
			return errors.New("Guest categories must be unique names between 1 and 80 characters.")
		}
		categoryNames[normalized] = true
	}
	saleIDs := make(map[string]bool, len(input.Sales))
	for _, item := range input.Sales {
		if !validLength(item.ID, 1, 128) || len([]rune(item.CashierName)) > 100 || saleIDs[item.ID] ||
			!validLength(item.CustomerName, 0, 120) || item.CompletedAt.IsZero() ||
			(item.PaymentMethod != "cash" && item.PaymentMethod != "mobile_money" && item.PaymentMethod != "card") ||
			!isValidAmount(item.Subtotal) || !isValidAmount(item.Discount) || !isValidAmount(item.Total) ||
			item.Discount > item.Subtotal || len(item.Items) == 0 || len(item.Items) > 100 {
			return errors.New("A guest sale has invalid or duplicate fields.")
		}
		saleIDs[item.ID] = true
		var subtotal float64
		for _, line := range item.Items {
			if !productIDs[line.ProductID] || !validLength(line.ProductName, 1, 160) ||
				!isValidQuantity(line.Quantity) || line.Quantity <= 0 ||
				!isValidAmount(line.UnitPrice) || line.UnitPrice <= 0 ||
				!isValidAmount(line.UnitCost) || !isValidAmount(line.LineTotal) || line.LineTotal <= 0 {
				return errors.New("A guest sale item has invalid fields or refers to an unknown product.")
			}
			subtotal += line.LineTotal
		}
		if math.Abs(math.Round(subtotal*100)/100-item.Subtotal) > 0.01 ||
			math.Abs(math.Round((item.Subtotal-item.Discount)*100)/100-item.Total) > 0.01 {
			return errors.New("A guest sale total does not match its items.")
		}
	}
	for _, movement := range input.StockMovements {
		if !validLength(movement.ID, 1, 128) || !productIDs[movement.ProductID] ||
			!isValidQuantityDelta(movement.QuantityDelta) || movement.QuantityDelta == 0 ||
			!validLength(movement.Reason, 1, 200) || movement.CreatedAt.IsZero() ||
			movement.SaleID != "" && !saleIDs[movement.SaleID] {
			return errors.New("A guest stock movement has invalid fields or references.")
		}
	}
	return nil
}

func importGuestData(r *http.Request, tx *sql.Tx, claims *sessionClaims, input guestClaimRequest) (guestClaimResponse, error) {
	result := guestClaimResponse{
		ProductIDs: make(map[string]int64, len(input.Products)),
		SaleIDs:    make(map[string]string, len(input.Sales)),
	}
	for _, category := range input.Categories {
		inserted, err := tx.ExecContext(r.Context(), `
			INSERT INTO categories (shop_id, name) VALUES ($1, $2)
			ON CONFLICT DO NOTHING`,
			claims.ShopID, strings.TrimSpace(category),
		)
		if err != nil {
			return result, err
		}
		if rows, err := inserted.RowsAffected(); err == nil && rows > 0 {
			result.Imported.Categories++
		}
	}
	for _, item := range input.Products {
		active := true
		if item.Active != nil {
			active = *item.Active
		}
		var cloudProductID int64
		err := tx.QueryRowContext(r.Context(), `
			INSERT INTO products (
				shop_id, sku, name, category, barcode, unit, selling_price, cost_price,
				quantity, min_stock, image, active
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			RETURNING id`,
			claims.ShopID, strings.TrimSpace(item.SKU), strings.TrimSpace(item.Name), strings.TrimSpace(item.Category),
			strings.TrimSpace(item.Barcode), strings.TrimSpace(item.Unit), item.SellingPrice, item.CostPrice,
			item.Quantity, item.MinStock, item.Image, active,
		).Scan(&cloudProductID)
		if err != nil {
			return result, err
		}
		result.ProductIDs[strconv.FormatInt(item.ID, 10)] = cloudProductID
		result.Imported.Products++
	}
	for _, item := range input.Sales {
		var saleID string
		cashierName := strings.TrimSpace(item.CashierName)
		if cashierName == "" {
			cashierName = "Shop Owner"
		}
		err := tx.QueryRowContext(r.Context(), `
			INSERT INTO sales (
				shop_id, cashier_id, cashier_name, customer_name, subtotal, discount, total,
				payment_method, completed_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			RETURNING id::text`,
			claims.ShopID, claims.Subject, cashierName, strings.TrimSpace(item.CustomerName),
			item.Subtotal, item.Discount, item.Total, item.PaymentMethod, item.CompletedAt,
		).Scan(&saleID)
		if err != nil {
			return result, err
		}
		result.SaleIDs[item.ID] = saleID
		for _, line := range item.Items {
			productID := result.ProductIDs[strconv.FormatInt(line.ProductID, 10)]
			if _, err := tx.ExecContext(r.Context(), `
				INSERT INTO sale_items (sale_id, product_id, product_name, quantity, unit_price, unit_cost, line_total)
				VALUES ($1, $2, $3, $4, $5, $6, $7)`,
				saleID, productID, line.ProductName, line.Quantity, line.UnitPrice, line.UnitCost, line.LineTotal,
			); err != nil {
				return result, err
			}
		}
		result.Imported.Sales++
	}
	for _, movement := range input.StockMovements {
		guestProductKey := strconv.FormatInt(movement.ProductID, 10)
		var saleID any
		if movement.SaleID != "" {
			saleID = result.SaleIDs[movement.SaleID]
		}
		if _, err := tx.ExecContext(r.Context(), `
			INSERT INTO stock_movements (shop_id, product_id, user_id, quantity_delta, reason, sale_id, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			claims.ShopID, result.ProductIDs[guestProductKey], claims.Subject,
			movement.QuantityDelta, strings.TrimSpace(movement.Reason), saleID, movement.CreatedAt,
		); err != nil {
			return result, err
		}
		result.Imported.StockMovements++
	}
	preferences, err := json.Marshal(input.Settings.Preferences)
	if err != nil {
		return result, err
	}
	_, err = tx.ExecContext(r.Context(), `
		UPDATE shops SET
			name = CASE WHEN COALESCE(preferences->>'__businessProfileSetupComplete', 'false') = 'true' THEN name ELSE $1 END,
			currency = $2,
			address = CASE WHEN COALESCE(preferences->>'__businessProfileSetupComplete', 'false') = 'true' THEN address ELSE $3 END,
			phone = CASE WHEN COALESCE(preferences->>'__businessProfileSetupComplete', 'false') = 'true' THEN phone ELSE $4 END,
			receipt_header = $5,
			receipt_footer = $6,
			receipt_show_phone = CASE WHEN COALESCE(preferences->>'__businessProfileSetupComplete', 'false') = 'true' THEN receipt_show_phone ELSE $7 END,
			preferences = (COALESCE(preferences, '{}'::jsonb) || ($8::jsonb - '__businessProfileSetupComplete')) ||
				CASE WHEN preferences ? '__businessProfileSetupComplete'
					THEN jsonb_build_object('__businessProfileSetupComplete', preferences->'__businessProfileSetupComplete')
					ELSE '{}'::jsonb
				END
		WHERE id = $9`,
		strings.TrimSpace(input.Settings.ShopName), strings.ToUpper(strings.TrimSpace(input.Settings.Currency)),
		strings.TrimSpace(input.Settings.Address), strings.TrimSpace(input.Settings.Phone),
		strings.TrimSpace(input.Settings.ReceiptHeader), strings.TrimSpace(input.Settings.ReceiptFooter),
		input.Settings.ReceiptShowPhone, preferences, claims.ShopID,
	)
	if err != nil {
		return result, err
	}
	return result, nil
}
