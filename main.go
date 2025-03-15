package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

// Структура для запроса к Ollama API
type OllamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

// Структура для ответа от Ollama API
type OllamaResponse struct {
	Model     string `json:"model"`
	CreatedAt string `json:"created_at"`
	Response  string `json:"response"`
	Done      bool   `json:"done"`
}

func main() {
	// URL API Ollama
	url := "http://localhost:11434/api/generate"

	// Создаем запрос
	requestBody := OllamaRequest{
		Model: "qwen2.5-coder:7b", // Меняем модель на более подходящую
		Prompt: `Ты — система автоматической фильтрации спама. Твоя задача - проанализировать предоставленный текст и определить, является ли он спамом. Используй следующие критерии:
Наличие типичных спамных признаков: избыточное использование заглавных букв, повторяющиеся слова или фразы, подозрительные ссылки, призывы к немедленным действиям (например, "купи сейчас', "перейди по ссылке").
Контекст и полезность: текст несет полезную информацию или выглядит как бессмысленный набор слов/реклама?
Языковые особенности: наличие грамматических ошибок, неестественных конструкций или явных попыток обойти фильтры (например, 'v1аgrа' вместо 'viagra").
Итоговая классификация: спам это или нет, с вероятностью в процентах (например, 90% спам) и кратким объяснением.
Проанализируй следующий текст и предоставь результат в формате JSON:
Поле spam_indicators: объект с булевыми значениями и пояснениями для каждого спамного
признака (например, excessive_caps, suspicious_links).
Поле context_usefulness: строка с оценкой полезности текста.
Поле language_features: строка с анализом языковых особенностей.
Поле classification: объект с полями is_spam (true/false), probability (число от 0до 100), reason
(строка с объяснением).
Текст для анализа: 'ПОКУПАЙ СЕЙЧАС!!! Лучшие цены на телефоны только у нас, переходи по
ссылке: bit.ly/superdeal!!!'`,
	}

	// Преобразуем запрос в JSON
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		fmt.Println("Ошибка при создании JSON:", err)
		return
	}

	// Создаем HTTP-запрос
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Println("Ошибка при создании запроса:", err)
		return
	}

	// Устанавливаем заголовки
	req.Header.Set("Content-Type", "application/json")

	// Отправляем запрос
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Ошибка при отправке запроса:", err)
		return
	}
	defer resp.Body.Close()

	// Читаем ответ построчно
	decoder := json.NewDecoder(resp.Body)
	var lastResponse string

	fmt.Println("Начинаем чтение ответа...")
	fmt.Printf("Статус ответа: %s\n", resp.Status)
	for decoder.More() {
		var response OllamaResponse
		if err := decoder.Decode(&response); err != nil {
			fmt.Printf("Ошибка при декодировании ответа: %v\n", err)
			return
		}
		fmt.Printf("Получен частичный ответ: %+v\n", response)
		lastResponse += response.Response
		if response.Done {
			break
		}
	}

	// Выводим результат
	if lastResponse == "" {
		fmt.Println("Внимание: получен пустой ответ от модели")
	} else {
		fmt.Println("Ответ модели:", lastResponse)
	}
}
