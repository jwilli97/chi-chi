package imperva

import (
	"fmt"
	"log"
	"os"
	"strconv"
)

// TestCookieFetch is a simple test function to verify cookie fetching works
// Usage: go run imperva/test_cookies.go <venue_id>
func TestCookieFetch() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run imperva/test_cookies.go <venue_id>")
		fmt.Println("Example: go run imperva/test_cookies.go 12345")
		os.Exit(1)
	}

	venueID, err := strconv.ParseInt(os.Args[1], 10, 64)
	if err != nil {
		log.Fatalf("Invalid venue ID: %v", err)
	}

	fmt.Printf("Fetching Imperva cookies for venue %d...\n", venueID)
	cookieData, err := FetchCookiesForAPI(venueID)
	if err != nil {
		log.Fatalf("Failed to fetch cookies: %v", err)
	}

	fmt.Printf("\nSuccessfully fetched cookies:\n")
	fmt.Printf("User-Agent: %s\n", cookieData.UserAgent)
	fmt.Printf("Number of cookies: %d\n\n", len(cookieData.Cookies))

	fmt.Println("Cookies:")
	for i, cookie := range cookieData.Cookies {
		fmt.Printf("%d. %s=%s (Domain: %s, Path: %s)\n",
			i+1, cookie.Name, cookie.Value[:min(20, len(cookie.Value))]+"...", cookie.Domain, cookie.Path)
	}

	fmt.Printf("\nCookie header string:\n%s\n", CookiesToHeaderString(cookieData.Cookies))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
