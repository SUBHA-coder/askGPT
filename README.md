# AskGo

A command-line AI assistant that uses the Groq API with LLaMA3-8B model.

## Features

- Interactive chat interface
- Colored output for better readability
- Option to save conversations to a file
- Secure API key management using environment variables

## Installation

1. Make sure you have Go 1.21 or later installed
2. Clone this repository
3. Install dependencies:
   ```bash
   go mod download
   ```

## Configuration

1. Create a `.env` file in the project root
2. Add your Groq API key:
   ```
   GROQ_API_KEY=your_api_key_here
   ```

## Usage

Run the program:
```bash
go run main.go
```

To save the conversation to a file, use the `--save` flag:
```bash
go run main.go --save
```

The conversation will be saved to `conversation.txt` in the current directory.

## Commands

- Type your message and press Enter to chat with the AI
- Type `exit` or `quit` to end the conversation

## Dependencies

- github.com/fatih/color - For colored terminal output
- github.com/joho/godotenv - For environment variable management 