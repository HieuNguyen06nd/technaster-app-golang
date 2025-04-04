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
	connStr := fmt.Sprintf(
		"postgresql://%s:%s@%s:%d/%s",
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

	app.Get("/", func(ctx iris.Context) {
		ctx.View("index.html")
	})

	app.Post("/generate", func(ctx iris.Context) {
		prompt := ctx.FormValue("prompt")
		dialogContent, err := generateDialog(prompt)
		if err != nil {
			showError(ctx, "Lỗi tạo hội thoại", err.Error())
			return
		}

		var dialogID int64
		err = db.QueryRow(context.Background(),
			"INSERT INTO dialog (lang, content) VALUES ($1, $2) RETURNING id",
			"vi", dialogContent,
		).Scan(&dialogID)
		if err != nil {
			showError(ctx, "Lỗi lưu hội thoại", err.Error())
			return
		}

		extractPrompt := fmt.Sprintf(
			`Từ hội thoại này (CHỈ TRẢ LỜI JSON):
			"%s"
			Lọc từ quan trọng theo mẫu: {"words": ["từ1", "từ2"]}`, 
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

		translatePrompt := fmt.Sprintf(
			`Dịch các từ này sang tiếng Anh (CHỈ TRẢ LỜI JSON):
			%s
			Mẫu: {"translated_words": [{"vi": "...", "en": "..."}]}`, 
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

		for _, word := range result.TranslatedWords {
			var wordID int64
			err = db.QueryRow(context.Background(),
				"INSERT INTO word (lang, content, translate) VALUES ($1, $2, $3) RETURNING id",
				"vi", word.Vi, word.En,
			).Scan(&wordID)
			
			if err == nil {
				db.Exec(context.Background(),
					"INSERT INTO word_dialog (dialog_id, word_id) VALUES ($1, $2)",
					dialogID, wordID,
				)
			}
		}

		ctx.ViewData("Dialog", formatDialog(dialogContent))
		ctx.ViewData("Words", result.TranslatedWords)
		ctx.View("index.html")
	})

	app.Listen(":8080")
}

func generateDialog(prompt string) (string, error) {
	client := &http.Client{}
	requestBody := map[string]interface{}{
		"model": "deepseek-r1-distill-llama-70b",
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}

	body, _ := json.Marshal(requestBody)
	req, _ := http.NewRequest(
		"POST", 
		"https://api.groq.com/openai/v1/chat/completions", 
		bytes.NewReader(body),
	)
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
		return "", fmt.Errorf("response parse failed: %v", err)
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("empty response from API")
	}

	return strings.TrimSpace(response.Choices[0].Message.Content), nil
}

func formatDialog(content string) template.HTML {
	return template.HTML(strings.ReplaceAll(content, "\n", "<br>"))
}

func extractJSON(raw string) string {
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}") + 1
	if start == -1 || end == -1 {
		return ""
	}
	return raw[start:end]
}

func showError(ctx iris.Context, title string, content string) {
    ctx.ViewData("Error", map[string]interface{}{
        "Title":   title,
        "Content": content,
    })
    ctx.View("error.html")
}