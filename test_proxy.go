package main

import (
	"fmt"
	"net/http"
)

func main() {
	req, _ := http.NewRequest("POST", "/api/v1/ai/enrichment/batch", nil)
	client := &http.Client{}
	_, err := client.Do(req)
	fmt.Println("Error:", err)
}
