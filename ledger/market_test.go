package ledger

import "testing"

func TestMergeMarketKeepsNewestScans(t *testing.T) {
	dst := marketDisk{Scans: []marketScan{
		{T: 100, Items: map[string]marketItem{"1": {Name: "Linen Cloth", Qty: 10, Listings: 2, Unit: 50}}},
		{T: 90, Items: map[string]marketItem{"1": {Name: "Linen Cloth", Qty: 8, Listings: 1, Unit: 40}}},
	}}
	src := marketDisk{Scans: []marketScan{
		{T: 100, Items: map[string]marketItem{"2": {Name: "Wool Cloth", Qty: 4, Listings: 1, Unit: 80}}},
		{T: 110, Items: map[string]marketItem{"2": {Name: "Wool Cloth", Qty: 5, Listings: 2, Unit: 75}}},
	}}
	got := mergeMarket(dst, src)
	if len(got.Scans) != 3 {
		t.Fatalf("len %d", len(got.Scans))
	}
	if got.Scans[0].T != 110 || got.Scans[1].T != 100 || got.Scans[2].T != 90 {
		t.Fatalf("order %+v", got.Scans)
	}
	if got.Scans[1].Items["1"].Name != "Linen Cloth" || got.Scans[1].Items["2"].Name != "Wool Cloth" {
		t.Fatalf("merged scan %+v", got.Scans[1].Items)
	}
}

func TestSearchMarketRanksPrefixHits(t *testing.T) {
	disk := marketDisk{Scans: []marketScan{{
		T: 1700000000,
		Items: map[string]marketItem{
			"2589":  {Name: "Linen Cloth", Qty: 20, Listings: 3, Unit: 12},
			"2592":  {Name: "Wool Cloth", Qty: 10, Listings: 2, Unit: 40},
			"4306":  {Name: "Silk Cloth", Qty: 8, Listings: 1, Unit: 90},
			"14047": {Name: "Runecloth", Qty: 6, Listings: 1, Unit: 200},
		},
	}}}
	hits := searchMarket(disk, "cloth")
	if len(hits) != 4 {
		t.Fatalf("len %d %+v", len(hits), hits)
	}
	if hits[0].Name != "Linen Cloth" || hits[0].ItemID != 2589 {
		t.Fatalf("first %+v", hits[0])
	}
	if hits[1].Name != "Runecloth" {
		t.Fatalf("second %+v", hits[1])
	}
	if len(hits[0].Points) != 1 || hits[0].Points[0].Day == "" || hits[0].Points[0].Unit != 12 {
		t.Fatalf("points %+v", hits[0].Points)
	}
	if len(searchMarket(disk, "c")) != 0 {
		t.Fatal("short query should be empty")
	}
}

func TestFindMarketItem(t *testing.T) {
	disk := marketDisk{Scans: []marketScan{
		{T: 1700000000, Items: map[string]marketItem{"13463": {Name: "Dreamfoil", Qty: 20, Listings: 4, Unit: 50000}}},
		{T: 1700086400, Items: map[string]marketItem{"13463": {Name: "Dreamfoil", Qty: 12, Listings: 2, Unit: 48000}}},
	}}
	hit := findMarketItem(disk, 13463)
	if hit == nil || hit.Name != "Dreamfoil" || len(hit.Points) != 2 {
		t.Fatalf("%+v", hit)
	}
	if hit.Points[0].Unit != 50000 || hit.Points[1].Unit != 48000 {
		t.Fatalf("order %+v", hit.Points)
	}
	if findMarketItem(disk, 0) != nil || findMarketItem(disk, 99) != nil {
		t.Fatal("missing ids should be nil")
	}
}
