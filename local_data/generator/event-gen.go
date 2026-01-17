package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"time"
)

func main() {
	types := []string{"impression", "click", "conversion"}
	countries := []string{"US", "IN", "DE", "FR", "UK", "CA", "AU", "BR", "JP", "SG"}
	devices := []string{"mobile", "desktop", "tablet"}
	sources := []string{"google", "facebook", "instagram", "tiktok", "linkedin"}
	campaigns := []string{"winden111", "shamli222", "pppppppppppppppppppp"}
	users := []string{"ADAM", "EVE", "JONAS"}

	events := make([]map[string]interface{}, 0, 1000)

	now := time.Now().Add(-time.Hour)

	for i := 0; i < 1000; i++ {
		eType := types[rand.Intn(len(types))]
		meta := map[string]interface{}{
			"source": sources[rand.Intn(len(sources))],
		}
		if eType == "conversion" {
			meta["revenue"] = float64(50 + rand.Intn(150))
			meta["currency"] = "USD"
		}

		events = append(events, map[string]interface{}{
			"event_type":  eType,
			"user_id":     users[rand.Intn(len(users))],
			"campaign_id": campaigns[rand.Intn(len(campaigns))],
			"geo_country": countries[rand.Intn(len(countries))],
			"device_type": devices[rand.Intn(len(devices))],
			"created_at":  now.Add(time.Duration(i) * time.Second).Format(time.RFC3339),
			"meta_data":   meta,
		})
	}

	b, _ := json.MarshalIndent(events, "", "  ")
	fmt.Println(string(b))
}
