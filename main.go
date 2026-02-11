package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/joho/godotenv"
	"github.com/sashabaranov/go-openai"
)

type ChatRequest struct {
	UserID  string `json:"user_id"`
	Message string `json:"message"`
}

type ChatResponse struct {
	Reply string `json:"reply"`
}

var (
	chatHistory = make(map[string][]openai.ChatCompletionMessage)
	mutex       sync.Mutex
)

func main() {
	_ = godotenv.Load()

	apiKey := os.Getenv("GROQ_API_KEY")
	if apiKey == "" {
		log.Fatal("GROQ_API_KEY not set")
	}

	config := openai.DefaultConfig(apiKey)
	config.BaseURL = "https://api.groq.com/openai/v1"
	client := openai.NewClientWithConfig(config)

	ctx := context.Background()

	// Serve frontend
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/", fs)

	// Chat API
	http.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
			return
		}

		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		if req.UserID == "" || req.Message == "" {
			http.Error(w, "user_id and message required", http.StatusBadRequest)
			return
		}

		mutex.Lock()
		chatHistory[req.UserID] = append(chatHistory[req.UserID],
			openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleUser,
				Content: req.Message,
			},
		)
		messages := chatHistory[req.UserID]
		mutex.Unlock()

		resp, err := client.CreateChatCompletion(
			ctx,
			openai.ChatCompletionRequest{
				Model:    "llama-3.1-8b-instant",
				Messages: messages,
			},
		)

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		reply := resp.Choices[0].Message.Content

		mutex.Lock()
		chatHistory[req.UserID] = append(chatHistory[req.UserID],
			openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleAssistant,
				Content: reply,
			},
		)
		mutex.Unlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ChatResponse{Reply: reply})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Println("🚀 Server running on port", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
