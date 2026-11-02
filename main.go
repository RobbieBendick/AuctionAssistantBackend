package main

import (
	"fmt"
	"net/http"
	"os"

	"AuctionAssistantBackend/ledger"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "4317"
	}
	fmt.Println("auction ledger api  http://0.0.0.0:" + port)
	if err := http.ListenAndServe(":"+port, ledger.Handler()); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
