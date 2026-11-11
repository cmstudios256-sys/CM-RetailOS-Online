package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type productItem struct {
	sku      string
	name     string
	category string
	unit     string
	selling  int64
	cost     int64
	quantity float64
	minStock float64
	image    string
}

var demoProducts = []productItem{
	// 1-3: Footwear
	{"FTW-SNK-001", "Classic Red Sneakers", "Footwear", "pair", 125000, 85000, 18, 5, "https://images.unsplash.com/photo-1542291026-7eec264c27ff?w=400&auto=format&fit=crop&q=80"},
	{"FTW-CNV-002", "White Canvas Low-Tops", "Footwear", "pair", 85000, 55000, 12, 4, "https://images.unsplash.com/photo-1525966222134-fcfa99b8ae77?w=400&auto=format&fit=crop&q=80"},
	{"FTW-OXF-003", "Leather Oxford Dress Shoes", "Footwear", "pair", 180000, 120000, 8, 3, "https://images.unsplash.com/photo-1614252235316-8c857d38b5f4?w=400&auto=format&fit=crop&q=80"},

	// 4-10: Electronics
	{"ELC-HDP-004", "Studio Wireless Headphones", "Electronics", "pcs", 185000, 120000, 7, 3, "https://images.unsplash.com/photo-1505740420928-5e560c06d30e?w=400&auto=format&fit=crop&q=80"},
	{"ELC-CAM-005", "Instant Film Camera", "Electronics", "pcs", 320000, 230000, 5, 2, "https://images.unsplash.com/photo-1526170375885-4d8ecf77b99f?w=400&auto=format&fit=crop&q=80"},
	{"ELC-SPK-006", "Bluetooth Speaker Mini", "Electronics", "pcs", 75000, 48000, 14, 4, "https://images.unsplash.com/photo-1608043152269-423dbba4e7e1?w=400&auto=format&fit=crop&q=80"},
	{"ELC-EAR-007", "True Wireless Earbuds", "Electronics", "pcs", 95000, 60000, 16, 4, "https://images.unsplash.com/photo-1590658268037-6bf12165a8df?w=400&auto=format&fit=crop&q=80"},
	{"ELC-CBL-008", "USB-C Fast Charging Cable", "Electronics", "pcs", 15000, 7500, 45, 10, "https://images.unsplash.com/photo-1541807084-5c52b6b3adef?w=400&auto=format&fit=crop&q=80"},
	{"ELC-ADP-009", "20W USB-C Power Adapter", "Electronics", "pcs", 35000, 20000, 22, 5, "https://images.unsplash.com/photo-1583863788434-e58a36330cf0?w=400&auto=format&fit=crop&q=80"},
	{"ELC-PWR-010", "20000mAh Power Bank", "Electronics", "pcs", 85000, 55000, 11, 3, "https://images.unsplash.com/photo-1621259182978-fbf93132d53d?w=400&auto=format&fit=crop&q=80"},

	// 11-15: Accessories
	{"ACC-BTL-011", "Everyday Water Bottle", "Accessories", "pcs", 28000, 16000, 24, 8, "https://images.unsplash.com/photo-1602143407151-7111542de6e8?w=400&auto=format&fit=crop&q=80"},
	{"ACC-WCH-012", "Minimalist Wristwatch", "Accessories", "pcs", 95000, 62000, 9, 3, "https://images.unsplash.com/photo-1523275335684-37898b6baf30?w=400&auto=format&fit=crop&q=80"},
	{"ACC-SUN-013", "Polarized Sunglasses", "Accessories", "pcs", 45000, 26000, 15, 5, "https://images.unsplash.com/photo-1572635196237-14b3f281503f?w=400&auto=format&fit=crop&q=80"},
	{"ACC-BPK-014", "Classic Canvas Backpack", "Accessories", "pcs", 75000, 48000, 10, 3, "https://images.unsplash.com/photo-1553062407-98eeb64c6a62?w=400&auto=format&fit=crop&q=80"},
	{"ACC-WLT-015", "Genuine Leather Bi-fold Wallet", "Accessories", "pcs", 55000, 32000, 14, 4, "https://images.unsplash.com/photo-1627123424574-724758594e93?w=400&auto=format&fit=crop&q=80"},

	// 16-30: Groceries
	{"GRO-COF-016", "Medium Roast Coffee Beans 500g", "Groceries", "pack", 42000, 26000, 9, 3, "https://images.unsplash.com/photo-1559056199-641a0ac8b55e?w=400&auto=format&fit=crop&q=80"},
	{"GRO-SUG-017", "Pure Granulated Sugar 1kg", "Groceries", "pack", 4500, 3200, 65, 15, "https://images.unsplash.com/photo-1612198188060-c7c2a3b66eae?w=400&auto=format&fit=crop&q=80"},
	{"GRO-RIC-018", "Premium Long Grain Rice 2kg", "Groceries", "pack", 8500, 6000, 38, 10, "https://images.unsplash.com/photo-1586201375761-83865001e31c?w=400&auto=format&fit=crop&q=80"},
	{"GRO-FLR-019", "Fortified Maize Flour 2kg", "Groceries", "pack", 5200, 3800, 42, 10, "https://images.unsplash.com/photo-1509440159596-0249088772ff?w=400&auto=format&fit=crop&q=80"},
	{"GRO-SLT-020", "Refined Table Salt 1kg", "Groceries", "pack", 2500, 1600, 80, 20, "https://images.unsplash.com/photo-1518110925495-5fe2fda0442c?w=400&auto=format&fit=crop&q=80"},
	{"GRO-OIL-021", "Pure Vegetable Cooking Oil 1L", "Groceries", "bottle", 11500, 8200, 28, 8, "https://images.unsplash.com/photo-1474979266404-7eaacbcd87c5?w=400&auto=format&fit=crop&q=80"},
	{"GRO-BAS-022", "Basmati Rice 5kg", "Groceries", "bag", 28000, 19500, 16, 5, "https://images.unsplash.com/photo-1536304993881-ff6e9eefa2a6?w=400&auto=format&fit=crop&q=80"},
	{"GRO-BNS-023", "Organic Brown Beans 1kg", "Groceries", "pack", 4800, 3200, 35, 10, "https://images.unsplash.com/photo-1551462147-ff29053bfc14?w=400&auto=format&fit=crop&q=80"},
	{"GRO-MLK-024", "Full Cream Milk Powder 400g", "Groceries", "tin", 16500, 11800, 22, 6, "https://images.unsplash.com/photo-1550583724-b2692b85b150?w=400&auto=format&fit=crop&q=80"},
	{"GRO-SOIL-025", "Sunflower Cooking Oil 500ml", "Groceries", "bottle", 6200, 4400, 30, 8, "https://images.unsplash.com/photo-1533473359331-0135ef1b58bf?w=400&auto=format&fit=crop&q=80"},
	{"GRO-TEA-026", "Premium Black Tea 50 Bags", "Groceries", "box", 6500, 4200, 34, 8, "https://images.unsplash.com/photo-1576092768241-dec231879fc3?w=400&auto=format&fit=crop&q=80"},
	{"GRO-PAS-027", "Spaghetti Pasta 500g", "Groceries", "pack", 3800, 2500, 48, 12, "https://images.unsplash.com/photo-1551462147-37885acc36f1?w=400&auto=format&fit=crop&q=80"},
	{"GRO-TOM-028", "Canned Peeled Tomatoes 400g", "Groceries", "can", 4200, 2800, 32, 8, "https://images.unsplash.com/photo-1592924357228-91a4daadcfea?w=400&auto=format&fit=crop&q=80"},
	{"GRO-OAT-029", "Rolled Oats 500g", "Groceries", "pack", 7500, 5000, 20, 5, "https://images.unsplash.com/photo-1586201375761-83865001e31c?w=400&auto=format&fit=crop&q=80"},
	{"GRO-HON-030", "Pure Natural Honey 500g", "Groceries", "jar", 18500, 12500, 15, 4, "https://images.unsplash.com/photo-1587049352846-4a222e784d38?w=400&auto=format&fit=crop&q=80"},

	// 31-37: Bakery & Snacks
	{"SNK-BRD-031", "Whole Wheat Sandwich Bread", "Bakery & Snacks", "loaf", 4500, 2800, 25, 8, "https://images.unsplash.com/photo-1509440159596-0249088772ff?w=400&auto=format&fit=crop&q=80"},
	{"SNK-POT-032", "Salted Potato Crisps 150g", "Bakery & Snacks", "pack", 3500, 2200, 40, 10, "https://images.unsplash.com/photo-1566478989037-eec170784d0b?w=400&auto=format&fit=crop&q=80"},
	{"SNK-NUT-033", "Roasted Salted Peanuts 250g", "Bakery & Snacks", "pack", 4000, 2500, 35, 10, "https://images.unsplash.com/photo-1567894340315-735d7c361db0?w=400&auto=format&fit=crop&q=80"},
	{"SNK-SHT-034", "Butter Shortbread Biscuits 200g", "Bakery & Snacks", "pack", 4200, 2700, 30, 8, "https://images.unsplash.com/photo-1558961363-fa8fdf82db35?w=400&auto=format&fit=crop&q=80"},
	{"SNK-POP-035", "Sweet Buttered Popcorn 100g", "Bakery & Snacks", "pack", 2500, 1500, 45, 12, "https://images.unsplash.com/photo-1578849278619-e73505e9610f?w=400&auto=format&fit=crop&q=80"},
	{"SNK-COF-036", "Premium Ground Coffee 250g", "Bakery & Snacks", "pack", 14500, 9500, 18, 5, "https://images.unsplash.com/photo-1514432324607-a09d9b4aefdd?w=400&auto=format&fit=crop&q=80"},
	{"SNK-CKI-037", "Chocolate Chip Cookies 200g", "Bakery & Snacks", "pack", 5000, 3200, 28, 8, "https://images.unsplash.com/photo-1499636136210-6f4ee915583e?w=400&auto=format&fit=crop&q=80"},

	// 38-44: Beverages
	{"BEV-COL-038", "Cola Soda 500ml", "Beverages", "bottle", 2000, 1300, 50, 15, "https://images.unsplash.com/photo-1622483767028-3f66f32aef97?w=400&auto=format&fit=crop&q=80"},
	{"BEV-LEM-039", "Lemon-Lime Soda 500ml", "Beverages", "bottle", 2000, 1300, 45, 15, "https://images.unsplash.com/photo-1625772299848-391b6a87d7b3?w=400&auto=format&fit=crop&q=80"},
	{"BEV-ENG-040", "Energy Drink Can 400ml", "Beverages", "can", 4500, 3000, 36, 10, "https://images.unsplash.com/photo-1622543925917-763c34d1a86e?w=400&auto=format&fit=crop&q=80"},
	{"BEV-WTR-041", "Natural Mineral Water 500ml", "Beverages", "bottle", 1500, 850, 72, 24, "https://images.unsplash.com/photo-1527661591475-527312dd65f5?w=400&auto=format&fit=crop&q=80"},
	{"BEV-TON-042", "Sparkling Tonic Water 330ml", "Beverages", "can", 3000, 1900, 28, 8, "https://images.unsplash.com/photo-1551024709-8f23befc6f87?w=400&auto=format&fit=crop&q=80"},
	{"BEV-CHOC-043", "Dark Chocolate Bar 100g", "Beverages", "piece", 5500, 3400, 38, 10, "https://images.unsplash.com/photo-1549007994-cb92caebd54b?w=400&auto=format&fit=crop&q=80"},
	{"BEV-JUC-044", "Fresh Tropical Fruit Juice 500ml", "Beverages", "bottle", 4500, 2800, 24, 6, "https://images.unsplash.com/photo-1613478223719-2ab802602423?w=400&auto=format&fit=crop&q=80"},

	// 45-47: Personal Care
	{"PER-SOP-045", "Antibacterial Body Soap 100g", "Personal Care", "piece", 3000, 1800, 55, 15, "https://images.unsplash.com/photo-1600857544200-b2f666a9a2ec?w=400&auto=format&fit=crop&q=80"},
	{"PER-PST-046", "Herbal Fluoride Toothpaste 120g", "Personal Care", "tube", 5500, 3500, 32, 8, "https://images.unsplash.com/photo-1584308666744-24d5c474f2ae?w=400&auto=format&fit=crop&q=80"},
	{"PER-LOT-047", "Nourishing Hair & Body Lotion 400ml", "Personal Care", "bottle", 14500, 9200, 18, 5, "https://images.unsplash.com/photo-1556228720-195a672e8a03?w=400&auto=format&fit=crop&q=80"},

	// 48-50: Household
	{"HOU-DET-048", "Bio Laundry Detergent Powder 1kg", "Household", "pack", 9500, 6200, 26, 6, "https://images.unsplash.com/photo-1610557892470-55d9e80c0bce?w=400&auto=format&fit=crop&q=80"},
	{"HOU-DSH-049", "Lemon Antibacterial Dish Soap 500ml", "Household", "bottle", 4500, 2600, 34, 8, "https://images.unsplash.com/photo-1585670270608-b404fb880c71?w=400&auto=format&fit=crop&q=80"},
	{"HOU-FLR-050", "Multi-Surface Floor Cleaner 1L", "Household", "bottle", 12000, 7500, 20, 5, "https://images.unsplash.com/photo-1584813470613-5b1c1cad3d69?w=400&auto=format&fit=crop&q=80"},
}

func generateEan13(index int) string {
	base := fmt.Sprintf("640000%06d", index+1)
	sum := 0
	for i := 0; i < 12; i++ {
		digit := int(base[i] - '0')
		if i%2 == 0 {
			sum += digit
		} else {
			sum += digit * 3
		}
	}
	return base + fmt.Sprintf("%d", (10-sum%10)%10)
}

func deterministicUUID(label string) string {
	sum := sha256.Sum256([]byte("cm-retailos-50|" + label))
	sum[6] = (sum[6] & 0x0f) | 0x40
	sum[8] = (sum[8] & 0x3f) | 0x80
	hex := fmt.Sprintf("%x", sum[:16])
	return hex[0:8] + "-" + hex[8:12] + "-" + hex[12:16] + "-" + hex[16:20] + "-" + hex[20:32]
}

type seededProduct struct {
	id       int64
	sku      string
	name     string
	selling  int64
	cost     int64
	quantity float64
}

func main() {
	ctx := context.Background()
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		databaseURL = "postgres://retailos:348666fc6e87155e6520b22371c0ca4b3820f957094116fb1d5eb66a45f42f30@127.0.0.1:5432/cm_retailos_online?sslmode=disable"
	}
	targetShopID := strings.TrimSpace(os.Getenv("SHOP_ID"))
	if targetShopID == "" {
		targetShopID = "89d88224-436d-4d30-9ed6-ebc071bc23ea" // Default to Christopher Muheki's "My Shop"
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		log.Fatal(err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer tx.Rollback()

	var shopName, currency string
	err = tx.QueryRowContext(ctx, `SELECT name, currency FROM shops WHERE id = $1 FOR UPDATE`, targetShopID).Scan(&shopName, &currency)
	if err != nil {
		log.Fatalf("shop %s not found: %v", targetShopID, err)
	}

	var ownerID string
	err = tx.QueryRowContext(ctx, `SELECT id::text FROM users WHERE shop_id = $1 AND role = 'owner' AND active = TRUE ORDER BY created_at LIMIT 1`, targetShopID).Scan(&ownerID)
	if err != nil {
		log.Fatalf("owner for shop %s not found: %v", targetShopID, err)
	}

	fmt.Printf("Seeding 50 demo products and 50 sales for shop %s (%s)...\n", targetShopID, shopName)

	// Clean out old demo sales and items for fresh seed
	if _, err := tx.ExecContext(ctx, `DELETE FROM stock_movements WHERE shop_id = $1`, targetShopID); err != nil {
		log.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM sale_items WHERE sale_id IN (SELECT id FROM sales WHERE shop_id = $1)`, targetShopID); err != nil {
		log.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM sales WHERE shop_id = $1`, targetShopID); err != nil {
		log.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM products WHERE shop_id = $1`, targetShopID); err != nil {
		log.Fatal(err)
	}

	now := time.Now()
	insertedProducts := make([]seededProduct, 0, len(demoProducts))

	for idx, item := range demoProducts {
		_, err := tx.ExecContext(ctx, `INSERT INTO categories (shop_id, name) VALUES ($1, $2) ON CONFLICT DO NOTHING`, targetShopID, item.category)
		if err != nil {
			log.Fatal(err)
		}

		barcode := generateEan13(idx)
		var insertedID int64
		err = tx.QueryRowContext(ctx, `
			INSERT INTO products (shop_id, sku, name, category, barcode, unit, selling_price, cost_price, quantity, min_stock, image)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11) RETURNING id`,
			targetShopID, item.sku, item.name, item.category, barcode, item.unit, item.selling, item.cost, item.quantity, item.minStock, item.image).Scan(&insertedID)
		if err != nil {
			log.Fatalf("insert product %s failed: %v", item.name, err)
		}

		sp := seededProduct{
			id:       insertedID,
			sku:      item.sku,
			name:     item.name,
			selling:  item.selling,
			cost:     item.cost,
			quantity: item.quantity,
		}
		insertedProducts = append(insertedProducts, sp)

		if item.quantity > 0 {
			_, err = tx.ExecContext(ctx, `
				INSERT INTO stock_movements (shop_id, product_id, user_id, quantity_delta, reason, created_at)
				VALUES ($1, $2, $3, $4, 'Demo opening stock', $5)`,
				targetShopID, insertedID, ownerID, item.quantity, now)
			if err != nil {
				log.Fatal(err)
			}
		}
	}

	customers := []string{
		"Sarah Kigozi", "David Mukasa", "Amina Nalubega", "Brian Okello", "Grace Tumusiime",
		"Patrick Waiswa", "Ritah Atuhaire", "Walk-in Customer", "Fatuma Kemigisha", "Christine Nabatanzi",
		"Moses Kato", "Doreen Namutebi", "Isaac Mugisha", "Hellen Kyomugisha", "Robert Asiimwe",
		"Agnes Mbabazi", "Joseph Ssempijja", "Florence Namaganda", "John Bosco", "Prossy Nanteza",
		"Charles Lwanga", "Stella Namubiru", "Geoffrey Otim", "Brenda Akello", "Emmanuel Kaggwa",
		"Juliet Nabirye", "Ronald Kigozi", "Beatrice Alupo", "Denis Mugerwa", "Sandra Nakato",
		"Gerald Katende", "Mariam Nabukenya", "Ivan Wasswa", "Lillian Namagembe", "Samson Byaruhanga",
		"Harriet Nakabugo", "Timothy Musoke", "Diana Kobusinge", "Fred Sekitoleko", "Winnie Nassolo",
		"Arthur Kiwanuka", "Mercy Ainembabazi", "Martin Lubega", "Patricia Nakayiza", "Victor Tumwine",
		"Rebecca Namubiru", "Daniel Kato", "Judith Nansubuga", "Herbert Wandera", "Evelyn Birungi",
	}

	payments := []string{"cash", "mobile_money", "card"}
	rng := rand.New(rand.NewSource(20261111))

	for saleIdx := 0; saleIdx < 50; saleIdx++ {
		var completedAt time.Time
		if saleIdx < 12 {
			// Today
			hour := 8 + (saleIdx * 10 / 12)
			min := rng.Intn(55)
			completedAt = time.Date(now.Year(), now.Month(), now.Day(), hour, min, 0, 0, now.Location())
		} else if saleIdx < 22 {
			// Yesterday
			hour := 8 + ((saleIdx - 12) * 10 / 10)
			min := rng.Intn(55)
			y := now.AddDate(0, 0, -1)
			completedAt = time.Date(y.Year(), y.Month(), y.Day(), hour, min, 0, 0, now.Location())
		} else if saleIdx < 44 {
			// 2 to 6 days ago
			daysAgo := 2 + (saleIdx-22)/4
			hour := 9 + rng.Intn(9)
			min := rng.Intn(55)
			d := now.AddDate(0, 0, -daysAgo)
			completedAt = time.Date(d.Year(), d.Month(), d.Day(), hour, min, 0, 0, now.Location())
		} else {
			// 1 to 4 weeks ago
			daysAgo := 7 + (saleIdx-44)*3
			hour := 10 + rng.Intn(8)
			min := rng.Intn(55)
			d := now.AddDate(0, 0, -daysAgo)
			completedAt = time.Date(d.Year(), d.Month(), d.Day(), hour, min, 0, 0, now.Location())
		}

		customer := customers[saleIdx%len(customers)]
		payment := payments[saleIdx%len(payments)]

		itemCount := rng.Intn(3) + 1
		subtotal := int64(0)

		type itemSpec struct {
			prod     seededProduct
			quantity int64
			line     int64
		}
		items := make([]itemSpec, 0, itemCount)

		// Pick distinct products
		used := make(map[int]bool)
		for it := 0; it < itemCount; it++ {
			pIdx := (saleIdx*3 + it*7) % len(insertedProducts)
			if used[pIdx] {
				pIdx = (pIdx + 1) % len(insertedProducts)
			}
			used[pIdx] = true
			prod := insertedProducts[pIdx]
			qty := int64(rng.Intn(3) + 1)
			line := prod.selling * qty
			subtotal += line
			items = append(items, itemSpec{prod: prod, quantity: qty, line: line})
		}

		discount := int64(0)
		if saleIdx%7 == 0 {
			discount = int64(rng.Intn(5)+1) * 1000
			if discount >= subtotal {
				discount = 0
			}
		}
		total := subtotal - discount

		saleID := deterministicUUID(fmt.Sprintf("%s-sale-50-%03d", targetShopID, saleIdx+1))
		_, err = tx.ExecContext(ctx, `
			INSERT INTO sales (id, shop_id, cashier_id, cashier_name, customer_name, subtotal, discount, total, payment_method, completed_at)
			VALUES ($1, $2, $3, 'Shop Owner', $4, $5, $6, $7, $8, $9)`,
			saleID, targetShopID, ownerID, customer, subtotal, discount, total, payment, completedAt)
		if err != nil {
			log.Fatalf("insert sale %d failed: %v", saleIdx+1, err)
		}

		for _, it := range items {
			_, err = tx.ExecContext(ctx, `
				INSERT INTO sale_items (sale_id, product_id, product_name, quantity, unit_price, unit_cost, line_total)
				VALUES ($1, $2, $3, $4, $5, $6, $7)`,
				saleID, it.prod.id, it.prod.name, it.quantity, it.prod.selling, it.prod.cost, it.line)
			if err != nil {
				log.Fatalf("insert sale item failed: %v", err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Successfully populated %s with %d products and 50 sales records!\n", targetShopID, len(insertedProducts))
}
