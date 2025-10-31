package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/spf13/viper"
)

type Config struct {
	TelegramToken      string   `mapstructure:"telegram_token"`
	DeepSeekAPIURL     string   `mapstructure:"deepseek_api_url"`
	DeepSeekAPIKey     string   `mapstructure:"deepseek_api_key"`
	TriggerProbability float64  `mapstructure:"trigger_probability"`
	ChatID             int64    `mapstructure:"chat_id"`
	DeepSeekModel      string   `mapstructure:"deepseek_model"`
	MaxTokens          int      `mapstructure:"max_tokens"`
	Temperature        float64  `mapstructure:"temperature"`
	StoreUpdates       int      `mapstructure:"store_updates"`
	Prompts            []string `mapstructure:"prompts"`
}

type deepSeekMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type deepSeekRequest struct {
	Model       string            `json:"model"`
	Messages    []deepSeekMessage `json:"messages"`
	MaxTokens   int               `json:"max_tokens"`
	Temperature float64           `json:"temperature"`
}

type deepSeekResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

func loadConfig() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	viper.SetDefault("deepseek_api_url", "https://api.deepseek.com/v1/chat/completions")
	viper.SetDefault("trigger_probability", 0.1)
	viper.SetDefault("deepseek_model", "deepseek-chat")
	viper.SetDefault("max_tokens", 150)
	viper.SetDefault("temperature", 0.8)
	viper.SetDefault("bot_debug", true)
	viper.SetDefault("store_updates", 20)
	viper.SetDefault("prompts", []string{
		"Ответь как мудрый инквизитор из вселенной Warhammer 40k на это сообщение но не больше 50 слов в ответе.",
		"Ответь как орк из Warhammer 40k на это но не больше 50 слов в ответе. ",
		"Ответь как космодесантник из Warhammer 40k (Адептус Астартес) на это сообщение но не больше 50 слов в ответе.",
		"Ответь как представитель Имперской администрации из Warhammer 40k но не больше 50 слов в ответе. ",
		"Ответь как техножрец Адептус Механикус из Warhammer 40k но не больше 50 слов в ответе.",
		"Ответь как эльдар из Warhammer 40k но не больше 50 слов в ответе.",
		"Ответь как Комиссар Кадианской гвардии из Warhammer 40k но не больше 50 слов в ответе.",
		"Ответь как некрон-лорд с неизмеримым интеллектом из Warhammer 40k но не больше 50 слов в ответе.",
		"Ответь как тиранид, управляемый Разум-ульем из Warhammer 40k, но не больше 50 слов в ответе. ",
		"Ответь как капитан Кастодян-Рыцарь, защитник Терры, из Warhammer 40k но не больше 50 слов в ответе. ",
		"Ответь как лояльный слуга Магнуса Красного из Thousand Sons из Warhammer 40k но не больше 50 слов в ответе. ",
		"Ответь как смертельно уставный и циничный имперский гвардеец из окопов Враки 3 из Warhammer 40k но не больше 50 слов в ответе. ",
		"Ответь как фанатичный сестра-диалогус из Ордена Проповедников из Warhammer 40k но не больше 50 слов в ответе.",
		"Ответь как хитрый дарк-эльдар (арконит) из Комморрага из Warhammer 40k но не больше 50 слов в ответе. ",
		"Ответь как легендарный Катан Шов, примарх Белых Шрамов из Warhammer 40k, но не больше 50 слов в ответе. ",
		"Ответь как одержимый даэмоном прислужник Хаоса из Warhammer 40k но не больше 50 слов в ответе. ",
		"Ответь как ритуальный слуга Гения-Искателя из Tzeentch из Warhammer 40k но не больше 50 слов в ответе. ",
		"Ответь как грубый, но прямой миротворец Арбитрес из мира-улья из Warhammer 40k но не больше 50 слов в ответе. ",
		"Ответь как изнеженный и декадентский повелитель командования Та'у из Warhammer 40k но не больше 50 слов в ответе.",
		"Ответь как древний и могущественный Примарх Робаут Жильман, Прокуратор Империума, из Warhammer 40k но не больше 50 слов в ответе. ",
		"Ответь как безумный техноеретик из Тёмных Механикус из Warhammer 40k но не больше 50 слов в ответе.",
		"Ответь как хаос-культист из Warhammer 40k на это но не больше 50 слов в ответе. ",
	})

	viper.ReadInConfig()

	viper.SetEnvPrefix("WHBOT")
	viper.AutomaticEnv()

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("unable to decode config into struct: %v", err)
	}

	if config.TelegramToken == "" {
		return nil, fmt.Errorf("telegram_token is required")
	}
	if config.DeepSeekAPIKey == "" {
		return nil, fmt.Errorf("deepseek_api_key is required")
	}
	if len(config.Prompts) == 0 {
		return nil, fmt.Errorf("prompts cant be empty")
	}

	return &config, nil
}

func main() {
	config, err := loadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	bot, err := tgbotapi.NewBotAPI(config.TelegramToken)
	if err != nil {
		log.Panic(err)
	}

	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	var lastUpdates []tgbotapi.Update
	var lastReplies []string
	for update := range updates {
		if update.Message == nil {
			continue
		}

		if config.ChatID != 0 && update.Message.Chat.ID != config.ChatID {
			log.Printf("Message from unauthorized chat: %d", update.Message.Chat.ID)
			continue
		}

		lastUpdates = writeAndRotate(lastUpdates, update, config.StoreUpdates)

		var replyContext string
		for i, u := range lastReplies {
			replyContext = replyContext + fmt.Sprintf("ответ %s: %s ;", strconv.Itoa(i), u)
		}

		reply, err := handleMessage(bot, update.Message, config, lastUpdates, replyContext)
		if err != nil {
			continue
		}
		lastReplies = writeAndRotate(lastReplies, reply, config.StoreUpdates)
	}
}

func sendMessage(bot *tgbotapi.BotAPI, chatID int64, text string, replyTo int) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyToMessageID = replyTo

	if _, err := bot.Send(msg); err != nil {
		log.Printf("Error sending message: %v", err)
	}
}

func handleMessage(bot *tgbotapi.BotAPI, message *tgbotapi.Message,
	config *Config, lastUpdates []tgbotapi.Update, lastResponses string) (reply string, err error) {
	if len(message.Text) < 5 {
		err = fmt.Errorf("Too short text")
		return
	}

	isMentioned := strings.Contains(strings.ToLower(message.Text), "@"+strings.ToLower(bot.Self.UserName))

	var replyTo bool

	if message.ReplyToMessage != nil &&
		message.ReplyToMessage.From != nil &&
		message.ReplyToMessage.From.ID == bot.Self.ID {
		replyTo = true
	}

	if !isMentioned && rand.Float64() > config.TriggerProbability && !replyTo {
		err = fmt.Errorf("Conditions not met")
		return
	}

	var chatContext string
	for i, u := range lastUpdates {
		chatContext = chatContext + fmt.Sprintf("сообщение от пользователя %s номер %s: %s ; ",
			u.Message.From.UserName, strconv.Itoa(i), u.Message.Text)
	}
	chatContext = strings.TrimSpace(chatContext)

	processedText := message.Text
	if isMentioned {
		processedText = strings.ReplaceAll(strings.ToLower(processedText), "@"+strings.ToLower(bot.Self.UserName), "")
		processedText = strings.TrimSpace(processedText)

		if len(processedText) < 3 {
			processedText = "Ты жалкий бот зачем ты существуешь"
		}
		if message.ReplyToMessage != nil {
			processedText = processedText + message.ReplyToMessage.Text
		}
	}
	now := time.Now().UTC().Truncate(time.Hour * 24)
	r := rand.New(rand.NewSource(now.Unix()))

	promptTemplate := config.Prompts[r.Intn(len(config.Prompts))]

	prompt := fmt.Sprintf("По возможности используя историю сообщений чата - %s."+
		"И твоих ответов в чате(старайся быть оригинальным и не повторяться, историят твоих ответов для понимания контекста) - %s."+
		"То как надо отвечать - %s."+
		"Само сообщение на которое нужно ответить - %s",
		chatContext, lastResponses, promptTemplate, processedText)

	log.Println("Request:", prompt)

	response, err := generateDeepSeekResponse(prompt, config)
	if err != nil {
		log.Printf("Error generating response: %v", err)
		fallbackResponses := []string{
			"Мои астропатические способности ослабли...",
			"Варпальные бури мешают связи!",
			"Техножрецы проверяют связь с духом машины...",
		}
		response = fallbackResponses[rand.Intn(len(fallbackResponses))]
	}

	sendMessage(bot, message.Chat.ID, response, message.MessageID)
	return response, nil
}

func generateDeepSeekResponse(prompt string, config *Config) (string, error) {
	requestBody := deepSeekRequest{
		Model: config.DeepSeekModel,
		Messages: []deepSeekMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		MaxTokens:   config.MaxTokens,
		Temperature: config.Temperature,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", config.DeepSeekAPIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.DeepSeekAPIKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var deepSeekResp deepSeekResponse
	if err := json.NewDecoder(resp.Body).Decode(&deepSeekResp); err != nil {
		return "", err
	}

	if deepSeekResp.Error.Message != "" {
		return "", fmt.Errorf("API error: %s", deepSeekResp.Error.Message)
	}

	if len(deepSeekResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	return strings.TrimSpace(deepSeekResp.Choices[0].Message.Content), nil
}

func writeAndRotate[T any](s []T, v T, l int) []T {
	if len(s) < l {
		s = append(s, v)
	} else {
		rotate(s, 1)
		s[(len(s) - 1)] = v
	}
	return s
}

func rotate[T any](s []T, k int) {
	n := len(s)
	if n == 0 {
		return
	}
	k = k % n
	slices.Reverse(s[:k])
	slices.Reverse(s[k:])
	slices.Reverse(s)
}
