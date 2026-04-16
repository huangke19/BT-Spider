package search

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/huangke/bt-spider/pkg/httputil"
)

var groqClient = httputil.NewClient(10 * time.Second)

const groqEndpoint = "https://api.groq.com/openai/v1/chat/completions"

const groqSystemPrompt = `You are a movie information extractor. Extract the movie title and release year from the user's input.

Rules:
- Return ONLY valid JSON with exactly these fields: {"title_en": "...", "year": "..."}
- title_en: The standard English movie title as listed on IMDb/TMDB (e.g. "Captain America: The Winter Soldier")
- year: 4-digit release year as a string, empty string if unknown
- Focus on movies only; if the input is not about a movie, return {"title_en": "", "year": ""}
- Do not include any explanation, only output the JSON object`

type groqRequest struct {
	Model          string        `json:"model"`
	Messages       []groqMessage `json:"messages"`
	ResponseFormat groqFormat    `json:"response_format"`
	MaxTokens      int           `json:"max_tokens"`
	Temperature    float64       `json:"temperature"`
}

type groqMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type groqFormat struct {
	Type string `json:"type"`
}

type groqResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type groqMovieResult struct {
	TitleEn string `json:"title_en"`
	Year    string `json:"year"`
}

// ResolveWithGroq 通过 Groq API 从自然语言输入中提取电影英文标题和年份。
func ResolveWithGroq(input, apiKey string) (movieMeta, bool) {
	body := groqRequest{
		Model: "llama-3.1-8b-instant",
		Messages: []groqMessage{
			{Role: "system", Content: groqSystemPrompt},
			{Role: "user", Content: input},
		},
		ResponseFormat: groqFormat{Type: "json_object"},
		MaxTokens:      100,
		Temperature:    0,
	}

	data, err := json.Marshal(body)
	if err != nil {
		return movieMeta{}, false
	}

	req, err := http.NewRequest(http.MethodPost, groqEndpoint, bytes.NewReader(data))
	if err != nil {
		return movieMeta{}, false
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := groqClient.Do(req)
	if err != nil {
		return movieMeta{}, false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return movieMeta{}, false
	}

	var gr groqResponse
	if err := json.NewDecoder(resp.Body).Decode(&gr); err != nil {
		return movieMeta{}, false
	}
	if len(gr.Choices) == 0 {
		return movieMeta{}, false
	}

	var result groqMovieResult
	if err := json.Unmarshal([]byte(gr.Choices[0].Message.Content), &result); err != nil {
		return movieMeta{}, false
	}

	result.TitleEn = strings.TrimSpace(result.TitleEn)
	result.Year = strings.TrimSpace(result.Year)
	if result.TitleEn == "" {
		return movieMeta{}, false
	}
	return movieMeta{Title: result.TitleEn, Year: result.Year}, true
}
