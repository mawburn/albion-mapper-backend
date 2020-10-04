package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/tebro/albion-mapper-backend/albion"

	"github.com/tebro/albion-mapper-backend/db"

	"github.com/gorilla/mux"

	"github.com/rs/cors"
)

type apiPortal struct {
	Source  string `json:"source"`
	Target  string `json:"target"`
	Size    int    `json:"size"`
	Hours   int    `json:"hours"`
	Minutes int    `json:"minutes"`
}

var password = os.Getenv("AUTH_PASSWORD")
var publicRead = os.Getenv("PUBLIC_READ") == "true"

func isAuth(r *http.Request) bool {
	if publicRead && r.Method == "GET" {
		return true
	}
	header := r.Header["X-Tebro-Auth"]
	for _, s := range header {
		if s == password {
			return true
		}
	}
	return false
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Hello world")
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	_, err := db.Hello()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "Error")
		log.Printf("Health error: %v\n", err)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "OK")
}

func send401(w http.ResponseWriter) {
	w.WriteHeader(http.StatusUnauthorized)
	fmt.Fprint(w, "Authenticate")
}

func send400AndLog(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusBadRequest)
	fmt.Fprint(w, "Bad request")
	log.Printf("400 error: %v\n", err)
}

func send500AndLog(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusInternalServerError)
	fmt.Fprint(w, "Error")
	log.Printf("500 error: %v\n", err)
}

func getZonesHandler(w http.ResponseWriter, r *http.Request) {
	if !isAuth(r) {
		send401(w)
		return
	}
	zones := albion.GetZones()
	json, err := json.Marshal(zones)
	if err != nil {
		send500AndLog(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, string(json))
}

func getPortalsHandler(w http.ResponseWriter, r *http.Request) {
	if !isAuth(r) {
		send401(w)
		return
	}
	portals, err := albion.GetPortals()
	if err != nil {
		send500AndLog(w, err)
		return
	}
	json, err := json.Marshal(portals)
	if err != nil {
		send500AndLog(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, string(json))
}

func addPortalHandler(w http.ResponseWriter, r *http.Request) {
	if !isAuth(r) {
		send401(w)
		return
	}

	var portal apiPortal
	err := json.NewDecoder(r.Body).Decode(&portal)
	if err != nil {
		send400AndLog(w, err)
		return
	}

	expires := time.Now()
	expires = expires.Add(time.Hour * time.Duration(portal.Hours))
	expires = expires.Add(time.Minute * time.Duration(portal.Minutes))

	dbPortal := albion.Portal{
		Source:  portal.Source,
		Target:  portal.Target,
		Size:    portal.Size,
		Expires: expires,
	}

	isValid, err := albion.IsValidPortal(dbPortal)
	if err != nil {
		send500AndLog(w, err)
		return
	}

	if !isValid {
		send400AndLog(w, fmt.Errorf("Invalid portal: %v", dbPortal))
		return
	}

	err = albion.AddPortal(dbPortal)
	if err != nil {
		send500AndLog(w, err)
		return
	}

	w.WriteHeader(http.StatusAccepted)
	fmt.Fprint(w, "ACCEPTED")
}

func configHandler(w http.ResponseWriter, r *http.Request) {
	data, _ := json.Marshal(map[string]interface{}{
		"publicRead": publicRead,
	})
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, "%s", data)
}

func setupRoutes(r *mux.Router) {
	r.HandleFunc("/", rootHandler)
	r.HandleFunc("/health", healthHandler)
	r.HandleFunc("/api/config", configHandler)
	r.HandleFunc("/api/zone", getZonesHandler)
	r.HandleFunc("/api/portal", addPortalHandler).Methods("POST")
	r.HandleFunc("/api/portal", getPortalsHandler)
}

// StartServer starts the HTTP server
func StartServer() error {
	router := mux.NewRouter()
	setupRoutes(router)

	c := cors.New(cors.Options{
		AllowedHeaders:   []string{"X-Requested-With"},
		AllowedOrigins:   []string{"*"},
		AllowCredentials: true,
		AllowedMethods:   []string{"GET", "HEAD", "POST", "PUT", "OPTIONS"},
	})

	handler := c.Handler(router)

	log.Println("Server starting on port 8080")
	err := http.ListenAndServe(":8080", handler)
	return err
}
