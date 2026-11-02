package ledger

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
)

const ledgerKey = "AuctionAssistant-ledger"

func Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/ledger", serveLedger)
	mux.HandleFunc("/api/icon/", serveIcon)
	return mux
}

func serveLedger(w http.ResponseWriter, r *http.Request) {
	cors(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if databaseURL() == "" {
		writeJSON(w, http.StatusServiceUnavailable, []byte(`{"error":"Set DATABASE_URL to the Neon connection string."}`))
		return
	}
	if r.Method == http.MethodPost && !BearerOk(r.Header.Get("authorization"), os.Getenv("LEDGER_TOKEN")) {
		writeJSON(w, http.StatusUnauthorized, []byte(`{"error":"unauthorized"}`))
		return
	}
	raw, err := kvGet(ledgerKey)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorJSON(err))
		return
	}
	events := []Event{}
	if raw != "" && raw != "null" {
		_ = json.Unmarshal([]byte(raw), &events)
	}
	if r.Method == http.MethodPost {
		var body struct {
			Events []Event `json:"events"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		events = MergeEvents(events, body.Events)
		encoded, _ := json.Marshal(events)
		if err := kvSet(ledgerKey, string(encoded)); err != nil {
			writeJSON(w, http.StatusBadGateway, errorJSON(err))
			return
		}
	} else if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, []byte(`{"error":"method"}`))
		return
	}
	body, _ := json.Marshal(struct {
		Events  []Event `json:"events"`
		Summary Summary `json:"summary"`
	}{events, Summarize(events)})
	writeJSON(w, http.StatusOK, body)
}

func serveIcon(w http.ResponseWriter, r *http.Request) {
	cors(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/icon/")
	if !itemID.MatchString(id) {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	icon, _ := kvGet("AuctionAssistant-icon:" + id)
	if icon == "" {
		tip, err := fetchBytes("https://nether.wowhead.com/tooltip/item/" + id)
		if err != nil {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		icon = IconFromTooltip(string(tip))
		if icon == "" {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		_ = kvSet("AuctionAssistant-icon:"+id, icon)
	}
	w.Header().Set("location", "https://wow.zamimg.com/images/wow/icons/large/"+icon+".jpg")
	w.Header().Set("cache-control", "public, max-age=86400")
	w.WriteHeader(http.StatusFound)
}

func cors(w http.ResponseWriter) {
	h := w.Header()
	h.Set("access-control-allow-origin", "*")
	h.Set("access-control-allow-methods", "GET, POST, OPTIONS")
	h.Set("access-control-allow-headers", "authorization, content-type")
	h.Set("access-control-max-age", "86400")
}

func writeJSON(w http.ResponseWriter, status int, body []byte) {
	w.Header().Set("content-type", "application/json; charset=utf-8")
	w.Header().Set("cache-control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func errorJSON(err error) []byte {
	body, _ := json.Marshal(map[string]string{"error": err.Error()})
	return body
}
