package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/kataras/iris/v12"
)

const (
	GROQ_API_URL    = "https://api.groq.com/openai/v1/chat/completions"
	REQUEST_TIMEOUT = 30 * time.Second
	DEFAULT_MODEL   = "llama3-70b-8192"
	API_KEY         = "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" // Thay bằng key thật
)

type GroqRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float32   `json:"temperature"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type GroqResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

func callGroqAPI(prompt string) (string, error) {
	requestBody := GroqRequest{
		Model: DEFAULT_MODEL,
		Messages: []Message{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Temperature: 0.7,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("error marshaling request: %w", err)
	}

	client := &http.Client{Timeout: REQUEST_TIMEOUT}
	req, err := http.NewRequest("POST", GROQ_API_URL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+API_KEY)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result GroqResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("error parsing response: %w", err)
	}

	if len(result.Choices) > 0 && result.Choices[0].Message.Content != "" {
		return result.Choices[0].Message.Content, nil
	}

	return "", fmt.Errorf("empty response from API")
}

func generateComparisonResponse(prompt string) string {
	re := regexp.MustCompile(`(\d+\.?\d*)\s+or\s+(\d+\.?\d*)`)
	matches := re.FindStringSubmatch(prompt)
	if len(matches) < 3 {
		return "Không tìm thấy hai số hợp lệ để so sánh"
	}

	num1, err1 := strconv.ParseFloat(matches[1], 64)
	num2, err2 := strconv.ParseFloat(matches[2], 64)
	if err1 != nil || err2 != nil {
		return "Dữ liệu đầu vào không phải là số hợp lệ"
	}

	// Format numbers with 2 decimal places
	strNum1 := fmt.Sprintf("%.2f", num1)
	strNum2 := fmt.Sprintf("%.2f", num2)

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Comparing %s and %s for Size\n\n", matches[1], matches[2]))
	result.WriteString(fmt.Sprintf("%s or %s which number is bigger?\n\n", matches[1], matches[2]))
	result.WriteString("To determine which number is larger, let's compare them step by step.\n\n")
	result.WriteString("Step-by-Step Comparison:\n\n")

	// Compare whole number parts
	intPart := int(num1) // Chỉ cần dùng 1 biến vì phần nguyên bằng nhau
	result.WriteString("1. Compare the Whole Number Parts:\n")
	result.WriteString(fmt.Sprintf("   - Both numbers have the same whole number part: %d.\n", intPart))

	// Compare decimal parts
	decimal1 := strings.Split(strNum1, ".")[1]
	decimal2 := strings.Split(strNum2, ".")[1]

	result.WriteString("2. Compare the Tenths Place:\n")
	result.WriteString(fmt.Sprintf("   - %s has a %c in the tenths place.\n", strNum1, decimal1[0]))
	result.WriteString(fmt.Sprintf("   - %s has a %c in the tenths place.\n", strNum2, decimal2[0]))

	if decimal1[0] > decimal2[0] {
		result.WriteString(fmt.Sprintf("   - Since %c > %c, %s is larger than %s at this stage.\n\n", 
			decimal1[0], decimal2[0], strNum1, strNum2))
	} else if decimal1[0] < decimal2[0] {
		result.WriteString(fmt.Sprintf("   - Since %c < %c, %s is smaller than %s at this stage.\n\n", 
			decimal1[0], decimal2[0], strNum1, strNum2))
	} else {
		result.WriteString("   - The tenths place is equal, comparing next digit...\n\n")
	}

	result.WriteString("For clarity, let's express both numbers with the same decimal places:\n")
	result.WriteString(fmt.Sprintf("   - %s becomes %s\n", matches[1], strNum1))
	result.WriteString(fmt.Sprintf("   - %s remains %s\n\n", matches[2], strNum2))

	result.WriteString("Final Answer:\n\n")
	if num1 > num2 {
		result.WriteString(fmt.Sprintf("[%s]", matches[1]))
	} else if num1 < num2 {
		result.WriteString(fmt.Sprintf("[%s]", matches[2]))
	} else {
		result.WriteString("Both numbers are equal")
	}

	return result.String()
}

func handleAsk(ctx iris.Context) {
	var request struct {
		Prompt string `json:"prompt"`
	}

	if err := ctx.ReadJSON(&request); err != nil {
		ctx.StatusCode(iris.StatusBadRequest)
		ctx.JSON(iris.Map{"error": "Invalid request format"})
		return
	}

	// Check if it's a number comparison question
	if strings.Contains(strings.ToLower(request.Prompt), "which number is bigger") ||
		strings.Contains(strings.ToLower(request.Prompt), "which is larger") {
		
		response := generateComparisonResponse(request.Prompt)
		ctx.JSON(iris.Map{"response": response})
		return
	}

	// For other questions, use Groq API
	response, err := callGroqAPI(request.Prompt)
	if err != nil {
		ctx.StatusCode(iris.StatusInternalServerError)
		ctx.JSON(iris.Map{"error": err.Error()})
		return
	}
	ctx.JSON(iris.Map{"response": response})
}

func main() {
	app := iris.New()

	// CORS configuration
	app.Use(func(ctx iris.Context) {
		ctx.Header("Access-Control-Allow-Origin", "*")
		ctx.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		ctx.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		ctx.Next()
	})

	// Serve static files
	app.HandleDir("/", iris.Dir("./templates"))

	// API endpoint
	app.Post("/ask", handleAsk)

	// Start server
	fmt.Println("Server running at http://localhost:8080")
	app.Listen(":8080", iris.WithOptimizations)
}