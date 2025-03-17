package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
)

type OllamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type OllamaResponse struct {
	Model     string `json:"model"`
	CreatedAt string `json:"created_at"`
	Response  string `json:"response"`
	Done      bool   `json:"done"`
}

func checkSpam(text string) (string, error) {
	url := os.Getenv("URL")
	if url == "" {
		log.Fatal("Не указан токен бота в переменной URL")
	}

	promptTemplate := `Проанализируй текст на признаки спама и верни результат строго в формате JSON.

Текст для анализа: '%s'

Верни ответ в следующем формате (замени значения, сохраняя структуру):
{
  "spam_indicators": {
    "excessive_caps": false,
    "suspicious_links": false,
    "aggressive_cta": false,
    "unrealistic_promises": false,
    "explanation": "В тексте нет признаков спама"
  },
  "context_usefulness": "Текст содержит полезную информацию",
  "language_features": "Естественный язык общения",
  "classification": {
    "is_spam": false,
    "probability": 5,
    "reason": "Обычное сообщение без признаков спама"
  }
}`

	model := os.Getenv("MODEL")
	if model == "" {
		log.Fatal("Не указан модель в переменной MODEL")
	}

	requestBody := OllamaRequest{
		Model:  model,
		Prompt: fmt.Sprintf(promptTemplate, text),
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("ошибка при создании JSON: %v", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("ошибка при создании запроса: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ошибка при отправке запроса: %v", err)
	}
	defer resp.Body.Close()

	decoder := json.NewDecoder(resp.Body)
	var lastResponse string

	for decoder.More() {
		var response OllamaResponse
		if err := decoder.Decode(&response); err != nil {
			return "", fmt.Errorf("ошибка при декодировании ответа: %v", err)
		}
		lastResponse += response.Response
		if response.Done {
			break
		}
	}

	if len(strings.TrimSpace(lastResponse)) == 0 {
		return "", fmt.Errorf("получен пустой ответ от модели")
	}

	return lastResponse, nil
}

func main() {
	// Загружаем переменные окружения из .env файла
	if err := godotenv.Load(); err != nil {
		log.Printf("Предупреждение: Не удалось загрузить файл .env: %v", err)
	}

	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		log.Fatal("Не указан токен бота в переменной TELEGRAM_BOT_TOKEN")
	}

	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Бот %s успешно запущен", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		// Проверяем только текстовые сообщения
		if update.Message.Text == "" {
			continue
		}

		log.Printf("Получено сообщение от %s: %s", update.Message.From.UserName, update.Message.Text)

		// Анализируем сообщение на спам
		result, err := checkSpam(update.Message.Text)
		if err != nil {
			log.Printf("Ошибка при проверке спама: %v", err)
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Извините, произошла ошибка при анализе сообщения")
			msg.ReplyToMessageID = update.Message.MessageID
			bot.Send(msg)
			continue
		}

		log.Printf("Получен ответ от модели (длина: %d): %s", len(result), result)

		// Очищаем ответ от возможных артефактов
		result = strings.TrimSpace(result)
		if !strings.HasPrefix(result, "{") {
			if idx := strings.Index(result, "{"); idx != -1 {
				result = result[idx:]
			}
		}
		if !strings.HasSuffix(result, "}") {
			if idx := strings.LastIndex(result, "}"); idx != -1 {
				result = result[:idx+1]
			}
		}

		log.Printf("Очищенный ответ: %s", result)

		// Парсим JSON ответ для красивого форматирования
		var jsonResponse map[string]interface{}
		if err := json.Unmarshal([]byte(result), &jsonResponse); err != nil {
			log.Printf("Ошибка при парсинге JSON ответа: %v", err)
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Извините, произошла ошибка при анализе сообщения")
			msg.ReplyToMessageID = update.Message.MessageID
			bot.Send(msg)
			continue
		}

		// Форматируем ответ для пользователя
		classification := jsonResponse["classification"].(map[string]interface{})
		isSpam := classification["is_spam"].(bool)
		probability := classification["probability"].(float64)
		reason := classification["reason"].(string)

		// Устанавливаем порог вероятности для спама (70%)
		const spamThreshold = 70.0
		isHighProbabilitySpam := isSpam && probability >= spamThreshold

		var status string
		var reply string

		if isHighProbabilitySpam {
			status = "❌ СПАМ"
			// Пытаемся удалить сообщение
			deleteMsg := tgbotapi.NewDeleteMessage(update.Message.Chat.ID, update.Message.MessageID)
			if _, err := bot.Request(deleteMsg); err != nil {
				log.Printf("Не удалось удалить спам-сообщение: %v", err)
				reply = fmt.Sprintf(`%s
⚠️ Внимание: обнаружен спам!
Вероятность: %.1f%%
Причина: %s

Не удалось автоматически удалить сообщение. Пожалуйста, убедитесь, что бот имеет права администратора.`, status, probability, reason)
			} else {
				reply = fmt.Sprintf(`%s
✅ Сообщение автоматически удалено
Вероятность: %.1f%%
Причина: %s`, status, probability, reason)
			}
		} else if isSpam {
			status = "⚠️ ВОЗМОЖНО СПАМ"
			reply = fmt.Sprintf(`%s
Вероятность: %.1f%%
Причина: %s

Сообщение не удалено автоматически, так как вероятность спама ниже %d%%`, status, probability, reason, int(spamThreshold))
		}
		// 		else {
		// 			status = "✅ НЕ СПАМ"
		// 			reply = fmt.Sprintf(`%s
		// Вероятность: %.1f%%
		// Причина: %s`, status, probability, reason)
		// 		}

		msg := tgbotapi.NewMessage(update.Message.Chat.ID, reply)
		if !isHighProbabilitySpam {
			msg.ReplyToMessageID = update.Message.MessageID
		}

		if _, err := bot.Send(msg); err != nil {
			log.Printf("Ошибка при отправке ответа: %v", err)
		}
	}
}
