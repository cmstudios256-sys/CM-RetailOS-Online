package httpapi

import (
	"net/http"
	"strconv"
	"time"
)

type publicWorkspaceResponse struct {
	ShopName string `json:"shopName"`
	Currency string `json:"currency"`
	Demo     bool   `json:"demo"`
}

type publicDashboardProduct struct {
	ID       int64   `json:"id"`
	Name     string  `json:"name"`
	Unit     string  `json:"unit"`
	Quantity float64 `json:"quantity"`
	MinStock float64 `json:"minStock"`
}

func (s *Server) publicWorkspace(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, publicWorkspaceResponse{
		ShopName: "Demo Shop",
		Currency: "UGX",
		Demo:     true,
	})
}

func (s *Server) publicDashboard(w http.ResponseWriter, _ *http.Request) {
	now := time.Now()
	chart := make([]dashboardChartPoint, 24)
	for hour := range chart {
		chart[hour] = dashboardChartPoint{
			Label: strconv.Itoa(hour) + ":00",
		}
	}
	chart[now.Hour()].Sales = 21000
	chart[now.Hour()].Profit = 8000
	writeJSON(w, http.StatusOK, map[string]any{
		"productCount":     3,
		"lowStockCount":    1,
		"inventoryValue":   231200,
		"todaySales":       2,
		"todayRevenue":     21000,
		"currency":         "UGX",
		"revenue":          21000,
		"profit":           8000,
		"previousRevenue":  24000,
		"previousProfit":   8000,
		"orderCount":       2,
		"previousOrders":   1,
		"unitsSold":        3,
		"chart":            chart,
		"topProducts":      []dashboardTopProduct{{Name: "Sample Notebook", QuantitySold: 2, Revenue: 9000}, {Name: "Demo Coffee Beans", QuantitySold: 1, Revenue: 12000}},
		"profitByCategory": []dashboardProfitCategory{{Name: "Demo Groceries", Value: 4000}, {Name: "Demo Stationery", Value: 4000}},
	})
}

func (s *Server) publicProducts(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string][]publicDashboardProduct{
		"products": {
			{ID: 1, Name: "Demo Coffee Beans", Unit: "bag", Quantity: 18, MinStock: 5},
			{ID: 2, Name: "Sample Notebook", Unit: "piece", Quantity: 32, MinStock: 8},
			{ID: 3, Name: "Example Snack Pack", Unit: "pack", Quantity: 4, MinStock: 6},
		},
	})
}
