package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/joho/godotenv"
)

type GroqRequest struct {
	Messages []Message `json:"messages"`
	Model    string    `json:"model"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type GroqResponse struct {
	Choices []Choice `json:"choices"`
}

type Choice struct {
	Message Message `json:"message"`
}

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		fmt.Println("Error loading .env file")
		os.Exit(1)
	}

	apiKey := os.Getenv("GROQ_API_KEY")
	if apiKey == "" {
		fmt.Println("GROQ_API_KEY environment variable not set")
		os.Exit(1)
	}

	// Create a new Fyne application
	myApp := app.New()
	window := myApp.NewWindow("AskGo AI Assistant")
	window.Resize(fyne.NewSize(800, 600))

	// Create conversation history display
	history := widget.NewTextGrid()
	history.SetText("Welcome to AskGo AI Assistant!\n\n")

	// Create input field
	input := widget.NewEntry()
	input.SetPlaceHolder("Type your message here...")

	// Create send button
	sendButton := widget.NewButton("Send", nil)

	// Create scroll container for history
	scrollContainer := container.NewScroll(history)
	scrollContainer.Resize(fyne.NewSize(800, 500))

	// Create main container
	content := container.NewBorder(
		nil,
		container.NewHBox(input, sendButton),
		nil,
		nil,
		scrollContainer,
	)

	// Set the content
	window.SetContent(content)

	// Handle send button click
	sendButton.OnTapped = func() {
		prompt := input.Text
		if strings.TrimSpace(prompt) == "" {
			return
		}

		// Add user message to history
		history.SetText(history.Text() + "You: " + prompt + "\n")
		input.SetText("")

		// Scroll to bottom
		scrollContainer.ScrollToBottom()

		// Prepare the request
		requestBody := GroqRequest{
			Messages: []Message{
				{
					Role:    "user",
					Content: prompt,
				},
			},
			Model: "llama3-8b-8192",
		}

		jsonData, err := json.Marshal(requestBody)
		if err != nil {
			history.SetText(history.Text() + "Error: " + err.Error() + "\n")
			return
		}

		// Create HTTP request
		req, err := http.NewRequest("POST", "https://api.groq.com/openai/v1/chat/completions", strings.NewReader(string(jsonData)))
		if err != nil {
			history.SetText(history.Text() + "Error: " + err.Error() + "\n")
			return
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+apiKey)

		// Send request
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			history.SetText(history.Text() + "Error: " + err.Error() + "\n")
			return
		}
		defer resp.Body.Close()

		// Read response
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			history.SetText(history.Text() + "Error: " + err.Error() + "\n")
			return
		}

		// Parse response
		var groqResponse GroqResponse
		if err := json.Unmarshal(body, &groqResponse); err != nil {
			history.SetText(history.Text() + "Error: " + err.Error() + "\n")
			return
		}

		// Add AI response to history
		response := groqResponse.Choices[0].Message.Content
		history.SetText(history.Text() + "AI: " + response + "\n\n")

		// Scroll to bottom
		scrollContainer.ScrollToBottom()
	}

	// Handle Enter key in input field
	input.OnSubmitted = func(text string) {
		sendButton.OnTapped()
	}

	// Show and run
	window.ShowAndRun()
}
