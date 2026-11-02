package ledger

import "testing"

const lua = "AuctionAssistantDB = {\n\t[\"ledgerWire\"] = \"10-1|1700000000|post|13463|20|50000|Dave|Nightslayer|Dreamfoil\\n10-2|1700000100|buy|13463|5|8000|Dave|Nightslayer|Dreamfoil\\n10-3|1700000200|sold|13463|0|47500|Dave|Nightslayer|Dreamfoil\",\n}\n"

func TestReadsSavedVariableWire(t *testing.T) {
	events := ParseWire(lua)
	if len(events) != 3 {
		t.Fatalf("len %d", len(events))
	}
	got := events[0]
	if got.ID != "10-1" || got.T != 1700000000 || got.Kind != "post" || got.ItemID != 13463 || got.Count != 20 || got.Copper != 50000 || got.Character != "Dave" || got.Realm != "Nightslayer" || got.Name != "Dreamfoil" {
		t.Fatalf("%+v", got)
	}
	if events[2].Kind != "sold" || events[2].Copper != 47500 {
		t.Fatalf("%+v", events[2])
	}
}

func TestShortBarcodeKeepsSavedCharacter(t *testing.T) {
	barcode, ok := ParseLine("10-2|b|13463|5|8000|Dreamfoil")
	if !ok || barcode.Kind != "buy" || barcode.T != 10 || barcode.Copper != 8000 || barcode.Name != "Dreamfoil" {
		t.Fatalf("%+v %v", barcode, ok)
	}
	saved := ParseWire(lua)
	row := findID(MergeEvents([]Event{barcode}, saved), "10-2")
	if row.Character != "Dave" || row.Realm != "Nightslayer" {
		t.Fatalf("barcode then saved: %+v", row)
	}
	kept := findID(MergeEvents(saved, []Event{barcode}), "10-2")
	if kept.Character != "Dave" || len(MergeEvents(saved, []Event{barcode})) != 3 {
		t.Fatalf("saved then barcode: %+v", kept)
	}
	tagged, ok := ParseLine("10-9-Touge|c|21991|2|9900|Heavy Netherweave Bandage")
	if !ok || tagged.Kind != "cancel" || tagged.Character != "Touge" {
		t.Fatalf("tagged cancel: %+v %v", tagged, ok)
	}
	cancelled, ok := ParseLine("10-9|c|13463|20|50000|Dreamfoil")
	if !ok || cancelled.Kind != "cancel" || cancelled.Name != "Dreamfoil" || cancelled.Copper != 50000 {
		t.Fatalf("%+v %v", cancelled, ok)
	}
	posted, ok := ParseLine("10-1|p|13463|20|50000|86400|Dreamfoil")
	if !ok || posted.Kind != "post" || posted.Copper != 50000 || posted.Duration != 86400 {
		t.Fatalf("%+v", posted)
	}
	withPost := MergeEvents(saved, []Event{posted})
	if len(withPost) != 3 {
		t.Fatal(len(withPost))
	}
	first := findID(withPost, "10-1")
	if first.Character != "Dave" || first.Duration != 86400 {
		t.Fatalf("%+v", first)
	}
	full, ok := ParseLine("10-1|1700000000|post|13463|20|50000|Dave|Nightslayer|86400|Dreamfoil")
	if !ok || full.Duration != 86400 || full.Name != "Dreamfoil" || full.T != 1700000000 {
		t.Fatalf("%+v", full)
	}
}

func TestMergeTotals(t *testing.T) {
	first := ParseWire(lua)
	again := MergeEvents(first, first)
	if len(again) != 3 {
		t.Fatal(len(again))
	}
	added := MergeEvents(again, []Event{{
		ID: "10-4", T: 1700000300, Kind: "buy", ItemID: 1, Count: 1,
		Copper: 1000, Character: "Dave", Realm: "Nightslayer", Name: "Linen Cloth",
	}})
	if len(added) != 4 || added[0].Name != "Linen Cloth" {
		t.Fatalf("%+v", added)
	}
	totals := Summarize(added)
	if totals.Posted != 50000 || totals.Bought != 9000 || totals.Sold != 47500 || totals.Net != 38500 {
		t.Fatalf("%+v", totals)
	}
}

func TestBearer(t *testing.T) {
	if !BearerOk("Bearer secret", "secret") || BearerOk("Bearer no", "secret") || BearerOk("Bearer secret", "") {
		t.Fatal("bearer")
	}
}

func TestIconFromTooltip(t *testing.T) {
	if IconFromTooltip(`{"name":"Fish Oil","icon":"INV_Potion_64"}`) != "inv_potion_64" {
		t.Fatal("fish oil")
	}
	if IconFromTooltip(`{"icon":"../nope"}`) != "" || IconFromTooltip("not json") != "" {
		t.Fatal("rejected")
	}
}

func findID(events []Event, id string) Event {
	for _, event := range events {
		if event.ID == id {
			return event
		}
	}
	return Event{}
}
