package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kataras/iris/v12"
)

const (
	host     = "localhost"
	port     = 5432
	user     = "postgres"
	password = "123456"
	dbname   = "language_db"
	groqAPI  = "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"
)

var db *pgxpool.Pool

func main() {
	// Khởi tạo kết nối database
	connStr := fmt.Sprintf(
		"postgresql://%s:%s@%s:%d/%s?pool_max_conns=10",
		user, password, host, port, dbname,
	)

	var err error
	db, err = pgxpool.New(context.Background(), connStr)
	if err != nil {
		panic(fmt.Sprintf("Unable to connect to database: %v", err))
	}
	defer db.Close()

	app := iris.New()
	app.RegisterView(iris.HTML("./templates", ".html").Reload(true))

	// Route chính
	app.Get("/", func(ctx iris.Context) {
		ctx.View("index.html")
	})

	app.Post("/generate", func(ctx iris.Context) {
		// Bước 1: Tạo hội thoại từ prompt
		prompt := ctx.FormValue("prompt")
		dialogContent, err := generateDialog(prompt)
		if err != nil {
			showError(ctx, "Lỗi tạo hội thoại", err.Error())
			return
		}

		// Bước 2: Lưu hội thoại vào database
		var dialogID int64
		err = db.QueryRow(context.Background(),
			"INSERT INTO dialog (lang, content) VALUES ($1, $2) RETURNING id",
			"vi", dialogContent,
		).Scan(&dialogID)
		if err != nil {
			showError(ctx, "Lỗi lưu hội thoại", err.Error())
			return
		}

		// Bước 3: Sử dụng AI để nhận diện và loại bỏ tên riêng
		extractPrompt := fmt.Sprintf(
			`Phân tích hội thoại sau và trích xuất từ quan trọng:
			"%s"
			Yêu cầu:
			1. Loại bỏ tất cả tên riêng (PERSON), địa điểm (LOCATION), tổ chức (ORGANIZATION)
			2. Chỉ giữ lại từ thông dụng (common words)
			3. Bỏ qua các từ chỉ tên (name entities)
			4. Trả về dạng JSON: {"words": ["từ1", "từ2"]}
			5. Chỉ trả về JSON, không giải thích`,
			dialogContent,
		)
		
		rawWords, err := generateDialog(extractPrompt)
		if err != nil {
			showError(ctx, "Lỗi trích xuất từ", err.Error())
			return
		}

		wordsJSON := extractJSON(rawWords)
		if wordsJSON == "" {
			showError(ctx, "Lỗi định dạng từ", "Không tìm thấy JSON hợp lệ")
			return
		}

		// Bước 4: Dịch các từ đã được lọc
		translatePrompt := fmt.Sprintf(
			`Dịch các từ tiếng Việt sau sang tiếng Anh:
			%s
			Yêu cầu:
			1. Dịch chính xác theo ngữ cảnh thông dụng
			2. Trả về JSON: {"translated_words": [{"vi": "...", "en": "..."}]}
			3. Chỉ trả về JSON, không giải thích`,
			wordsJSON,
		)
		
		rawTranslation, err := generateDialog(translatePrompt)
		if err != nil {
			showError(ctx, "Lỗi dịch từ", err.Error())
			return
		}

		translatedJSON := extractJSON(rawTranslation)
		if translatedJSON == "" {
			showError(ctx, "Lỗi định dạng dịch", "Không tìm thấy JSON hợp lệ")
			return
		}

		var result struct {
			TranslatedWords []struct {
				Vi string `json:"vi"`
				En string `json:"en"`
			} `json:"translated_words"`
		}

		if err := json.Unmarshal([]byte(translatedJSON), &result); err != nil {
			showError(ctx, "Lỗi xử lý dữ liệu", fmt.Sprintf("%v\nDữ liệu: %s", err, translatedJSON))
			return
		}

		// Bước 5: Lưu từ và bản dịch vào database
		for _, word := range result.TranslatedWords {
			var wordID int64
			err = db.QueryRow(context.Background(),
				`INSERT INTO word (lang, content, translate) 
				VALUES ($1, $2, $3)
				ON CONFLICT (content) DO UPDATE SET translate = EXCLUDED.translate
				RETURNING id`,
				"vi", word.Vi, word.En,
			).Scan(&wordID)
			
			if err == nil {
				db.Exec(context.Background(),
					"INSERT INTO word_dialog (dialog_id, word_id) VALUES ($1, $2) ON CONFLICT DO NOTHING",
					dialogID, wordID,
				)
			}
		}

		// Hiển thị kết quả
		ctx.ViewData("Dialog", formatDialog(dialogContent))
		ctx.ViewData("Words", result.TranslatedWords)
		ctx.View("index.html")
	})

	app.Listen(":8080")
}

// Hàm gọi Groq API
func generateDialog(prompt string) (string, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	requestBody := map[string]interface{}{
		"model": "deepseek-r1-distill-llama-70b",
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"temperature": 0.7,
		"max_tokens":  1000,
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %v", err)
	}

	req, err := http.NewRequest(
		"POST", 
		"https://api.groq.com/openai/v1/chat/completions", 
		bytes.NewReader(body),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+groqAPI)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("API request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("failed to decode response: %v", err)
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("empty response from API")
	}

	return strings.TrimSpace(response.Choices[0].Message.Content), nil
}

// Định dạng hội thoại để hiển thị HTML
func formatDialog(content string) template.HTML {
	return template.HTML(strings.ReplaceAll(content, "\n", "<br>"))
}

// Trích xuất chuỗi JSON từ response
func extractJSON(raw string) string {
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}") + 1
	if start == -1 || end == -1 || start >= end {
		return ""
	}
	return raw[start:end]
}

// Hiển thị trang lỗi
func showError(ctx iris.Context, title string, content string) {
	ctx.ViewData("Error", map[string]string{
		"Title":   title,
		"Content": content,
	})
	if err := ctx.View("error.html"); err != nil {
		ctx.WriteString(fmt.Sprintf("Error: %s - %s", title, content))
	}
}