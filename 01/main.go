package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/kataras/iris/v12"
	"github.com/russross/blackfriday/v2"
)

const GROQ_API_URL = "https://api.groq.com/v1/chat/completions"
const API_KEY = "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"

type RequestBody struct {
	Prompt string `json:"prompt"`
}

type GroqResponse struct {
	Choices []struct {
		Text string `json:"text"`
	} `json:"choices"`
}

func callGroqAPI(prompt string) (string, error) {
	requestBody, _ := json.Marshal(map[string]string{"prompt": prompt})

	req, _ := http.NewRequest("POST", GROQ_API_URL, bytes.NewBuffer(requestBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+API_KEY)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	var groqResponse GroqResponse
	json.Unmarshal(body, &groqResponse)

	if len(groqResponse.Choices) > 0 {
		return groqResponse.Choices[0].Text, nil
	}

	return "", fmt.Errorf("no response from Groq")
}

func main() {
	app := iris.New()

	app.Post("/ask", func(ctx iris.Context) {
		var req RequestBody
		if err := ctx.ReadJSON(&req); err != nil {
			ctx.StatusCode(iris.StatusBadRequest)
			return
		}

		response, err := callGroqAPI(req.Prompt)
		if err != nil {
			ctx.StatusCode(iris.StatusInternalServerError)
			return
		}

		ctx.JSON(iris.Map{"response": string(blackfriday.Run([]byte(response)))})
	})

	app.Listen(":8080")
}
