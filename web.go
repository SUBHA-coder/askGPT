package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type GroqRequest struct {
	Messages []ChatMessage `json:"messages"`
	Model    string        `json:"model"`
}

type GroqResponse struct {
	Choices []Choice `json:"choices"`
}

type Choice struct {
	Message ChatMessage `json:"message"`
}

type PageData struct {
	Messages []string
}

var messages []string

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		fmt.Println("Error loading .env file")
		os.Exit(1)
	}

	// Initialize messages slice
	messages = make([]string, 0)

	// Serve static files
	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// Handle routes
	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/chat", handleChat)
	http.HandleFunc("/new-chat", handleNewChat)

	// Start server
	fmt.Println("Starting server on http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	// Create template with custom functions
	tmpl := template.Must(template.New("index.html").Funcs(template.FuncMap{
		"contains": strings.Contains,
	}).ParseFiles("templates/index.html"))

	data := PageData{
		Messages: messages,
	}
	tmpl.Execute(w, data)
}

func handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get user message from form
	userMessage := r.FormValue("message")
	if userMessage == "" {
		http.Error(w, "Message is required", http.StatusBadRequest)
		return
	}

	// Add user message to history
	messages = append(messages, "You: "+userMessage)

	// Prepare the request
	requestBody := GroqRequest{
		Messages: []ChatMessage{
			{
				Role:    "user",
				Content: userMessage,
			},
		},
		Model: "llama3-8b-8192",
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		http.Error(w, "Error preparing request", http.StatusInternalServerError)
		return
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", "https://api.groq.com/openai/v1/chat/completions", strings.NewReader(string(jsonData)))
	if err != nil {
		http.Error(w, "Error creating request", http.StatusInternalServerError)
		return
	}

	apiKey := os.Getenv("GROQ_API_KEY")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "Error sending request", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Error reading response", http.StatusInternalServerError)
		return
	}

	// Parse response
	var groqResponse GroqResponse
	if err := json.Unmarshal(body, &groqResponse); err != nil {
		http.Error(w, "Error parsing response", http.StatusInternalServerError)
		return
	}

	// Add AI response to history with markdown formatting
	response := groqResponse.Choices[0].Message.Content
	messages = append(messages, "AI: "+response)

	// Return success response
	w.WriteHeader(http.StatusOK)
}

func handleNewChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Clear messages
	messages = make([]string, 0)

	// Return success response
	w.WriteHeader(http.StatusOK)
}
