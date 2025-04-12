package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/joho/godotenv"

	"askgo/gui"
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

	// Parse command line flags
	saveFlag := flag.Bool("save", false, "Save the conversation to a file")
	guiFlag := flag.Bool("gui", false, "Start the GUI version")
	webFlag := flag.Bool("web", false, "Start the web interface")
	flag.Parse()

	apiKey := os.Getenv("GROQ_API_KEY")
	if apiKey == "" {
		fmt.Println("GROQ_API_KEY environment variable not set")
		os.Exit(1)
	}

	// Initialize color output
	userColor := color.New(color.FgGreen).SprintFunc()
	aiColor := color.New(color.FgCyan).SprintFunc()

	// Create a slice to store conversation history
	var history []string

	if *webFlag {
		startWebServer()
	} else if *guiFlag {
		gui.StartGUI()
	} else {
		for {
			fmt.Print(userColor("You: "))
			reader := bufio.NewReader(os.Stdin)
			prompt, err := reader.ReadString('\n')
			if err != nil {
				fmt.Println("Error reading input:", err)
				continue
			}
			prompt = strings.TrimSpace(prompt)

			if prompt == "exit" || prompt == "quit" {
				break
			}

			// Add to history
			history = append(history, "You: "+prompt)

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
				fmt.Println("Error marshaling request:", err)
				continue
			}

			// Create HTTP request
			req, err := http.NewRequest("POST", "https://api.groq.com/openai/v1/chat/completions", bytes.NewBuffer(jsonData))
			if err != nil {
				fmt.Println("Error creating request:", err)
				continue
			}

			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+apiKey)

			// Send request
			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				fmt.Println("Error sending request:", err)
				continue
			}
			defer resp.Body.Close()

			// Read response
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				fmt.Println("Error reading response:", err)
				continue
			}

			// Parse response
			var groqResponse GroqResponse
			if err := json.Unmarshal(body, &groqResponse); err != nil {
				fmt.Println("Error parsing response:", err)
				continue
			}

			// Print AI response
			response := groqResponse.Choices[0].Message.Content
			fmt.Println(aiColor("AI: " + response))

			// Add to history
			history = append(history, "AI: "+response)

			// Save conversation if flag is set
			if *saveFlag {
				if err := saveConversation(history); err != nil {
					fmt.Println("Error saving conversation:", err)
				}
			}
		}
	}
}

func saveConversation(history []string) error {
	file, err := os.OpenFile("conversation.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	for _, line := range history {
		if _, err := file.WriteString(line + "\n"); err != nil {
			return err
		}
	}
	return nil
}
