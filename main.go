// main.go
package main

import (
    "encoding/json"
    "html/template"
    "log"
    "net/http"
    "strconv"
    "strings"
    "time"

    "github.com/21Bruce/resolved-server/api"
    "github.com/21Bruce/resolved-server/api/resy"
    "github.com/21Bruce/resolved-server/app"
    "github.com/gorilla/securecookie"
)

type TemplateData struct {
    Message        string
    RestaurantName string
    SearchResults  []api.SearchResult
}

// Structures for JSON responses
type SearchResponse struct {
    Results []api.SearchResult `json:"results"`
    Error   string             `json:"error,omitempty"`
}

type LoginRequest struct {
    Email    string `json:"email"`
    Password string `json:"password"`
}

type LoginResponse struct {
    AuthToken string `json:"auth_token,omitempty"`
    VenueID   int64  `json:"venue_id,omitempty"`
    Error     string `json:"error,omitempty"`
}

type ReserveRequest struct {
    VenueID          int64    `json:"venue_id"`
    ReservationTime  string   `json:"reservation_time"`  // in UTC: YYYY:MM:DD:HH:MM
    PartySize        int      `json:"party_size"`
    TablePreferences []string `json:"table_preferences"`
    IsImmediate      bool     `json:"is_immediate"`
    RequestTime      string   `json:"request_time"` // in UTC: YYYY:MM:DD:HH:MM
}

type ReserveResponse struct {
    ReservationTime string `json:"reservation_time,omitempty"`
    Error           string `json:"error,omitempty"`
}

type SelectVenueRequest struct {
    VenueID int64 `json:"venue_id"`
}

type SelectVenueResponse struct {
    Message string `json:"message,omitempty"`
    Error   string `json:"error,omitempty"`
}

var s = securecookie.New(securecookie.GenerateRandomKey(32), securecookie.GenerateRandomKey(32))

type ScheduledReservation struct {
    ReserveParam api.ReserveParam
    RunTime      time.Time // UTC time at which we attempt the reservation
}

// Store scheduled reservations in memory
var scheduledReservations []ScheduledReservation

// In-memory log lines
var logLines []string

func main() {
    resyAPI := resy.GetDefaultAPI()
    appCtx := app.AppCtx{API: &resyAPI}

    tmpl := template.Must(template.ParseFiles("index.html", "login.html", "reserve.html"))

    http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

    // Search API endpoint
    http.HandleFunc("/api/search", func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost {
            http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
            return
        }

        var searchRequest struct {
            Name  string `json:"name"`
            Limit int    `json:"limit"`
        }

        if err := json.NewDecoder(r.Body).Decode(&searchRequest); err != nil {
            sendJSONResponse(w, SearchResponse{Error: "Invalid request format"}, http.StatusBadRequest)
            return
        }

        searchParam := api.SearchParam{
            Name:  searchRequest.Name,
            Limit: searchRequest.Limit,
        }

        results, err := appCtx.API.Search(searchParam)
        if err != nil {
            sendJSONResponse(w, SearchResponse{Error: err.Error()}, http.StatusInternalServerError)
            return
        }

        sendJSONResponse(w, SearchResponse{Results: results.Results}, http.StatusOK)
    })

    // Select Venue API endpoint
    http.HandleFunc("/api/select-venue", func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost {
            http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
            return
        }

        var selectReq SelectVenueRequest
        if err := json.NewDecoder(r.Body).Decode(&selectReq); err != nil {
            sendJSONResponse(w, SelectVenueResponse{Error: "Invalid request format"}, http.StatusBadRequest)
            return
        }

        session, err := getSession(r)
        if err != nil {
            session = make(map[string]string)
        }

        session["venue_id"] = strconv.FormatInt(selectReq.VenueID, 10)

        encoded, err := s.Encode("session", session)
        if err != nil {
            sendJSONResponse(w, SelectVenueResponse{Error: "Failed to encode session"}, http.StatusInternalServerError)
            return
        }

        cookie := &http.Cookie{
            Name:     "session",
            Value:    encoded,
            Path:     "/",
            HttpOnly: true,
            Secure:   true,
        }
        http.SetCookie(w, cookie)

        sendJSONResponse(w, SelectVenueResponse{Message: "Venue selected successfully"}, http.StatusOK)
    })

    // Login API endpoint
    http.HandleFunc("/api/login", func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost {
            http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
            return
        }

        var loginReq LoginRequest
        if err := json.NewDecoder(r.Body).Decode(&loginReq); err != nil {
            sendJSONResponse(w, LoginResponse{Error: "Invalid request format"}, http.StatusBadRequest)
            return
        }

        venueIDStr, err := getCookieValue(r, "venue_id")
        if err != nil {
            sendJSONResponse(w, LoginResponse{Error: "Venue ID not found. Please select a restaurant first."}, http.StatusBadRequest)
            return
        }
        venueID, err := strconv.ParseInt(venueIDStr, 10, 64)
        if err != nil {
            sendJSONResponse(w, LoginResponse{Error: "Invalid Venue ID"}, http.StatusBadRequest)
            return
        }

        loginParam := api.LoginParam{
            Email:    loginReq.Email,
            Password: loginReq.Password,
        }

        loginResp, err := appCtx.API.Login(loginParam)
        if err != nil {
            switch err {
            case api.ErrLoginWrong:
                sendJSONResponse(w, LoginResponse{Error: "Incorrect email or password"}, http.StatusUnauthorized)
            case api.ErrNetwork:
                sendJSONResponse(w, LoginResponse{Error: "Network error. Please try again later."}, http.StatusInternalServerError)
            case api.ErrNoPayInfo:
                sendJSONResponse(w, LoginResponse{Error: "No payment information found. Please update your account."}, http.StatusBadRequest)
            default:
                sendJSONResponse(w, LoginResponse{Error: "An unexpected error occurred."}, http.StatusInternalServerError)
            }
            return
        }

        value := map[string]string{
            "auth_token": loginResp.AuthToken,
            "venue_id":   strconv.FormatInt(venueID, 10),
        }
        encoded, err := s.Encode("session", value)
        if err != nil {
            sendJSONResponse(w, LoginResponse{Error: "Failed to set session"}, http.StatusInternalServerError)
            return
        }

        cookie := &http.Cookie{
            Name:     "session",
            Value:    encoded,
            Path:     "/",
            HttpOnly: true,
            Secure:   true,
        }
        http.SetCookie(w, cookie)

        sendJSONResponse(w, LoginResponse{
            AuthToken: loginResp.AuthToken,
            VenueID:   venueID,
        }, http.StatusOK)
    })

    // Reserve API endpoint
    http.HandleFunc("/api/reserve", func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost {
            http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
            return
        }

        var reserveReq ReserveRequest
        if err := json.NewDecoder(r.Body).Decode(&reserveReq); err != nil {
            sendJSONResponse(w, ReserveResponse{Error: "Invalid request format"}, http.StatusBadRequest)
            return
        }

        session, err := getSession(r)
        if err != nil {
            sendJSONResponse(w, ReserveResponse{Error: "Unauthorized. Please log in."}, http.StatusUnauthorized)
            return
        }

        authToken, ok := session["auth_token"]
        if !ok || authToken == "" {
            sendJSONResponse(w, ReserveResponse{Error: "Authentication token missing. Please log in."}, http.StatusUnauthorized)
            return
        }

        venueID := reserveReq.VenueID
        if venueID == 0 {
            venueIDStr, ok := session["venue_id"]
            if !ok || venueIDStr == "" {
                sendJSONResponse(w, ReserveResponse{Error: "Venue ID missing. Please select a restaurant first."}, http.StatusBadRequest)
                return
            }
            venueID, err = strconv.ParseInt(venueIDStr, 10, 64)
            if err != nil {
                sendJSONResponse(w, ReserveResponse{Error: "Invalid Venue ID"}, http.StatusBadRequest)
                return
            }
        }

        // Parse the reservation time (UTC)
        reservationTime, err := parseTime(reserveReq.ReservationTime)
        if err != nil {
            sendJSONResponse(w, ReserveResponse{Error: "Invalid reservation time format"}, http.StatusBadRequest)
            return
        }

        var requestTime time.Time
        if !reserveReq.IsImmediate {
            requestTime, err = parseTime(reserveReq.RequestTime)
            if err != nil {
                sendJSONResponse(w, ReserveResponse{Error: "Invalid request time: " + err.Error()}, http.StatusBadRequest)
                return
            }
        }

        // Convert table preferences
        var tableTypes []api.TableType
        for _, pref := range reserveReq.TablePreferences {
            tableTypes = append(tableTypes, api.TableType(pref))
        }

        reserveParam := api.ReserveParam{
            VenueID:          venueID,
            ReservationTimes: []time.Time{reservationTime},
            PartySize:        reserveReq.PartySize,
            LoginResp:        api.LoginResponse{AuthToken: authToken},
            TableTypes:       tableTypes,
        }

        if reserveReq.IsImmediate {
            // Attempt reservation now
            appendLog("Attempting immediate reservation")
            reserveResp, err := appCtx.API.Reserve(reserveParam)
            if err != nil {
                appendLog("Immediate reservation failed: " + err.Error())
                switch err {
                case api.ErrNetwork:
                    sendJSONResponse(w, ReserveResponse{Error: "Network error. Please try again later."}, http.StatusInternalServerError)
                case api.ErrNoTable:
                    sendJSONResponse(w, ReserveResponse{Error: "No available tables found for the selected time."}, http.StatusBadRequest)
                default:
                    sendJSONResponse(w, ReserveResponse{Error: "An unexpected error occurred."}, http.StatusInternalServerError)
                }
                return
            }

            appendLog("Immediate reservation successful")
            sendJSONResponse(w, ReserveResponse{
                ReservationTime: reserveResp.ReservationTime.Format("2006-01-02 15:04:05"),
            }, http.StatusOK)
        } else {
            // Schedule for later
            appendLog("Scheduling reservation for: " + requestTime.Format(time.RFC3339))
            scheduledReservations = append(scheduledReservations, ScheduledReservation{
                ReserveParam: reserveParam,
                RunTime:      requestTime,
            })
            sendJSONResponse(w, ReserveResponse{
                ReservationTime: "",
            }, http.StatusOK)
        }
    })

    // Logs endpoint
    http.HandleFunc("/api/logs", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(logLines)
    })

    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path != "/" {
            http.NotFound(w, r)
            return
        }
        data := TemplateData{
            Message: "Welcome to GoResyBot Where cravings meet convenience",
        }
        if err := tmpl.ExecuteTemplate(w, "index.html", data); err != nil {
            http.Error(w, "Failed to render template", http.StatusInternalServerError)
            appendLog("Template execution error: " + err.Error())
        }
    })

    http.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodGet {
            http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
            return
        }
        data := TemplateData{}
        if err := tmpl.ExecuteTemplate(w, "login.html", data); err != nil {
            http.Error(w, "Failed to render template", http.StatusInternalServerError)
            appendLog("Template execution error: " + err.Error())
        }
    })

    http.HandleFunc("/reserve", func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodGet {
            http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
            return
        }
        _, err := getSession(r)
        if err != nil {
            http.Redirect(w, r, "/login", http.StatusSeeOther)
            return
        }
        data := TemplateData{}
        if err := tmpl.ExecuteTemplate(w, "reserve.html", data); err != nil {
            http.Error(w, "Failed to render template", http.StatusInternalServerError)
            appendLog("Template execution error: " + err.Error())
        }
    })

    // Start the scheduling goroutine
    go handleScheduledReservations(appCtx)

    // Start server
    port := "8090"
    appendLog("Starting server on port " + port + "...")
    if err := http.ListenAndServe(":"+port, nil); err != nil {
        log.Fatalf("Failed to start server: %v", err)
    }
}

func handleScheduledReservations(appCtx app.AppCtx) {
    for {
        if len(scheduledReservations) == 0 {
            // No scheduled reservations, check again in a minute
            time.Sleep(1 * time.Minute)
            continue
        }

        now := time.Now().UTC()

        // Find the earliest scheduled reservation
        var earliestIndex int
        var earliestTime time.Time
        foundEarliest := false
        for i, sr := range scheduledReservations {
            if !foundEarliest || sr.RunTime.Before(earliestTime) {
                earliestTime = sr.RunTime
                earliestIndex = i
                foundEarliest = true
            }
        }

        if earliestTime.After(now) {
            // Sleep until the scheduled time
            sleepDuration := earliestTime.Sub(now)
            time.Sleep(sleepDuration)
        } else {
            // Time to attempt booking
            sr := scheduledReservations[earliestIndex]
            appendLog("Attempting scheduled reservation at: " + sr.RunTime.Format(time.RFC3339))
            _, err := appCtx.API.Reserve(sr.ReserveParam)
            if err != nil {
                appendLog("Failed to book scheduled reservation: " + err.Error())
            } else {
                appendLog("Successfully booked scheduled reservation at: " + sr.RunTime.Format(time.RFC3339))
            }

            // Remove the reservation from the list
            scheduledReservations = append(scheduledReservations[:earliestIndex], scheduledReservations[earliestIndex+1:]...)
        }
    }
}

// Helper function to send JSON responses
func sendJSONResponse(w http.ResponseWriter, response interface{}, statusCode int) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(statusCode)
    json.NewEncoder(w).Encode(response)
}

func getCookieValue(r *http.Request, name string) (string, error) {
    cookie, err := r.Cookie("session")
    if err != nil {
        return "", err
    }
    value := make(map[string]string)
    if err = s.Decode("session", cookie.Value, &value); err != nil {
        return "", err
    }
    return value[name], nil
}

func getSession(r *http.Request) (map[string]string, error) {
    cookie, err := r.Cookie("session")
    if err != nil {
        return nil, err
    }
    value := make(map[string]string)
    if err = s.Decode("session", cookie.Value, &value); err != nil {
        return nil, err
    }
    return value, nil
}

func parseTime(timeStr string) (time.Time, error) {
    // timeStr format: YYYY:MM:DD:HH:MM (UTC)
    parts := strings.Split(timeStr, ":")
    if len(parts) != 5 {
        return time.Time{}, &time.ParseError{Layout: "YYYY:MM:DD:HH:MM", Value: timeStr}
    }

    year, err := strconv.Atoi(parts[0])
    if err != nil {
        return time.Time{}, err
    }
    monthInt, err := strconv.Atoi(parts[1])
    if err != nil {
        return time.Time{}, err
    }
    day, err := strconv.Atoi(parts[2])
    if err != nil {
        return time.Time{}, err
    }
    hour, err := strconv.Atoi(parts[3])
    if err != nil {
        return time.Time{}, err
    }
    minute, err := strconv.Atoi(parts[4])
    if err != nil {
        return time.Time{}, err
    }

    return time.Date(year, time.Month(monthInt), day, hour, minute, 0, 0, time.UTC), nil
}

// appendLog adds a log message to both the standard log and in-memory slice
func appendLog(message string) {
    logLines = append(logLines, time.Now().Format("2006-01-02 15:04:05")+" "+message)
    log.Println(message)
}

