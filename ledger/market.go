package ledger

import (
	"encoding/json"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

const marketKey = "AuctionAssistant-market"

type marketItem struct {
	Name     string `json:"name"`
	Qty      int64  `json:"qty"`
	Listings int64  `json:"listings"`
	Unit     int64  `json:"unit"`
}

type marketScan struct {
	T     int64                 `json:"t"`
	Items map[string]marketItem `json:"items"`
}

type marketDisk struct {
	Scans []marketScan `json:"scans"`
}

type marketPoint struct {
	T        int64  `json:"t"`
	Day      string `json:"day"`
	Unit     int64  `json:"unit"`
	Qty      int64  `json:"qty"`
	Listings int64  `json:"listings"`
	Scans    int    `json:"scans"`
}

type marketHit struct {
	ItemID int64         `json:"itemId"`
	Name   string        `json:"name"`
	Points []marketPoint `json:"points"`
}

func serveMarket(w http.ResponseWriter, r *http.Request) {
	cors(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if databaseURL() == "" {
		writeJSON(w, http.StatusServiceUnavailable, []byte(`{"error":"Set DATABASE_URL to the Neon connection string."}`))
		return
	}
	if r.Method == http.MethodPost {
		if !BearerOk(r.Header.Get("authorization"), os.Getenv("LEDGER_TOKEN")) {
			writeJSON(w, http.StatusUnauthorized, []byte(`{"error":"unauthorized"}`))
			return
		}
		var body marketDisk
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, []byte(`{"error":"bad json"}`))
			return
		}
		disk := loadMarket()
		disk = mergeMarket(disk, body)
		encoded, _ := json.Marshal(disk)
		if err := kvSet(marketKey, string(encoded)); err != nil {
			writeJSON(w, http.StatusBadGateway, errorJSON(err))
			return
		}
		writeJSON(w, http.StatusOK, []byte(`{"ok":true}`))
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, []byte(`{"error":"method"}`))
		return
	}
	disk := loadMarket()
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id != "" {
		itemID, _ := strconv.ParseInt(id, 10, 64)
		body, _ := json.Marshal(struct {
			Item *marketHit `json:"item"`
		}{Item: findMarketItem(disk, itemID)})
		writeJSON(w, http.StatusOK, body)
		return
	}
	body, _ := json.Marshal(struct {
		Items []marketHit `json:"items"`
	}{Items: searchMarket(disk, r.URL.Query().Get("q"))})
	writeJSON(w, http.StatusOK, body)
}

func loadMarket() marketDisk {
	raw, err := kvGet(marketKey)
	if err != nil || raw == "" || raw == "null" {
		return marketDisk{}
	}
	var disk marketDisk
	_ = json.Unmarshal([]byte(raw), &disk)
	return disk
}

func mergeMarket(dst, src marketDisk) marketDisk {
	byT := map[int64]*marketScan{}
	order := make([]int64, 0, len(dst.Scans)+len(src.Scans))
	for i := range dst.Scans {
		scan := dst.Scans[i]
		if scan.Items == nil {
			scan.Items = map[string]marketItem{}
		}
		cp := scan
		byT[scan.T] = &cp
		order = append(order, scan.T)
	}
	for _, scan := range src.Scans {
		if scan.T == 0 {
			continue
		}
		found := byT[scan.T]
		if found == nil {
			items := map[string]marketItem{}
			for id, row := range scan.Items {
				items[id] = row
			}
			cp := marketScan{T: scan.T, Items: items}
			byT[scan.T] = &cp
			order = append(order, scan.T)
			continue
		}
		if found.Items == nil {
			found.Items = map[string]marketItem{}
		}
		for id, row := range scan.Items {
			found.Items[id] = row
		}
	}
	sort.Slice(order, func(i, j int) bool { return order[i] > order[j] })
	seen := map[int64]bool{}
	out := marketDisk{Scans: make([]marketScan, 0, 24)}
	for _, t := range order {
		if seen[t] {
			continue
		}
		seen[t] = true
		if scan := byT[t]; scan != nil {
			out.Scans = append(out.Scans, *scan)
		}
		if len(out.Scans) >= 24 {
			break
		}
	}
	return out
}

func catalog(disk marketDisk) map[int64]*marketHit {
	items := map[int64]*marketHit{}
	for _, scan := range disk.Scans {
		for idText, row := range scan.Items {
			itemID, _ := strconv.ParseInt(idText, 10, 64)
			if itemID <= 0 || scan.T <= 0 {
				continue
			}
			hit := items[itemID]
			if hit == nil {
				hit = &marketHit{ItemID: itemID, Name: row.Name}
				items[itemID] = hit
			}
			if row.Name != "" {
				hit.Name = row.Name
			}
			hit.Points = append(hit.Points, marketPoint{
				T:        scan.T,
				Unit:     row.Unit,
				Qty:      row.Qty,
				Listings: row.Listings,
			})
		}
	}
	for _, hit := range items {
		hit.Points = dailyPoints(hit.Points)
	}
	return items
}

func dayKey(unix int64) string {
	date := time.Unix(unix, 0).UTC()
	return date.Format("2006-01-02")
}

func dailyPoints(points []marketPoint) []marketPoint {
	byDay := map[string]marketPoint{}
	order := make([]string, 0, len(points))
	for _, point := range points {
		day := dayKey(point.T)
		prev, ok := byDay[day]
		if !ok {
			point.Day = day
			point.Scans = 1
			byDay[day] = point
			order = append(order, day)
			continue
		}
		prev.Scans++
		if point.T >= prev.T {
			point.Day = day
			point.Scans = prev.Scans
			byDay[day] = point
		} else {
			byDay[day] = prev
		}
	}
	sort.Strings(order)
	out := make([]marketPoint, 0, len(order))
	for _, day := range order {
		out = append(out, byDay[day])
	}
	return out
}

func searchMarket(disk marketDisk, query string) []marketHit {
	q := strings.ToLower(strings.TrimSpace(query))
	if len(q) < 2 {
		return []marketHit{}
	}
	type ranked struct {
		hit  marketHit
		rank int
	}
	var hits []ranked
	for _, item := range catalog(disk) {
		name := strings.ToLower(item.Name)
		if !strings.Contains(name, q) {
			continue
		}
		rank := 1
		if strings.HasPrefix(name, q) {
			rank = 0
		}
		hits = append(hits, ranked{hit: *item, rank: rank})
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].rank != hits[j].rank {
			return hits[i].rank < hits[j].rank
		}
		if hits[i].hit.Name != hits[j].hit.Name {
			return hits[i].hit.Name < hits[j].hit.Name
		}
		return hits[i].hit.ItemID < hits[j].hit.ItemID
	})
	if len(hits) > 12 {
		hits = hits[:12]
	}
	out := make([]marketHit, 0, len(hits))
	for _, row := range hits {
		out = append(out, row.hit)
	}
	return out
}

func findMarketItem(disk marketDisk, itemID int64) *marketHit {
	if itemID <= 0 {
		return nil
	}
	if hit := catalog(disk)[itemID]; hit != nil && hit.Name != "" {
		return hit
	}
	return nil
}
