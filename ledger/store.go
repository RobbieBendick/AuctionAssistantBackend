// Ledger rules: the addon's saved-variable string becomes events, duplicates merge,
// and the three kinds are totaled.
package ledger

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var kinds = map[string]bool{"post": true, "buy": true, "sold": true, "cancel": true}

var (
	wireQuoted = regexp.MustCompile(`\["ledgerWire"\]\s*=\s*"((?:\\.|[^"\\])*)"`)
	wireBare   = regexp.MustCompile(`(?:^|\n)\s*ledgerWire\s*=\s*"((?:\\.|[^"\\])*)"`)
	digits     = regexp.MustCompile(`^\d+$`)
)

type Event struct {
	ID        string `json:"id"`
	T         int64  `json:"t"`
	Kind      string `json:"kind"`
	ItemID    int64  `json:"itemId"`
	Count     int64  `json:"count"`
	Copper    int64  `json:"copper"`
	Character string `json:"character"`
	Realm     string `json:"realm"`
	Name      string `json:"name"`
	Duration  int64  `json:"duration,omitempty"`
	Icon      string `json:"icon,omitempty"`
}

type Summary struct {
	Posted int64 `json:"posted"`
	Bought int64 `json:"bought"`
	Sold   int64 `json:"sold"`
	Net    int64 `json:"net"`
	Count  int64 `json:"count"`
}

func unescapeLua(value string) string {
	value = strings.ReplaceAll(value, `\n`, "\n")
	value = strings.ReplaceAll(value, `\r`, "")
	value = strings.ReplaceAll(value, `\"`, `"`)
	value = strings.ReplaceAll(value, `\\`, `\`)
	return value
}

func extractWire(src string) string {
	if m := wireQuoted.FindStringSubmatch(src); m != nil {
		return unescapeLua(m[1])
	}
	if m := wireBare.FindStringSubmatch(src); m != nil {
		return unescapeLua(m[1])
	}
	return ""
}

func ParseLine(line string) (Event, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return Event{}, false
	}
	parts := strings.Split(line, "|")
	id := parts[0]
	if id == "" {
		return Event{}, false
	}
	short := ""
	if len(parts) >= 6 {
		switch parts[1] {
		case "b":
			short = "buy"
		case "p":
			short = "post"
		case "c":
			short = "cancel"
		}
	}
	if short != "" {
		duration := int64(0)
		nameAt := 5
		if len(parts) >= 7 && digits.MatchString(parts[5]) {
			duration = atoi(parts[5])
			nameAt = 6
		}
		name := strings.TrimSpace(strings.Join(parts[nameAt:], "|"))
		if name == "" {
			return Event{}, false
		}
		return Event{
			ID: id, T: atoi(strings.Split(id, "-")[0]), Kind: short,
			ItemID: atoi(parts[2]), Count: atoi(parts[3]), Copper: atoi(parts[4]),
			Character: characterFromID(id),
			Name: name, Duration: duration,
		}, true
	}
	if len(parts) < 9 || !kinds[parts[2]] {
		return Event{}, false
	}
	duration := int64(0)
	name := ""
	if len(parts) >= 10 && digits.MatchString(parts[8]) {
		duration = atoi(parts[8])
		name = strings.TrimSpace(strings.Join(parts[9:], "|"))
	} else {
		name = strings.TrimSpace(strings.Join(parts[8:], "|"))
	}
	if name == "" {
		return Event{}, false
	}
	return Event{
		ID: id, T: atoi(parts[1]), Kind: parts[2],
		ItemID: atoi(parts[3]), Count: atoi(parts[4]), Copper: atoi(parts[5]),
		Character: parts[6], Realm: parts[7], Name: name, Duration: duration,
	}, true
}

func ParseWire(src string) []Event {
	wire := src
	if strings.Contains(src, "ledgerWire") || strings.Contains(src, `\n`) {
		wire = extractWire(src)
	}
	var events []Event
	for _, line := range strings.Split(wire, "\n") {
		if event, ok := ParseLine(line); ok {
			events = append(events, event)
		}
	}
	if events == nil {
		events = []Event{}
	}
	return events
}

func MergeEvents(existing, incoming []Event) []Event {
	byID := map[string]Event{}
	var order []string
	put := func(event Event) {
		if event.ID == "" {
			return
		}
		if _, seen := byID[event.ID]; !seen {
			order = append(order, event.ID)
		}
		if event.Character == "" {
			event.Character = characterFromID(event.ID)
		}
		if prev, ok := byID[event.ID]; ok {
			if event.Character == "" {
				event.Character = prev.Character
			}
			if event.Realm == "" {
				event.Realm = prev.Realm
			}
			if event.T == 0 {
				event.T = prev.T
			}
			if event.Duration == 0 {
				event.Duration = prev.Duration
			}
			if event.Icon == "" {
				event.Icon = prev.Icon
			}
		}
		byID[event.ID] = event
	}
	for _, event := range existing {
		put(event)
	}
	for _, event := range incoming {
		put(event)
	}
	out := make([]Event, 0, len(order))
	for _, id := range order {
		out = append(out, byID[id])
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].T != out[j].T {
			return out[i].T > out[j].T
		}
		return out[i].ID > out[j].ID
	})
	return out
}

func Summarize(events []Event) Summary {
	var totals Summary
	for _, event := range events {
		switch event.Kind {
		case "post":
			totals.Posted += event.Copper
		case "buy":
			totals.Bought += event.Copper
		case "sold":
			totals.Sold += event.Copper
		default:
			continue
		}
		totals.Count++
	}
	totals.Net = totals.Sold - totals.Bought
	return totals
}

func BearerOk(header, expected string) bool {
	return expected != "" && header == "Bearer "+expected
}

func characterFromID(id string) string {
	parts := strings.SplitN(id, "-", 3)
	if len(parts) < 3 {
		return ""
	}
	tag := strings.TrimSpace(parts[2])
	if tag == "" {
		return ""
	}
	for _, r := range tag {
		if (r < '0' || r > '9') && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') && r != '_' {
			return ""
		}
	}
	return tag
}

func atoi(value string) int64 {
	n, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0
	}
	return n
}
