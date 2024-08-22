package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"time"
)

type Timestamp struct {
	Current int64 `json:"current"`
	Next    int64 `json:"next"`
}

type Store struct {
	Username   string  `json:"username"`
	Timestamps []int64 `json:"timestamps"`
}

type PageData struct {
	Title    string
	ShowForm bool
	Message  string
}

func main() {
	// Check if directory exists, if not create it
	if _, err := os.Stat("./data"); os.IsNotExist(err) {
		err := os.Mkdir("./data", 0755)
		if err != nil {
			fmt.Println("Error creating directory, check permissions")
			os.Exit(1)
		}
	}

	// Set environment variables
	RESET := os.Getenv("RESET")
	if RESET == "true" {
		if err := os.Remove("./data/taken.json"); err != nil {
			fmt.Println("Error removing file, check permissions")
		}
	}
	API_KEY := os.Getenv("API_KEY")
	fmt.Println("API_KEY: ", API_KEY)

	// Start server
	http.HandleFunc("/", index)
	if err := http.ListenAndServe(":8080", nil); err != nil {
		fmt.Println("Error starting server")
		os.Exit(1)
	}
}

func handlePost() PageData {
	current := time.Now().Unix()
	var day int64 = 60 * 60 * 24
	next := current + day

	// If there is no file, create one
	if _, err := os.Stat("./data/taken.json"); os.IsNotExist(err) {
		if _, err := os.Create("./data/taken.json"); err != nil {
			fmt.Println("Error creating file, check permissions")
			os.Exit(1)
		}
	}

	// Write to file
	if file, err := os.OpenFile("./data/taken.json", os.O_RDWR, os.ModePerm); err != nil {
		fmt.Println("Error opening file, check permissions")
		os.Exit(1)
	} else {
		encoder := json.NewEncoder(file)
		if err = encoder.Encode(Timestamp{current, next}); err != nil {
			fmt.Println("Error encoding file")
			os.Exit(1)
		}
		if err = file.Close(); err != nil {
			fmt.Println("Error closing file")
			os.Exit(1)
		}
	}

	return PageData{
		Title:    "Form submitted",
		ShowForm: false,
		Message:  "Form submitted",
	}
}

func checkFile() PageData {
	// Check if file exists
	file, err := os.Open("./data/taken.json")
	if err != nil {
		fmt.Println("Error opening file, check perms or likely does not exist")
		return PageData{
			Title:    "Form page",
			Message:  "Hello first time user, please enter API key to submit form",
			ShowForm: true,
		}
	}
	// Decode file
	info := Timestamp{}
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&info); err != nil {
		fmt.Println("Error decoding file")
	}
	if err := file.Close(); err != nil {
		fmt.Println("Error closing file")
	}
	if time.Now().Unix() > info.Next {
		return PageData{
			Title:    "Form page",
			Message:  "Already submmited form today, please wait until tomorrow",
			ShowForm: false,
		}
	}
	return PageData{
		Title:    "Form page",
		Message:  "Please enter API key to submit form",
		ShowForm: true,
	}
}

func index(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("index.html"))
	var data PageData
	if r.Method == "GET" {
		fmt.Println("GET request received")
		data = checkFile()
	}
	if r.Method == "POST" {
		fmt.Println("POST request received")
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Error parsing form", http.StatusBadRequest)
			return
		}
		if r.Form.Get("key") != os.Getenv("API_KEY") {
			http.Error(w, "Invalid API key", http.StatusUnauthorized)
			return
		}
		data = handlePost()
	}
	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, "Error rendering template", http.StatusInternalServerError)
	}
}
