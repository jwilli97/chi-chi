/*
Author: Bruce Jagid
Created On: Aug 12, 2023
*/
package resy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/21Bruce/resolved-server/api"
	"github.com/21Bruce/resolved-server/config"
	"github.com/21Bruce/resolved-server/store"
)

/*
Name: API
Type: API interface struct
Purpose: This struct acts as the resy implementation of the
api interface.
Note: The only known working APIKey value can be located and
defaulted using the GetDefaultAPI function, but we leave
it exposed so front-facing wrappers may expose it as a
setting
*/
type API struct {
	APIKey    string
	Cookies   []*http.Cookie // Imperva cookies for bypassing WAF
	UserAgent string         // User agent matching the cookies
}

/*
Name: isCodeFail
Type: Internal Func
Purpose: Function which takes in an HTTP code and returns
true if it is not a success code and false otherwise
*/
func isCodeFail(code int) bool {
	fst := code / 100
	return (fst != 2)
}

/*
Name: byteToJSONString
Type: Internal Func
Purpose: Function which takes in a byte sequence
representing a JSON struct and returns a string
or error. Useful for debugging
*/
func byteToJSONString(data []byte) (string, error) {
	var out bytes.Buffer
	err := json.Indent(&out, data, "", " ")

	if err != nil {
		return "", err
	}

	d := out.Bytes()
	return string(d), nil
}

/*
Name: min
Type: Internal Func
Purpose: Function that determins the min of two ints
*/
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

/*
Name: SetCookies
Type: API Func
Purpose: Set Imperva cookies and user agent for the API client
*/
func (a *API) SetCookies(cookies []*http.Cookie, userAgent string) {
	a.Cookies = cookies
	if userAgent != "" {
		a.UserAgent = userAgent
	} else {
		// Default user agent if none provided
		a.UserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	}
}

/*
Name: addCookiesToRequest
Type: Internal Func
Purpose: Add Imperva cookies and user agent to HTTP request
*/
func (a *API) addCookiesToRequest(req *http.Request) {
	// Add cookies to request
	if len(a.Cookies) > 0 {
		for _, cookie := range a.Cookies {
			req.AddCookie(cookie)
		}
	}

	// Set user agent if available
	if a.UserAgent != "" {
		req.Header.Set("User-Agent", a.UserAgent)
	}
}

/*
Name: extractCookiesFromResponse
Type: Internal Func
Purpose: Extract cookies from HTTP response headers and update API client cookies
*/
func (a *API) extractCookiesFromResponse(resp *http.Response) {
	// Check if this is an Imperva response
	if resp.Header.Get("X-Cdn") == "Imperva" || resp.Header.Get("Server") == "nginx" {
		fmt.Println("Detected Imperva challenge response, extracting cookies...")

		// Parse Set-Cookie headers
		for _, cookieStr := range resp.Header.Values("Set-Cookie") {
			// Parse the cookie string manually
			parts := strings.Split(cookieStr, ";")
			if len(parts) > 0 {
				nameValue := strings.SplitN(parts[0], "=", 2)
				if len(nameValue) == 2 {
					cookieName := strings.TrimSpace(nameValue[0])
					cookieValue := nameValue[1]

					// Check if it's an Imperva cookie
					if strings.HasPrefix(cookieName, "_incap_") ||
						strings.HasPrefix(cookieName, "incap_ses_") ||
						strings.HasPrefix(cookieName, "_visid_") ||
						strings.HasPrefix(cookieName, "visid_incap_") ||
						strings.HasPrefix(cookieName, "nlbi_") {

						cookie := &http.Cookie{
							Name:   cookieName,
							Value:  cookieValue,
							Domain: ".resy.com",
							Path:   "/",
						}

						// Parse additional attributes
						for i := 1; i < len(parts); i++ {
							part := strings.TrimSpace(parts[i])
							if strings.HasPrefix(strings.ToLower(part), "domain=") {
								cookie.Domain = strings.TrimPrefix(part, "domain=")
							} else if strings.HasPrefix(strings.ToLower(part), "path=") {
								cookie.Path = strings.TrimPrefix(part, "path=")
							} else if strings.ToLower(part) == "secure" {
								cookie.Secure = true
							} else if strings.ToLower(part) == "httponly" {
								cookie.HttpOnly = true
							} else if strings.HasPrefix(strings.ToLower(part), "expires=") {
								// Parse expiration if needed
								expiresStr := strings.TrimPrefix(part, "expires=")
								if t, err := time.Parse(time.RFC1123, expiresStr); err == nil {
									cookie.Expires = t
								}
							}
						}

						// Add or update cookie
						found := false
						for i, existingCookie := range a.Cookies {
							if existingCookie.Name == cookie.Name {
								a.Cookies[i] = cookie
								found = true
								break
							}
						}
						if !found {
							a.Cookies = append(a.Cookies, cookie)
						}

						fmt.Printf("Extracted Imperva cookie: %s\n", cookie.Name)
					}
				}
			}
		}

		if len(a.Cookies) > 0 {
			fmt.Printf("Updated API client with %d Imperva cookies from challenge response\n", len(a.Cookies))
		}
	}
}

/*
Name: isImpervaChallenge
Type: Internal Func
Purpose: Check if an HTTP response is an Imperva challenge
*/
func isImpervaChallenge(resp *http.Response) bool {
	// Imperva can return 500, 403, or 503
	if resp.StatusCode != 500 && resp.StatusCode != 403 && resp.StatusCode != 503 {
		return false
	}
	// Check for Imperva headers
	if resp.Header.Get("X-Cdn") == "Imperva" {
		return true
	}
	// Sometimes nginx is used as a proxy
	if resp.Header.Get("Server") == "nginx" && resp.StatusCode == 500 {
		return true
	}
	return false
}

/*
Name: doRequestWithRetry
Type: Internal Func
Purpose: Execute HTTP request with automatic retry on Imperva challenge
Note: For POST requests, the bodyBytes should be provided to recreate the request on retry
Returns api.ErrImperva if all retries fail due to Imperva challenge
*/
func (a *API) doRequestWithRetry(client *http.Client, req *http.Request, bodyBytes []byte, maxRetries int, venueID int64) (*http.Response, error) {
	// Store original headers for retry
	originalHeaders := make(map[string][]string)
	for key, values := range req.Header {
		originalHeaders[key] = values
	}
	originalURL := req.URL.String()
	originalMethod := req.Method

	var lastImpervaResponse bool

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// On retry, recreate the request with the body
		if attempt > 0 {
			fmt.Printf("Retrying request (attempt %d/%d) with updated cookies...\n", attempt+1, maxRetries+1)

			// Recreate request with body for POST requests
			if bodyBytes != nil {
				var err error
				req, err = http.NewRequest(originalMethod, originalURL, bytes.NewBuffer(bodyBytes))
				if err != nil {
					return nil, fmt.Errorf("failed to recreate request: %w", err)
				}

				// Copy headers from original request
				for key, values := range originalHeaders {
					for _, value := range values {
						req.Header.Add(key, value)
					}
				}
			}

			// Re-add cookies in case they were updated
			a.addCookiesToRequest(req)

			// Small delay before retry
			time.Sleep(1 * time.Second)
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}

		// Check if this is an Imperva challenge
		if isImpervaChallenge(resp) {
			fmt.Printf("Received Imperva challenge (status %d), extracting cookies and retrying...\n", resp.StatusCode)
			lastImpervaResponse = true

			// Extract cookies from response
			a.extractCookiesFromResponse(resp)

			// Retry if we haven't exceeded max retries
			if attempt < maxRetries {
				resp.Body.Close()
				continue
			} else {
				// Retries exhausted - return ErrImperva
				resp.Body.Close()
				fmt.Println("Retries exhausted, Imperva challenge not resolved. Please refresh cookies via /admin/cookies/import")
				return nil, api.ErrImperva
			}
		}

		lastImpervaResponse = false
		return resp, nil
	}

	if lastImpervaResponse {
		return nil, api.ErrImperva
	}
	return nil, fmt.Errorf("max retries exceeded")
}

/*
Name: LoadCookiesFromStore
Type: API Func
Purpose: Load cookies from Redis store for a venue
*/
func (a *API) LoadCookiesFromStore(venueID int64) error {
	ctx := context.Background()
	cookieData, err := store.GetCookies(ctx, venueID)
	if err != nil {
		return err
	}
	a.SetCookies(cookieData.Cookies, cookieData.UserAgent)
	fmt.Printf("Loaded %d cookies from store for venue %d\n", len(cookieData.Cookies), venueID)
	return nil
}

/*
Name: GetDefaultAPI
Type: External Func
Purpose: Function that provides an out of the box
working API struct
*/
func GetDefaultAPI() API {
	return API{
		APIKey: config.Get().ResyAPIKey,
	}
}

/*
Name: Login
Type: API Func
Purpose: Resy implementation of the Login api func
Note: The only required login fields for this func
are Email and Password.
*/
func (a *API) Login(params api.LoginParam) (*api.LoginResponse, error) {
	authUrl := "https://api.resy.com/3/auth/password"
	email := url.QueryEscape(params.Email)
	password := url.QueryEscape(params.Password)
	bodyStr := `email=` + email + `&password=` + password
	bodyBytes := []byte(bodyStr)

	request, err := http.NewRequest("POST", authUrl, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, err
	}

	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Authorization", `ResyAPI api_key="`+a.APIKey+`"`)

	// Add Imperva cookies and user agent
	a.addCookiesToRequest(request)

	client := &http.Client{}
	response, err := client.Do(request)

	if err != nil {
		return nil, err
	}

	// Resy servers return a 419 is the auth parameters were invalid
	if response.StatusCode == 419 {
		return nil, api.ErrLoginWrong
	}

	if isCodeFail(response.StatusCode) {
		return nil, api.ErrNetwork
	}

	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)

	if err != nil {
		return nil, err
	}

	var jsonMap map[string]interface{}
	err = json.Unmarshal(responseBody, &jsonMap)
	if err != nil {
		return nil, err
	}

	if jsonMap["payment_method_id"] == nil {
		return nil, api.ErrNoPayInfo
	}

	loginResponse := api.LoginResponse{
		ID:              int64(jsonMap["id"].(float64)),
		FirstName:       jsonMap["first_name"].(string),
		LastName:        jsonMap["last_name"].(string),
		Mobile:          jsonMap["mobile_number"].(string),
		Email:           jsonMap["em_address"].(string),
		PaymentMethodID: int64(jsonMap["payment_method_id"].(float64)),
		AuthToken:       jsonMap["token"].(string),
	}

	return &loginResponse, nil

}

/*
Name: Search
Type: API Func
Purpose: Resy implementation of the Search api func
*/
func (a *API) Search(params api.SearchParam) (*api.SearchResponse, error) {
	searchUrl := "https://api.resy.com/3/venuesearch/search"

	bodyStr := `{"query":"` + params.Name + `"}`
	bodyBytes := []byte(bodyStr)

	request, err := http.NewRequest("POST", searchUrl, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, err
	}

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", `ResyAPI api_key="`+a.APIKey+`"`)
	request.Header.Set("Origin", `https://resy.com`)
	request.Header.Set("Referer", `https://resy.com/`)

	// Add Imperva cookies and user agent
	a.addCookiesToRequest(request)

	client := &http.Client{}
	response, err := client.Do(request)

	if err != nil {
		return nil, err
	}

	defer response.Body.Close()

	if isCodeFail(response.StatusCode) {
		responseBody, _ := io.ReadAll(response.Body)
		fmt.Printf("Search request failed with status code: %d, body: %s\n", response.StatusCode, string(responseBody))
		return nil, api.ErrNetwork
	}

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	var jsonTopLevelMap map[string]interface{}
	err = json.Unmarshal(responseBody, &jsonTopLevelMap)
	if err != nil {
		fmt.Printf("Error unmarshaling search response: %v, body: %s\n", err, string(responseBody))
		return nil, err
	}

	// Check if "search" key exists
	searchValue, ok := jsonTopLevelMap["search"]
	if !ok {
		fmt.Printf("Search response missing 'search' key. Response: %s\n", string(responseBody))
		return nil, api.ErrNetwork
	}

	jsonSearchMap, ok := searchValue.(map[string]interface{})
	if !ok {
		fmt.Printf("Search response 'search' is not a map. Response: %s\n", string(responseBody))
		return nil, api.ErrNetwork
	}

	// Check if "hits" key exists
	hitsValue, ok := jsonSearchMap["hits"]
	if !ok {
		fmt.Printf("Search response missing 'hits' key. Response: %s\n", string(responseBody))
		return nil, api.ErrNetwork
	}

	jsonHitsMap, ok := hitsValue.([]interface{})
	if !ok {
		fmt.Printf("Search response 'hits' is not an array. Response: %s\n", string(responseBody))
		return nil, api.ErrNetwork
	}

	numHits := len(jsonHitsMap)

	// if input param limit is nonnegative, limit the search loop
	var limit int
	if params.Limit > 0 {
		limit = min(params.Limit, numHits)
	} else {
		limit = numHits
	}

	searchResults := make([]api.SearchResult, 0, limit)
	for i := 0; i < limit; i++ {
		jsonHitMap, ok := jsonHitsMap[i].(map[string]interface{})
		if !ok {
			fmt.Printf("Hit %d is not a map, skipping\n", i)
			continue
		}

		// Safely extract fields with nil checks
		objectID, ok := jsonHitMap["objectID"].(string)
		if !ok {
			fmt.Printf("Hit %d missing or invalid objectID, skipping\n", i)
			continue
		}

		venueID, err := strconv.ParseInt(objectID, 10, 64)
		if err != nil {
			fmt.Printf("Error parsing venueID %s: %v, skipping\n", objectID, err)
			continue
		}

		name, _ := jsonHitMap["name"].(string)
		region, _ := jsonHitMap["region"].(string)
		locality, _ := jsonHitMap["locality"].(string)
		neighborhood, _ := jsonHitMap["neighborhood"].(string)

		searchResults = append(searchResults, api.SearchResult{
			VenueID:      venueID,
			Name:         name,
			Region:       region,
			Locality:     locality,
			Neighborhood: neighborhood,
		})
	}

	searchResponse := api.SearchResponse{
		Results: searchResults,
	}

	return &searchResponse, nil
}

/*
Name: Reserve
Type: API Func
Purpose: Resy implementation of the Reserve api func
*/
func (a *API) Reserve(params api.ReserveParam) (*api.ReserveResponse, error) {
	fmt.Println("Starting Reserve function")
	defer fmt.Println("Exiting Reserve function")

	// Try to load cookies from Redis store for this venue
	if err := a.LoadCookiesFromStore(params.VenueID); err != nil {
		fmt.Printf("Warning: Could not load cookies from store for venue %d: %v\n", params.VenueID, err)
		// Continue anyway - cookies might have been set manually or we'll get Imperva error
	}

	// Converting fields to URL query format
	// IMPORTANT: Convert to NYC timezone before extracting date components
	// The reservation time is stored in UTC, but Resy expects the date in NYC timezone
	fmt.Println("Converting reservation times to date string")
	nycLocation, err := time.LoadLocation("America/New_York")
	if err != nil {
		fmt.Printf("Error loading NYC timezone: %v, using UTC\n", err)
		nycLocation = time.UTC
	}
	reservationTimeNYC := params.ReservationTimes[0].In(nycLocation)
	fmt.Printf("Reservation time in NYC: %s\n", reservationTimeNYC.Format("2006-01-02 15:04:05 MST"))

	year := strconv.Itoa(reservationTimeNYC.Year())
	monthInt := int(reservationTimeNYC.Month())
	dayInt := reservationTimeNYC.Day()

	// Zero-pad month and day
	month := fmt.Sprintf("%02d", monthInt)
	day := fmt.Sprintf("%02d", dayInt)

	date := year + "-" + month + "-" + day
	fmt.Printf("Formatted date: %s\n", date)
	fmt.Printf("Using venue_id: %d\n", params.VenueID)

	// Use JSON body for find request (Resy API expects application/json)
	requestBody := map[string]interface{}{
		"day":        date,
		"venue_id":   params.VenueID,
		"party_size": params.PartySize,
		"lat":        0,
		"long":       0,
	}
	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		fmt.Printf("Error marshaling find request body: %v\n", err)
		return nil, err
	}
	fmt.Printf("Find request body: %s\n", string(bodyBytes))

	findUrl := "https://api.resy.com/4/find"
	fmt.Printf("Find URL: %s\n", findUrl)

	request, err := http.NewRequest("POST", findUrl, bytes.NewBuffer(bodyBytes))
	if err != nil {
		fmt.Printf("Error creating find request: %v\n", err)
		return nil, err
	}

	// Setting headers - Important: User-Agent needed to bypass Imperva WAF
	fmt.Println("Setting headers for find request")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", `ResyAPI api_key="`+a.APIKey+`"`)
	request.Header.Set("X-Resy-Auth-Token", params.LoginResp.AuthToken)
	request.Header.Set("X-Resy-Universal-Auth-Token", params.LoginResp.AuthToken)
	request.Header.Set("Referer", "https://resy.com/")
	request.Header.Set("Origin", "https://resy.com")

	// Add Imperva cookies and user agent (will override default User-Agent if set)
	a.addCookiesToRequest(request)

	// Fallback to default User-Agent if not set via cookies
	if a.UserAgent == "" {
		request.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	}

	// POST Variations (uncomment to try if GET fails):
	//
	// Option A: POST with auth token in body (form-encoded)
	// bodyStr := fmt.Sprintf("day=%s&venue_id=%d&party_size=%d&x-resy-auth-token=%s",
	//     url.QueryEscape(date), params.VenueID, params.PartySize, url.QueryEscape(params.LoginResp.AuthToken))
	// request, err = http.NewRequest("POST", "https://api.resy.com/4/find", bytes.NewBuffer([]byte(bodyStr)))
	// request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	//
	// Option B: POST with JSON body
	// requestBody := map[string]interface{}{
	//     "day": date,
	//     "venue_id": params.VenueID,
	//     "party_size": params.PartySize,
	//     "x-resy-auth-token": params.LoginResp.AuthToken,
	// }
	// jsonBody, _ := json.Marshal(requestBody)
	// request, err = http.NewRequest("POST", "https://api.resy.com/4/find", bytes.NewBuffer(jsonBody))
	// request.Header.Set("Content-Type", "application/json")
	//
	// Option C: Add User-Agent header (as book endpoint uses)
	// request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	//
	// Option D: Try X-Resy-Universal-Auth instead of X-Resy-Universal-Auth-Token (as book endpoint uses)
	// request.Header.Set("X-Resy-Universal-Auth", params.LoginResp.AuthToken)
	// Remove or comment out: request.Header.Set("X-Resy-Universal-Auth-Token", ...)

	// Enhanced debugging: Print all request details
	fmt.Println("=== REQUEST DEBUG INFO ===")
	fmt.Printf("Method: %s\n", request.Method)
	fmt.Printf("URL: %s\n", request.URL.String())
	fmt.Println("Headers:")
	for key, values := range request.Header {
		for _, value := range values {
			// Mask auth token in logs for security
			if strings.Contains(key, "Auth") {
				fmt.Printf("  %s: %s\n", key, "***REDACTED***")
			} else {
				fmt.Printf("  %s: %s\n", key, value)
			}
		}
	}
	fmt.Println("==========================")

	client := &http.Client{}
	fmt.Println("Sending find request")

	// Use retry logic for Imperva challenges (pass bodyBytes to recreate request on retry, and venueID for fallback)
	response, err := a.doRequestWithRetry(client, request, bodyBytes, 2, params.VenueID)
	if err != nil {
		fmt.Printf("Error sending find request: %v\n", err)
		return nil, err
	}
	fmt.Printf("Received find response with status code: %d\n", response.StatusCode)

	// Enhanced debugging: Print response headers
	fmt.Println("=== RESPONSE DEBUG INFO ===")
	fmt.Printf("Status Code: %d\n", response.StatusCode)
	fmt.Println("Response Headers:")
	for key, values := range response.Header {
		for _, value := range values {
			fmt.Printf("  %s: %s\n", key, value)
		}
	}
	fmt.Println("===========================")

	defer response.Body.Close()

	// Always read the response body, even on error, to see what the API says
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		fmt.Printf("Error reading find response body: %v\n", err)
		return nil, err
	}
	fmt.Printf("Find response body: %s\n", string(responseBody))

	if isCodeFail(response.StatusCode) {
		fmt.Printf("Find request failed with status code: %d\n", response.StatusCode)
		fmt.Printf("Error details: %s\n", string(responseBody))

		// Enhanced error parsing: Try to extract detailed error information
		errorMsg := string(responseBody)
		var errorMap map[string]interface{}
		if json.Unmarshal(responseBody, &errorMap) == nil {
			fmt.Println("=== PARSED ERROR DETAILS ===")
			for key, value := range errorMap {
				fmt.Printf("  %s: %v\n", key, value)
			}
			fmt.Println("============================")

			if message, ok := errorMap["message"].(string); ok {
				fmt.Printf("API error message: %s\n", message)
				errorMsg = message
			}
			if errorType, ok := errorMap["type"].(string); ok {
				fmt.Printf("API error type: %s\n", errorType)
			}
			if errors, ok := errorMap["errors"].(map[string]interface{}); ok {
				fmt.Printf("API errors object: %v\n", errors)
			}
		} else {
			// If not JSON, print raw response
			fmt.Printf("Response is not JSON, raw content: %s\n", string(responseBody))
		}

		return nil, api.NewNetworkError("find", response.StatusCode, errorMsg)
	}

	var jsonTopLevelMap map[string]interface{}
	err = json.Unmarshal(responseBody, &jsonTopLevelMap)
	if err != nil {
		fmt.Printf("Error unmarshaling find response JSON: %v\n", err)
		return nil, err
	}

	// Navigate JSON structure
	fmt.Println("Parsing JSON response for venues and slots")
	jsonResultsMap, ok := jsonTopLevelMap["results"].(map[string]interface{})
	if !ok {
		fmt.Println("Error: 'results' key not found or invalid in JSON response")
		return nil, api.NewNetworkError("find", 0, "invalid response: 'results' key not found")
	}

	jsonVenuesList, ok := jsonResultsMap["venues"].([]interface{})
	if !ok {
		fmt.Println("Error: 'venues' key not found or invalid in JSON response")
		return nil, api.NewNetworkError("find", 0, "invalid response: 'venues' key not found")
	}

	if len(jsonVenuesList) == 0 {
		fmt.Println("No venues found in the response")
		return nil, api.ErrNoOffer
	}

	// Find the venue that matches the requested venue ID
	var jsonVenueMap map[string]interface{}
	for i, v := range jsonVenuesList {
		venue, ok := v.(map[string]interface{})
		if !ok {
			fmt.Printf("Skipping invalid venue structure at index %d\n", i)
			continue
		}

		// Try to extract venue ID from the response structure
		// Resy API returns venue info nested under "venue" key
		if venueInfo, ok := venue["venue"].(map[string]interface{}); ok {
			if idInfo, ok := venueInfo["id"].(map[string]interface{}); ok {
				if resyID, ok := idInfo["resy"].(float64); ok {
					fmt.Printf("Found venue at index %d with ID %d\n", i, int64(resyID))
					if int64(resyID) == params.VenueID {
						fmt.Printf("Matched requested venue ID %d\n", params.VenueID)
						jsonVenueMap = venue
						break
					}
				}
			}
		}
	}

	// If no matching venue found, log warning and fall back to first venue
	if jsonVenueMap == nil {
		fmt.Printf("Warning: Could not find venue matching ID %d in response, using first venue\n", params.VenueID)
		var ok bool
		jsonVenueMap, ok = jsonVenuesList[0].(map[string]interface{})
		if !ok {
			fmt.Println("Error: Invalid venue structure in JSON response")
			return nil, api.NewNetworkError("find", 0, "invalid response: venue structure is invalid")
		}
	}

	jsonSlotsList, ok := jsonVenueMap["slots"].([]interface{})
	if !ok {
		fmt.Println("Error: 'slots' key not found or invalid in venue JSON")
		return nil, api.NewNetworkError("find", 0, "invalid response: 'slots' key not found in venue")
	}

	fmt.Printf("Number of slots available: %d\n", len(jsonSlotsList))

	// Iterate over table types and reservation times
	// If no table types specified, match any slot based on time only
	hasTableTypePreference := len(params.TableTypes) > 0

	for k := 0; k < len(params.TableTypes) || (!hasTableTypePreference && k == 0); k++ {
		var currentTableType api.TableType
		if hasTableTypePreference {
			currentTableType = params.TableTypes[k]
			fmt.Printf("Searching for table type: %s\n", currentTableType)
		} else {
			fmt.Printf("No table type preference provided. Matching any slot based on time only.\n")
		}

		for i := 0; i < len(params.ReservationTimes); i++ {
			currentTime := params.ReservationTimes[i]
			fmt.Printf("Checking reservation time: %s\n", currentTime.Format("2006-01-02 15:04:00"))

			// First pass: Try to find exact match, then closest match within window
			var bestSlot map[string]interface{}
			var bestSlotIndex int = -1
			var bestSlotTime time.Time
			var bestSlotConfigToken string
			var bestTimeDiff time.Duration = 31 * time.Minute // Track smallest time difference found (start larger than max)
			const maxTimeDiff = 30 * time.Minute              // Maximum allowed time difference
			foundExactMatch := false

			fmt.Printf("Starting slot search for time %s (total slots: %d)\n", currentTime.Format("15:04"), len(jsonSlotsList))

			for j := 0; j < len(jsonSlotsList); j++ {
				fmt.Printf("Evaluating slot %d\n", j)
				jsonSlotMap, ok := jsonSlotsList[j].(map[string]interface{})
				if !ok {
					fmt.Printf("Error: Invalid slot structure at index %d\n", j)
					continue
				}

				jsonDateMap, ok := jsonSlotMap["date"].(map[string]interface{})
				if !ok {
					fmt.Printf("Error: 'date' key missing or invalid in slot %d\n", j)
					continue
				}

				startRaw, ok := jsonDateMap["start"].(string)
				if !ok {
					fmt.Printf("Error: 'start' key missing or invalid in slot %d\n", j)
					continue
				}
				fmt.Printf("Slot start time: %s\n", startRaw)

				startFields := strings.Split(startRaw, " ")
				if len(startFields) != 2 {
					fmt.Printf("Error: Unexpected 'start' format in slot %d\n", j)
					continue
				}

				dateStr := startFields[0]
				timeFields := strings.Split(startFields[1], ":")
				if len(timeFields) != 3 {
					fmt.Printf("Error: Unexpected time format in slot %d\n", j)
					continue
				}

				// Parse the slot's full date/time
				// NOTE: Resy API returns times in the venue's local timezone (NYC), not UTC
				// We need to parse it as NYC time and compare with the requested time in NYC
				dateTimeStr := dateStr + " " + timeFields[0] + ":" + timeFields[1] + ":00"
				slotTime, err := time.ParseInLocation("2006-01-02 15:04:05", dateTimeStr, nycLocation)
				if err != nil {
					fmt.Printf("Error parsing slot time: %v\n", err)
					continue
				}
				fmt.Printf("Parsed slot time (NYC): %s\n", slotTime.Format("2006-01-02 15:04:05 MST"))

				// Convert currentTime to NYC for comparison
				currentTimeNYC := currentTime.In(nycLocation)

				// Check if the slot is on the same date as the requested time (in NYC timezone)
				slotDateStr := slotTime.Format("2006-01-02")
				currentDateStr := currentTimeNYC.Format("2006-01-02")
				if slotTime.Year() != currentTimeNYC.Year() ||
					slotTime.Month() != currentTimeNYC.Month() ||
					slotTime.Day() != currentTimeNYC.Day() {
					fmt.Printf("Slot %d date %s doesn't match requested date %s, skipping\n",
						j, slotDateStr, currentDateStr)
					continue
				}
				fmt.Printf("Slot %d date matches: %s\n", j, slotDateStr)

				// Check if the slot matches the desired time (exact match) using NYC times
				timeMatches := slotTime.Hour() == currentTimeNYC.Hour() && slotTime.Minute() == currentTimeNYC.Minute()

				// Get config map to check table type
				jsonConfigMap, ok := jsonSlotMap["config"].(map[string]interface{})
				if !ok {
					fmt.Printf("Error: 'config' key missing or invalid in slot %d\n", j)
					continue
				}

				// Check table type if preference is specified
				if hasTableTypePreference {
					tableType, ok := jsonConfigMap["type"].(string)
					if !ok {
						fmt.Printf("Error: 'type' key missing or invalid in config of slot %d\n", j)
						continue
					}
					fmt.Printf("Slot %d table type: %s\n", j, tableType)

					if !strings.Contains(strings.ToLower(tableType), string(currentTableType)) {
						fmt.Printf("Slot %d table type '%s' doesn't match preference '%s', skipping\n", j, tableType, currentTableType)
						continue
					}
				} else {
					// Just log the table type for debugging
					if tableType, ok := jsonConfigMap["type"].(string); ok {
						fmt.Printf("Slot %d table type: %s (no preference, accepting any)\n", j, tableType)
					}
				}

				// If exact time match, use it immediately
				if timeMatches {
					fmt.Printf("Found exact match at slot %d for time %s\n", j, currentTimeNYC.Format("15:04"))
					bestSlot = jsonSlotMap
					bestSlotIndex = j
					bestSlotTime = slotTime
					configToken, ok := jsonConfigMap["token"].(string)
					if ok {
						bestSlotConfigToken = configToken
					}
					foundExactMatch = true
					break
				}

				// If no exact match yet, track the closest slot within the time window
				// Compare using NYC times since slots are in NYC timezone
				if !foundExactMatch {
					timeDiff := slotTime.Sub(currentTimeNYC)
					absTimeDiff := timeDiff
					if absTimeDiff < 0 {
						absTimeDiff = -absTimeDiff // Use absolute value
					}
					fmt.Printf("Slot %d time difference from requested: %v (absolute: %v)\n", j, timeDiff, absTimeDiff)

					// Only consider slots within the max time window and that are better than current best
					if absTimeDiff <= maxTimeDiff && absTimeDiff < bestTimeDiff {
						bestTimeDiff = absTimeDiff
						bestSlot = jsonSlotMap
						bestSlotIndex = j
						bestSlotTime = slotTime
						configToken, ok := jsonConfigMap["token"].(string)
						if ok {
							bestSlotConfigToken = configToken
						}
						fmt.Printf("Found closer slot at index %d (time difference: %v, slot time: %s)\n",
							j, absTimeDiff, slotTime.Format("15:04"))
					}
				}
			}

			// Summary of slot search
			fmt.Printf("Slot search complete. Found %d slots total.\n", len(jsonSlotsList))
			currentTimeNYC := currentTime.In(nycLocation)
			if bestSlotIndex >= 0 {
				if foundExactMatch {
					fmt.Printf("✓ Using exact match at slot %d for time %s NYC\n", bestSlotIndex, currentTimeNYC.Format("15:04"))
				} else {
					fmt.Printf("✓ No exact match found. Using closest available slot at %s (requested: %s NYC, difference: %v)\n",
						bestSlotTime.Format("15:04"), currentTimeNYC.Format("15:04"), bestTimeDiff)
				}
			} else {
				fmt.Printf("✗ No suitable slot found within %v of requested time %s NYC\n", maxTimeDiff, currentTimeNYC.Format("15:04"))
			}

			// If we found a slot (exact or closest), proceed with booking
			if bestSlotIndex >= 0 {

				configToken := bestSlotConfigToken
				if configToken == "" {
					jsonConfigMap, ok := bestSlot["config"].(map[string]interface{})
					if !ok {
						fmt.Printf("Error: 'config' key missing in best slot\n")
						continue
					}
					configToken, ok = jsonConfigMap["token"].(string)
					if !ok {
						fmt.Printf("Error: 'token' key missing in best slot config\n")
						continue
					}
				}

				detailUrl := "https://api.resy.com/3/details"
				fmt.Printf("Detail URL: %s\n", detailUrl)

				// Prepare the request body
				requestBody := map[string]string{
					"commit":     strconv.Itoa(1),                // Convert integer 1 to string
					"config_id":  configToken,                    // Assuming configToken is already a string
					"day":        date,                           // Assuming date is already a string
					"party_size": strconv.Itoa(params.PartySize), // Convert PartySize (an int) to string
				}
				jsonBody, err := json.Marshal(requestBody)

				if err != nil {
					fmt.Printf("Error marshaling request body: %v\n", err)
					continue
				}
				fmt.Printf("Request Body: %s\n", string(jsonBody)) // Add this line

				requestDetail, err := http.NewRequest("POST", detailUrl, bytes.NewBuffer(jsonBody))
				if err != nil {
					fmt.Printf("Error creating detail request: %v\n", err)
					continue
				}

				// Setting headers for detail request
				// Set the appropriate headers
				requestDetail.Header.Set("Content-Type", "application/json")
				requestDetail.Header.Set("Authorization", "ResyAPI api_key=\"VbWk7s3L4KiK5fzlO7JD3Q5EYolJI7n5\"")

				// Add Imperva cookies and user agent
				a.addCookiesToRequest(requestDetail)

				// Fallback to default User-Agent if not set via cookies
				if a.UserAgent == "" {
					requestDetail.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
				}
				// Log the request headers
				fmt.Println("Request Headers:")
				for key, value := range requestDetail.Header {
					fmt.Printf("%s: %s\n", key, strings.Join(value, ", "))
				}

				fmt.Println("Sending detail request")
				responseDetail, err := client.Do(requestDetail)
				print(responseDetail)
				if err != nil {
					fmt.Printf("Error sending detail request: %v\n", err)
					continue
				}
				fmt.Printf("Received detail response with status code: %d\n", responseDetail.StatusCode)

				if isCodeFail(responseDetail.StatusCode) {
					responseDetailBody, err := io.ReadAll(responseDetail.Body)
					if err != nil {
						fmt.Printf("Error reading detail response body: %v\n", err)
						continue
					}
					fmt.Printf("Detail response body: %s\n", string(responseDetailBody))
					fmt.Printf("Detail request failed with status code: %d\n", responseDetail.StatusCode)
					return nil, api.NewNetworkError("detail", responseDetail.StatusCode, string(responseDetailBody))
				}

				defer responseDetail.Body.Close()

				responseDetailBody, err := io.ReadAll(responseDetail.Body)
				fmt.Printf("Detail response body: %s\n", string(responseDetailBody))
				if err != nil {
					fmt.Printf("Error reading detail response body: %v\n", err)
					continue
				}
				fmt.Printf("Detail response body: %s\n", string(responseDetailBody))

				var detailTopLevelMap map[string]interface{}
				err = json.Unmarshal(responseDetailBody, &detailTopLevelMap)
				if err != nil {
					fmt.Printf("Error unmarshaling detail response JSON: %v\n", err)
					return nil, err
				}

				jsonBookTokenMap, ok := detailTopLevelMap["book_token"].(map[string]interface{})
				if !ok {
					fmt.Println("Error: 'book_token' key missing or invalid in detail JSON")
					continue
				}

				bookToken, ok := jsonBookTokenMap["value"].(string)
				if !ok {
					fmt.Println("Error: 'value' key missing or invalid in 'book_token'")
					continue
				}
				fmt.Printf("Obtained book token: %s\n", bookToken)

				// Proceed to booking step
				bookUrl := "https://api.resy.com/3/book"
				fmt.Printf("Book URL: %s\n", bookUrl)

				bookField := "book_token=" + url.QueryEscape(bookToken)
				paymentMethodStr := `{"id":` + strconv.FormatInt(params.LoginResp.PaymentMethodID, 10) + `}`
				paymentMethodField := "struct_payment_method=" + url.QueryEscape(paymentMethodStr)
				requestBookBodyStr := bookField + "&" + paymentMethodField + "&" + "source_id=resy.com-venue-details"
				fmt.Printf("Book request body: %s\n", requestBookBodyStr)

				requestBook, err := http.NewRequest("POST", bookUrl, bytes.NewBuffer([]byte(requestBookBodyStr)))
				if err != nil {
					fmt.Printf("Error creating book request: %v\n", err)
					continue
				}

				// Setting headers for book request
				fmt.Println("Setting headers for book request")
				requestBook.Header.Set("Authorization", `ResyAPI api_key="`+a.APIKey+`"`)
				requestBook.Header.Set("Content-Type", `application/x-www-form-urlencoded`)
				requestBook.Header.Set("Host", `api.resy.com`)
				requestBook.Header.Set("X-Resy-Auth-Token", params.LoginResp.AuthToken)
				requestBook.Header.Set("X-Resy-Universal-Auth", params.LoginResp.AuthToken)
				requestBook.Header.Set("Referer", "https://resy.com/")

				// Add Imperva cookies and user agent
				a.addCookiesToRequest(requestBook)

				// Fallback to default User-Agent if not set via cookies
				if a.UserAgent == "" {
					requestBook.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
				}

				fmt.Println("Sending book request")
				responseBook, err := client.Do(requestBook)
				if err != nil {
					fmt.Printf("Error sending book request: %v\n", err)
					continue
				}
				fmt.Printf("Received book response with status code: %d\n", responseBook.StatusCode)

				if isCodeFail(responseBook.StatusCode) {
					fmt.Printf("Book request failed with status code: %d\n", responseBook.StatusCode)
					continue
				}

				responseBookBody, err := io.ReadAll(responseBook.Body)
				if err != nil {
					fmt.Printf("Error reading book response body: %v\n", err)
					continue
				}
				fmt.Printf("Book response body: %s\n", string(responseBookBody))

				var bookTopLevelMap map[string]interface{}
				err = json.Unmarshal(responseBookBody, &bookTopLevelMap)
				if err != nil {
					fmt.Printf("Error unmarshaling book response JSON: %v\n", err)
					continue
				}

				// Check if booking was successful
				if _, ok := bookTopLevelMap["reservation_id"]; ok {
					fmt.Println("Booking confirmed successfully")
					resp := api.ReserveResponse{
						ReservationTime: bestSlotTime,
					}
					return &resp, nil
				} else {
					fmt.Println("Booking response does not contain confirmation")
					fmt.Printf("Book response JSON: %v\n", bookTopLevelMap)
					// If booking failed with 402, it might be a payment issue
					// Try to continue to next slot if available
					if responseBook.StatusCode == 402 {
						fmt.Printf("Payment error (402) for slot at %s, will try next available slot if any\n", bestSlotTime.Format("15:04"))
					}
					continue
				}
			} else {
				// No slot found within the time window
				fmt.Printf("No available slot found within 30 minutes of requested time %s\n", currentTime.Format("15:04"))
			}
		}
	}

	// If no table was found after all iterations
	fmt.Println("No available tables found for the given parameters")
	return nil, api.ErrNoTable
}

/*
Name: AuthMinExpire
Type: API Func
Purpose: Resy implementation of the AuthMinExpire api func.
The largest minimum validity time is 6 days.
*/
func (a *API) AuthMinExpire() time.Duration {
	/* 6 days */
	var d time.Duration = time.Hour * 24 * 6
	return d
}

//func (a *API) Cancel(params api.CancelParam) (*api.CancelResponse, error) {
//    cancelUrl := `https://api.resy.com/3/cancel`
//    resyToken := url.QueryEscape(params.ResyToken)
//    requestBodyStr := "resy_token=" + resyToken
//    request, err := http.NewRequest("POST", cancelUrl, bytes.NewBuffer([]byte(requestBodyStr)))
//    if err != nil {
//        return nil, err
//    }
//
//    request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
//    request.Header.Set("Authorization", `ResyAPI api_key="` + a.APIKey + `"`)
//    request.Header.Set("X-Resy-Auth-Token", params.AuthToken)
//    request.Header.Set("X-Resy-Universal-Auth-Token", params.AuthToken)
//    request.Header.Set("Referer", "https://resy.com/")
//    request.Header.Set("Origin", "https://resy.com")
//
//
//    client := &http.Client{}
//    response, err := client.Do(request)
//    if err != nil {
//        return nil, err
//    }
//
//    if isCodeFail(response.StatusCode) {
//        return nil, api.ErrNetwork
//    }
//
//    responseBody, err := io.ReadAll(response.Body)
//    if err != nil {
//        return nil, err
//    }
//
//    defer response.Body.Close()
//    var jsonTopLevelMap map[string]interface{}
//    err = json.Unmarshal(responseBody, &jsonTopLevelMap)
//    if err != nil {
//        return nil, err
//    }
//
//    jsonPaymentMap := jsonTopLevelMap["payment"].(map[string]interface{})
//    jsonTransactionMap := jsonPaymentMap["transaction"].(map[string]interface{})
//    refund := jsonTransactionMap["refund"].(int) == 1
//    return &api.CancelResponse{Refund: refund}, nil
//}
//
