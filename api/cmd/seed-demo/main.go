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

type productSeed struct {
	id       int64
	sku      string
	name     string
	category string
	barcode  string
	unit     string
	selling  int64
	cost     int64
	quantity float64
	minStock float64
}

var catalog = map[string][]struct {
	name   string
	unit   string
	price  int64
	margin float64
}{
	"Groceries":             {{"Sugar 1kg", "pack", 4500, .72}, {"Cooking Oil 500ml", "bottle", 6000, .66}, {"Rice 2kg", "pack", 8200, .70}, {"Posho Flour 2kg", "pack", 6200, .68}, {"Salt 1kg", "pack", 2800, .75}, {"Beans 1kg", "pack", 5200, .62}, {"Maize Flour 2kg", "pack", 4800, .69}, {"Bread Loaf", "loaf", 4200, .60}, {"Egg Tray", "tray", 13000, .65}, {"Milk Powder 400g", "tin", 14500, .71}, {"Groundnuts 500g", "pack", 7500, .55}, {"Tea Leaves 250g", "pack", 4500, .64}, {"Tomato Sauce", "bottle", 3500, .58}, {"Canned Tomatoes", "can", 3800, .62}, {"Banana Matooke", "bunch", 18000, .55}, {"Irish Potatoes 5kg", "bag", 12000, .60}, {"Ghee 500g", "tub", 10500, .67}, {"Breakfast Cereal", "pack", 17000, .64}, {"Noodles", "pack", 2500, .52}, {"Biscuits", "pack", 3000, .48}},
	"Beverages":             {{"Soda 500ml", "bottle", 1500, .61}, {"Soda 1L", "bottle", 3000, .64}, {"Mineral Water 500ml", "bottle", 1800, .58}, {"Mineral Water 1.5L", "bottle", 3000, .60}, {"Energy Drink 400ml", "can", 4000, .69}, {"Fresh Juice 500ml", "bottle", 3500, .57}, {"Instant Coffee 200g", "jar", 16000, .70}, {"Black Tea 100 bags", "box", 13000, .68}, {"Malt Drink", "bottle", 2500, .62}, {"Yoghurt 500ml", "bottle", 3500, .60}, {"Chilled Water 20L", "bottle", 9000, .55}, {"Fruit Drink 1L", "carton", 6000, .62}, {"Soya Drink", "bottle", 2500, .58}, {"Iced Coffee", "bottle", 4500, .53}, {"Sports Drink", "bottle", 3500, .66}, {"Pineapple Juice 1L", "carton", 6500, .60}, {"Mango Juice 500ml", "bottle", 3200, .56}, {"Chocolate Malt", "bottle", 2800, .61}, {"Milk 500ml", "bottle", 2200, .60}, {"Cocoa Drink 200g", "tin", 12000, .68}},
	"Snacks":                {{"Potato Crisps", "pack", 3500, .45}, {"Popcorn Snack", "pack", 2500, .42}, {"Roasted Groundnuts", "pack", 3000, .48}, {"Samosa 4-pack", "pack", 6000, .50}, {"Mandazi Pack", "pack", 4000, .52}, {"Chocolate Bar", "piece", 3500, .55}, {"Sweet Biscuits", "pack", 2800, .43}, {"Cheese Crackers", "pack", 4500, .46}, {"Candy Mix", "pack", 2200, .40}, {"Chewing Gum", "pack", 2000, .44}, {"Cookies", "pack", 3800, .49}, {"Doughnut", "piece", 1800, .50}, {"Meat Pie", "piece", 3500, .47}, {"Banana Crisps", "pack", 3200, .43}, {"Peanut Brittle", "pack", 3000, .42}, {"Corn Nuts", "pack", 2800, .45}, {"Fruit Bar", "piece", 2500, .44}, {"Cupcake", "piece", 3000, .48}, {"Mini Cake", "pack", 5500, .52}, {"Pretzels", "pack", 4200, .46}},
	"Stationery":            {{"Exercise Book A4", "piece", 2800, .52}, {"Exercise Book A5", "piece", 1800, .50}, {"Blue Pen", "piece", 1000, .38}, {"Black Marker", "piece", 3500, .44}, {"Pencil Set", "pack", 2500, .42}, {"School Eraser", "piece", 800, .35}, {"Geometry Set", "pack", 12000, .58}, {"Stapler", "piece", 9000, .60}, {"Staples Pack", "pack", 4500, .56}, {"Glue Stick", "piece", 5000, .48}, {"File Folder", "piece", 2200, .46}, {"Ring Binder", "piece", 7000, .55}, {"Plain Copy Paper", "ream", 22000, .70}, {"Ruled Notebook", "piece", 4500, .49}, {"Index Card Pack", "pack", 3000, .45}, {"Correction Fluid", "bottle", 5500, .52}, {"Highlighter", "piece", 3500, .47}, {"Permanent Marker", "piece", 4000, .49}, {"Packing Tape", "roll", 6500, .58}, {"Envelope Pack", "pack", 5000, .54}},
	"Household":             {{"Laundry Detergent 1kg", "pack", 18000, .68}, {"Dishwashing Liquid 500ml", "bottle", 7000, .60}, {"Bar Soap Pack", "pack", 6500, .57}, {"Toilet Cleaner", "bottle", 8000, .62}, {"Floor Cleaner 1L", "bottle", 12000, .65}, {"Mopping Bucket", "piece", 28000, .72}, {"Kitchen Sponge", "pack", 3500, .44}, {"Garbage Bags", "roll", 9000, .55}, {"Matches Box", "box", 1500, .50}, {"Torch Battery AA", "pack", 6000, .48}, {"Light Bulb LED", "piece", 14000, .66}, {"Laundry Basket", "piece", 22000, .68}, {"Cloth Hanger 5-pack", "pack", 8000, .62}, {"Cleaning Cloth", "piece", 4000, .45}, {"Dust Pan", "piece", 10000, .64}, {"Broom", "piece", 8500, .61}, {"Mosquito Coil Pack", "pack", 5500, .52}, {"Air Freshener", "can", 12500, .67}, {"Glass Cleaner", "bottle", 9000, .63}, {"Steel Wool Pack", "pack", 4500, .49}},
	"Personal Care":         {{"Toothpaste 100ml", "tube", 5500, .64}, {"Toothbrush", "piece", 4000, .45}, {"Bath Soap", "piece", 3500, .52}, {"Shampoo 400ml", "bottle", 9000, .61}, {"Body Lotion 400ml", "bottle", 12000, .63}, {"Roll-on Deodorant", "piece", 8500, .58}, {"Hair Oil", "bottle", 7000, .56}, {"Cotton Buds", "pack", 4500, .49}, {"Hand Sanitizer 250ml", "bottle", 6500, .60}, {"Face Tissue", "pack", 3000, .46}, {"Disposable Razors", "pack", 7000, .57}, {"Menstrual Pads", "pack", 7500, .60}, {"Vaseline 250ml", "tin", 9000, .62}, {"Hair Conditioner", "bottle", 10500, .64}, {"Lip Balm", "piece", 4500, .47}, {"Nail Cutter", "piece", 5000, .50}, {"Comb", "piece", 2500, .42}, {"Perfume Roll-on", "bottle", 8000, .55}, {"Skin Cream", "tube", 11000, .63}, {"Baby Powder", "tin", 7500, .61}},
	"Baby":                  {{"Baby Diapers Pack", "pack", 28000, .69}, {"Baby Wipes", "pack", 9000, .60}, {"Baby Bath Soap", "piece", 5500, .53}, {"Baby Lotion", "bottle", 12000, .64}, {"Baby Oil", "bottle", 8500, .59}, {"Baby Cereal", "pack", 16000, .67}, {"Baby Milk 400g", "tin", 18000, .70}, {"Baby Feeding Bottle", "piece", 13000, .65}, {"Baby Blanket", "piece", 25000, .72}, {"Pacifier", "piece", 7000, .57}, {"Teething Toy", "piece", 9000, .60}, {"Baby Socks", "pair", 8000, .55}, {"Baby Shampoo", "bottle", 9500, .62}, {"Changing Mat", "piece", 22000, .69}, {"Baby Bib", "piece", 6000, .51}, {"Nappy Rash Cream", "tube", 10000, .63}, {"Baby Towel", "piece", 18000, .68}, {"Cotton Wool", "pack", 5500, .48}, {"Baby Food Jar", "jar", 8000, .58}, {"Baby Detergent", "pack", 14000, .66}},
	"Airtime & Accessories": {{"MTN Airtime UGX 5k", "voucher", 5000, .95}, {"MTN Airtime UGX 10k", "voucher", 10000, .95}, {"Airtel Airtime UGX 5k", "voucher", 5000, .95}, {"USB Charging Cable", "piece", 12000, .68}, {"Phone Charger", "piece", 28000, .72}, {"Power Bank 10Ah", "piece", 65000, .75}, {"SIM Card", "piece", 3000, .50}, {"Phone Pouch", "piece", 15000, .64}, {"Screen Protector", "piece", 9000, .58}, {"Earphones", "piece", 18000, .66}, {"Memory Card 32GB", "piece", 35000, .70}, {"OTG Adapter", "piece", 10000, .60}, {"Bluetooth Speaker Mini", "piece", 55000, .74}, {"Extension Cable", "piece", 22000, .69}, {"Phone Stand", "piece", 14000, .61}, {"Headset", "piece", 25000, .67}, {"Car Charger", "piece", 18000, .65}, {"Wall Adapter", "piece", 16000, .63}, {"Data Cable Type-C", "piece", 15000, .67}, {"Lightning Cable", "piece", 18000, .69}},
	"Health":                {{"Paracetamol Tablets", "pack", 3000, .55}, {"Vitamin C Tablets", "pack", 8000, .60}, {"ORS Sachets", "pack", 4500, .58}, {"Antiseptic 250ml", "bottle", 9000, .62}, {"Bandage Roll", "piece", 5500, .52}, {"First Aid Kit", "kit", 35000, .72}, {"Antacid Tablets", "pack", 6500, .57}, {"Cough Syrup", "bottle", 12000, .64}, {"Digital Thermometer", "piece", 22000, .68}, {"Face Mask Pack", "pack", 7000, .54}, {"Hand Gloves Pack", "pack", 9000, .56}, {"Pain Relief Gel", "tube", 10000, .61}, {"Eye Drops", "bottle", 8000, .60}, {"Medical Tape", "roll", 5000, .53}, {"Cotton Wool Roll", "pack", 6500, .55}, {"Disposable Gloves", "pack", 12000, .59}, {"Pregnancy Test Kit", "piece", 12000, .63}, {"Insect Bite Cream", "tube", 9000, .60}, {"Antiseptic Wipes", "pack", 6500, .54}, {"Alcohol Swabs", "pack", 5500, .52}},
	"Hardware":              {{"Padlock Medium", "piece", 12000, .60}, {"Hammer", "piece", 18000, .62}, {"Screwdriver Set", "set", 22000, .64}, {"Tape Measure 5m", "piece", 15000, .58}, {"Wall Nails Pack", "pack", 8000, .50}, {"Door Hinges Pair", "pair", 9000, .55}, {"PVC Elbow", "piece", 3000, .52}, {"PVC Pipe 1m", "piece", 11000, .57}, {"Electrical Tape", "roll", 3500, .48}, {"Bulb Holder", "piece", 5000, .54}, {"Extension Socket", "piece", 16000, .66}, {"Wood Glue", "bottle", 9000, .59}, {"Paint Brush 2in", "piece", 7000, .51}, {"Utility Knife", "piece", 6500, .53}, {"Rubber Mallet", "piece", 14000, .61}, {"Hose Clamp Pack", "pack", 8000, .56}, {"Tile Spacer Pack", "pack", 4000, .47}, {"Wall Plug Pack", "pack", 5000, .49}, {"Padlock Small", "piece", 8000, .58}, {"Screw Pack", "pack", 6000, .50}},
}

func ean13(index int) string {
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
	sum := sha256.Sum256([]byte("cm-retailos-demo|" + label))
	copy(sum[6:8], sum[6:8])
	sum[6] = (sum[6] & 0x0f) | 0x40
	sum[8] = (sum[8] & 0x3f) | 0x80
	hex := fmt.Sprintf("%x", sum[:16])
	return hex[0:8] + "-" + hex[8:12] + "-" + hex[12:16] + "-" + hex[16:20] + "-" + hex[20:32]
}

func main() {
	ctx := context.Background()
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	demoShopID := strings.TrimSpace(os.Getenv("DEMO_SHOP_ID"))
	if databaseURL == "" || demoShopID == "" {
		log.Fatal("DATABASE_URL and DEMO_SHOP_ID are required")
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
	var isDemo bool
	err = tx.QueryRowContext(ctx, `SELECT name, currency, is_demo FROM shops WHERE id = $1 FOR UPDATE`, demoShopID).Scan(&shopName, &currency, &isDemo)
	if err == sql.ErrNoRows {
		log.Fatal("refusing seed: DEMO_SHOP_ID does not exist")
	}
	if err != nil {
		log.Fatal(err)
	}
	if !isDemo || strings.TrimSpace(shopName) != "Demo Shop" || strings.TrimSpace(currency) != "UGX" {
		log.Fatal("refusing seed: target is not the explicitly marked UGX Demo Shop")
	}

	var ownerID string
	err = tx.QueryRowContext(ctx, `SELECT id::text FROM users WHERE shop_id = $1 AND role = 'owner' AND active = TRUE ORDER BY created_at LIMIT 1`, demoShopID).Scan(&ownerID)
	if err == sql.ErrNoRows {
		log.Fatal("refusing seed: demo shop has no owner user")
	}
	if err != nil {
		log.Fatal(err)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM stock_movements WHERE shop_id = $1`, demoShopID); err != nil {
		log.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM sale_items WHERE sale_id IN (SELECT id FROM sales WHERE shop_id = $1)`, demoShopID); err != nil {
		log.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM sales WHERE shop_id = $1`, demoShopID); err != nil {
		log.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM products WHERE shop_id = $1`, demoShopID); err != nil {
		log.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM categories WHERE shop_id = $1`, demoShopID); err != nil {
		log.Fatal(err)
	}

	now := time.Now()
	rng := rand.New(rand.NewSource(20241118))
	products := make([]productSeed, 0, 220)
	nextID := int64(1)
	for category, items := range catalog {
		if _, err := tx.ExecContext(ctx, `INSERT INTO categories (shop_id, name) VALUES ($1, $2) ON CONFLICT DO NOTHING`, demoShopID, category); err != nil {
			log.Fatal(err)
		}
		for _, item := range items {
			price := item.price + int64(rng.Intn(11)-5)*100
			if price < 500 {
				price = 500
			}
			cost := int64(float64(price) * item.margin)
			state := nextID % 100
			minStock := float64(rng.Intn(9) + 4)
			quantity := 0.0
			if state < 77 {
				quantity = float64(rng.Intn(100) + int(minStock) + 5)
			} else if state < 92 {
				quantity = float64(rng.Intn(int(minStock)) + 1)
			}
			seed := productSeed{nextID, fmt.Sprintf("DEMO-%03d", nextID), item.name, category, ean13(int(nextID) - 1), item.unit, price, cost, quantity, minStock}
			var insertedID int64
			err := tx.QueryRowContext(ctx, `
				INSERT INTO products (shop_id, sku, name, category, barcode, unit, selling_price, cost_price, quantity, min_stock, image)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'') RETURNING id`,
				demoShopID, seed.sku, seed.name, seed.category, seed.barcode, seed.unit, seed.selling, seed.cost, seed.quantity, seed.minStock).Scan(&insertedID)
			if err != nil {
				log.Fatal(err)
			}
			seed.id = insertedID
			products = append(products, seed)
			if seed.quantity > 0 {
				if _, err := tx.ExecContext(ctx, `
					INSERT INTO stock_movements (shop_id, product_id, user_id, quantity_delta, reason, created_at)
					VALUES ($1,$2,$3,$4,'Demo opening stock',$5)`,
					demoShopID, seed.id, ownerID, seed.quantity, now); err != nil {
					log.Fatal(err)
				}
			}
			nextID++
		}
	}

	peakHours := []int{7, 7, 8, 12, 12, 13, 14, 17, 18, 18, 19, 18, 19, 10, 15, 20, 13}
	for saleIndex := 0; saleIndex < 2000; saleIndex++ {
		var completed time.Time
		if saleIndex < 36 {
			hour := peakHours[saleIndex%len(peakHours)]
			if saleIndex == 0 {
				hour = 13
			}
			completed = time.Date(now.Year(), now.Month(), now.Day(), hour, rng.Intn(60), 0, 0, now.Location())
		} else {
			daysAgo := 2 + rng.Intn(88)
			completed = now.AddDate(0, 0, -daysAgo)
			completed = time.Date(completed.Year(), completed.Month(), completed.Day(), peakHours[rng.Intn(len(peakHours))], rng.Intn(60), 0, 0, now.Location())
		}
		itemCount := rng.Intn(6) + 1
		subtotal := int64(0)
		itemValues := make([]struct {
			product  productSeed
			quantity int64
		}, 0, itemCount)
		for itemIndex := 0; itemIndex < itemCount; itemIndex++ {
			productIndex := rng.Intn(len(products))
			if saleIndex%2 == 0 && itemIndex == 0 {
				productIndex = 25
			}
			if saleIndex%11 == 0 && itemIndex == 0 {
				productIndex = 72
			}
			quantity := int64(rng.Intn(4) + 1)
			subtotal += products[productIndex].selling * quantity
			itemValues = append(itemValues, struct {
				product  productSeed
				quantity int64
			}{products[productIndex], quantity})
		}
		discount := int64(0)
		if saleIndex%9 == 0 {
			discount = subtotal * int64(rng.Intn(8)+5) / 100
		}
		total := subtotal - discount
		payment := []string{"cash", "mobile_money", "card"}[saleIndex%3]
		saleID := deterministicUUID(fmt.Sprintf("sale-%04d", saleIndex+1))
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO sales (id, shop_id, cashier_id, cashier_name, customer_name, subtotal, discount, total, payment_method, completed_at)
			VALUES ($1,$2,$3,'Demo Shop Owner','Demo Customer',$4,$5,$6,$7,$8)`,
			saleID, demoShopID, ownerID, subtotal, discount, total, payment, completed); err != nil {
			log.Fatal(err)
		}
		for _, item := range itemValues {
			lineTotal := item.product.selling * item.quantity
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO sale_items (sale_id, product_id, product_name, quantity, unit_price, unit_cost, line_total)
				VALUES ($1,$2,$3,$4,$5,$6,$7)`,
				saleID, item.product.id, item.product.name, item.quantity, item.product.selling, item.product.cost, lineTotal); err != nil {
				log.Fatal(err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Demo seed complete: shop=%s products=%d sales=%d categories=%d\\n", demoShopID, len(products), 2000, len(catalog))
}
