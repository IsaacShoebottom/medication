package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"html/template"
	"math"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"time"
)

type Timestamp struct {
	Current int64 `json:"current"`
	Next    int64 `json:"next"`
}

type Store struct {
	Username   string      `json:"username"`
	Timestamps []Timestamp `json:"timestamps"`
}

type PageData struct {
	Title         string
	Message       string
	ShowLogin     bool
	ShowInput     bool
	Authenticated bool
	Timestamps    [][]string
}

var Username string
var Password string
var DataDir string
var DataStore string
var Authorised []int64 // List of authorised "uuids"

func main() {
	// Get environment variables
	Username = os.Getenv("USERNAME")
	fmt.Println("Username: ", Username)

	Password = os.Getenv("PASSWORD")
	fmt.Println("Password: ", Password)

	DataDir = os.Getenv("DATA_DIR")
	if DataDir == "" {
		DataDir = "./data"
	}
	fmt.Println("DataDir: ", DataDir)

	DataStore = DataDir + "/data.json"

	Reset := os.Getenv("RESET")
	if Reset == "true" {
		if err := os.Remove(DataStore); err != nil {
			fmt.Println("Error removing file, check permissions")
		}
		if err := os.Remove(DataDir); err != nil {
			fmt.Println("Error removing directory, check permissions")
		}
	}

	// Check if directory exists, if not create it
	if _, err := os.Stat(DataDir); os.IsNotExist(err) {
		err := os.Mkdir(DataDir, 0755)
		if err != nil {
			fmt.Println("Error creating directory, check permissions")
			os.Exit(1)
		}
	}
	// Check if file exists, if not create it, and write empty json
	if _, err := os.Stat(DataStore); os.IsNotExist(err) {
		if _, err := os.Create(DataStore); err != nil {
			fmt.Println("Error creating file, check permissions")
			os.Exit(1)
		}
		if file, err := os.OpenFile(DataStore, os.O_RDWR, os.ModePerm); err != nil {
			fmt.Println("Error opening file, check permissions")
			os.Exit(1)
		} else {
			encoder := json.NewEncoder(file)
			store := Store{
				Username:   Username,
				Timestamps: []Timestamp{},
			}
			if err = encoder.Encode(store); err != nil {
				fmt.Println("Error encoding file")
				os.Exit(1)
			}
			if err = file.Close(); err != nil {
				fmt.Println("Error closing file")
				os.Exit(1)
			}
		}
	}

	// Start server
	http.HandleFunc("/", index)
	http.HandleFunc("/post", post)
	http.HandleFunc("/login", login)
	http.HandleFunc("/logout", logout)
	if err := http.ListenAndServe(":8080", nil); err != nil {
		fmt.Println("Error starting server")
		os.Exit(1)
	}
}

func commit(next int64) {
	current := time.Now().Unix()
	// Next is in hours, so convert to seconds
	next = next * 60 * 60
	// Next is the time when the form can be submitted again
	next = current + next

	// Read from file
	if file, err := os.OpenFile(DataStore, os.O_RDWR, os.ModePerm); err != nil {
		fmt.Println("Error opening file, check permissions")
		os.Exit(1)
	} else {
		decoder := json.NewDecoder(file)
		store := Store{}
		if err = decoder.Decode(&store); err != nil {
			fmt.Println("Error decoding file")
			os.Exit(1)
		}
		store.Timestamps = append(store.Timestamps, Timestamp{current, next})

		if _, err := file.Seek(0, 0); err != nil {
			fmt.Println("Error seeking file")
			os.Exit(1)
		}

		// Write to file
		encoder := json.NewEncoder(file)
		if err = encoder.Encode(store); err != nil {
			fmt.Println("Error encoding file")
			os.Exit(1)
		}
		if err = file.Close(); err != nil {
			fmt.Println("Error closing file")
			os.Exit(1)
		}
	}
}

func queryCanPost() bool {
	// Check if file exists
	if file, err := os.Open(DataStore); err != nil {
		fmt.Println("Error opening file, check permissions")
		os.Exit(1)
	} else {
		// Decode file
		decoder := json.NewDecoder(file)
		store := Store{}
		if err = decoder.Decode(&store); err != nil {
			fmt.Println("Error decoding file")
			os.Exit(1)
		}
		if err = file.Close(); err != nil {
			fmt.Println("Error closing file")
			os.Exit(1)
		}

		if len(store.Timestamps) == 0 {
			return true
		}

		// Check if user has already submitted form today
		current := time.Now().Unix()
		// Get last timestamp
		last := store.Timestamps[len(store.Timestamps)-1]
		// If current time is greater than the next time the form can be submitted
		if current > last.Next {
			return true
		}
	}
	return false
}

func queryData() []Timestamp {
	// Check if file exists
	if file, err := os.Open(DataStore); err != nil {
		fmt.Println("Error opening file, check permissions")
		os.Exit(1)
	} else {
		// Decode file
		decoder := json.NewDecoder(file)
		store := Store{}
		if err = decoder.Decode(&store); err != nil {
			fmt.Println("Error decoding file")
			os.Exit(1)
		}
		if err = file.Close(); err != nil {
			fmt.Println("Error closing file")
			os.Exit(1)
		}

		return store.Timestamps
	}
	return nil
}

func index(w http.ResponseWriter, r *http.Request) {
	fmt.Println("GET request received")
	tmpl := template.Must(template.ParseFiles("index.html"))
	auth := checkAuth(r)
	data := PageData{
		Authenticated: auth,
	}
	if auth {
		data.Title = "View data"
		data.Message = "Welcome back " + Username + ". Check in after the timeout"
		data.ShowInput = queryCanPost()

		var timestamps [][]string
		for _, ts := range queryData() {
			one := time.Unix(ts.Current, 0)
			two := time.Unix(ts.Next, 0)
			ended := time.Now().Unix() > ts.Next
			str := strconv.FormatBool(ended)
			timestamps = append(timestamps, []string{one.Format(time.DateTime), two.Format(time.DateTime), str})
		}

		data.Timestamps = timestamps
	} else {
		data.Title = "Log in"
		data.Message = "Please enter your username and password"
		data.ShowInput = false
		data.Timestamps = nil
	}
	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, "Error rendering template", http.StatusInternalServerError)
	}
}

func post(w http.ResponseWriter, r *http.Request) {
	fmt.Println("POST request received")

	if !checkAuth(r) {
		http.Error(w, "Not authorised", http.StatusUnauthorized)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}
	next := r.Form.Get("next")
	if next, err := strconv.ParseInt(next, 10, 64); err != nil {
		http.Error(w, "Invalid time", http.StatusBadRequest)
		return
	} else {
		commit(next)
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
	return
}

func login(w http.ResponseWriter, r *http.Request) {
	fmt.Println("LOGIN request received")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}
	username := r.Form.Get("username")
	password := r.Form.Get("password")
	if username != os.Getenv("USERNAME") || password != os.Getenv("PASSWORD") {
		http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		return
	}
	// Create session cookie
	if session, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64)); err != nil {
		http.Error(w, "Error creating session", http.StatusInternalServerError)
		return
	} else {
		Authorised = append(Authorised, session.Int64())
		cookie := http.Cookie{
			Name:    "session",
			Value:   session.String(),
			Expires: time.Now().Add(24 * time.Hour),
		}
		http.SetCookie(w, &cookie)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
}

func logout(w http.ResponseWriter, r *http.Request) {
	fmt.Println("LOGOUT request received")

	if !checkAuth(r) {
		http.Error(w, "Not authorised", http.StatusUnauthorized)
		return
	}

	cookie := http.Cookie{
		Name:  "session",
		Value: "",
	}
	http.SetCookie(w, &cookie)
	http.Redirect(w, r, "/", http.StatusSeeOther)
	return
}

func checkAuth(r *http.Request) bool {
	// Check for session cookie
	cookie, err := r.Cookie("session")
	if err != nil {
		return false
	} else {
		// Parse cookie to int64
		if session, err := strconv.ParseInt(cookie.Value, 10, 64); err != nil {
			fmt.Println("Error parsing cookie")
			return false
		} else {
			for _, uuid := range Authorised {
				if uuid == session {
					return true
				}
			}
		}
	}
	return false
}
