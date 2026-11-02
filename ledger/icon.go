package ledger

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
)

var itemID = regexp.MustCompile(`^[1-9]\d*$`)
var iconName = regexp.MustCompile(`^[a-z0-9_]+$`)

func IconFromTooltip(body string) string {
	var data struct {
		Icon string `json:"icon"`
	}
	if json.Unmarshal([]byte(body), &data) != nil {
		return ""
	}
	icon := strings.ToLower(data.Icon)
	if !iconName.MatchString(icon) {
		return ""
	}
	return icon
}

func fetchBytes(url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("user-agent", "AuctionAssistant")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, os.ErrNotExist
	}
	return io.ReadAll(res.Body)
}
