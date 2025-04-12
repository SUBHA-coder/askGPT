package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/gorilla/sessions"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"askgo/database"
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
	User     *database.User
	Error    string
}

var (
	messages []string
	store    = sessions.NewCookieStore([]byte("your-secret-key"))
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	clients   = make(map[*websocket.Conn]bool)
	clientsMu sync.Mutex
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		fmt.Println("Error loading .env file")
		os.Exit(1)
	}

	// Initialize database
	if err := database.InitDB(); err != nil {
		fmt.Println("Error initializing database:", err)
		os.Exit(1)
	}
	defer database.CloseDB()

	// Initialize messages slice
	messages = make([]string, 0)

	// Serve static files
	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// Handle routes
	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/login", handleLogin)
	http.HandleFunc("/signup", handleSignup)
	http.HandleFunc("/logout", handleLogout)
	http.HandleFunc("/chat", handleChat)
	http.HandleFunc("/new-chat", handleNewChat)
	http.HandleFunc("/ws", handleWebSocket)

	// Start server
	fmt.Println("Starting server on http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}

func getUserFromSession(r *http.Request) *database.User {
	session, _ := store.Get(r, "session")
	if userID, ok := session.Values["user_id"].(string); ok {
		objID, err := primitive.ObjectIDFromHex(userID)
		if err != nil {
			return nil
		}
		user, err := database.GetUserByID(objID)
		if err != nil {
			return nil
		}
		return user
	}
	return nil
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		tmpl := template.Must(template.ParseFiles("templates/login.html"))
		tmpl.Execute(w, PageData{})
		return
	}

	email := r.FormValue("email")
	password := r.FormValue("password")

	user, err := database.AuthenticateUser(email, password)
	if err != nil {
		tmpl := template.Must(template.ParseFiles("templates/login.html"))
		tmpl.Execute(w, PageData{Error: "Invalid email or password"})
		return
	}

	session, _ := store.Get(r, "session")
	session.Values["user_id"] = user.ID.Hex()
	session.Save(r, w)

	// Load user's chat history
	chatMessages, err := database.GetChatHistory(user.ID)
	if err != nil {
		messages = make([]string, 0)
	} else {
		messages = chatMessages
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func handleSignup(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		tmpl := template.Must(template.ParseFiles("templates/signup.html"))
		tmpl.Execute(w, PageData{})
		return
	}

	username := r.FormValue("username")
	email := r.FormValue("email")
	password := r.FormValue("password")
	confirmPassword := r.FormValue("confirm_password")

	if password != confirmPassword {
		tmpl := template.Must(template.ParseFiles("templates/signup.html"))
		tmpl.Execute(w, PageData{Error: "Passwords do not match"})
		return
	}

	user, err := database.CreateUser(username, email, password)
	if err != nil {
		tmpl := template.Must(template.ParseFiles("templates/signup.html"))
		tmpl.Execute(w, PageData{Error: "Error creating user"})
		return
	}

	session, _ := store.Get(r, "session")
	session.Values["user_id"] = user.ID.Hex()
	session.Save(r, w)

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "session")
	if userID, ok := session.Values["user_id"].(string); ok {
		objID, err := primitive.ObjectIDFromHex(userID)
		if err == nil {
			// Save chat history before clearing session
			if len(messages) > 0 {
				database.SaveChatHistory(objID, messages)
			}
		}
	}
	
	// Clear session and messages
	delete(session.Values, "user_id")
	session.Save(r, w)
	messages = make([]string, 0)
	
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	user := getUserFromSession(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	tmpl := template.Must(template.New("index.html").Funcs(template.FuncMap{
		"contains": strings.Contains,
	}).ParseFiles("templates/index.html"))

	data := PageData{
		Messages: messages,
		User:     user,
	}
	tmpl.Execute(w, data)
}

func handleChat(w http.ResponseWriter, r *http.Request) {
	user := getUserFromSession(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userMessage := r.FormValue("message")
	if userMessage == "" {
		http.Error(w, "Message is required", http.StatusBadRequest)
		return
	}

	// Only add to messages, don't broadcast (will be handled by WebSocket)
	messages = append(messages, "You: "+userMessage)

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

	req, err := http.NewRequest("POST", "https://api.groq.com/openai/v1/chat/completions", strings.NewReader(string(jsonData)))
	if err != nil {
		http.Error(w, "Error creating request", http.StatusInternalServerError)
		return
	}

	apiKey := os.Getenv("GROQ_API_KEY")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "Error sending request", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Error reading response", http.StatusInternalServerError)
		return
	}

	var groqResponse GroqResponse
	if err := json.Unmarshal(body, &groqResponse); err != nil {
		http.Error(w, "Error parsing response", http.StatusInternalServerError)
		return
	}

	response := groqResponse.Choices[0].Message.Content
	messages = append(messages, "AI: "+response)

	// Save updated chat history
	if err := database.SaveChatHistory(user.ID, messages); err != nil {
		fmt.Println("Error saving chat history:", err)
	}

	// Send response back to client
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"response": response})
}

func handleNewChat(w http.ResponseWriter, r *http.Request) {
	user := getUserFromSession(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Clear current messages
	messages = make([]string, 0)
	
	// Clear chat history in database
	err := database.ClearChatHistory(user.ID)
	if err != nil {
		http.Error(w, "Error clearing chat history", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("Error upgrading to WebSocket:", err)
		return
	}

	clientsMu.Lock()
	clients[conn] = true
	clientsMu.Unlock()

	defer func() {
		clientsMu.Lock()
		delete(clients, conn)
		clientsMu.Unlock()
		conn.Close()
	}()

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func broadcastMessage(message string) {
	clientsMu.Lock()
	defer clientsMu.Unlock()

	for client := range clients {
		err := client.WriteMessage(websocket.TextMessage, []byte(message))
		if err != nil {
			client.Close()
			delete(clients, client)
		}
	}
}
