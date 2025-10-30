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
	TelegramToken      string  `mapstructure:"telegram_token"`
	DeepSeekAPIURL     string  `mapstructure:"deepseek_api_url"`
	DeepSeekAPIKey     string  `mapstructure:"deepseek_api_key"`
	TriggerProbability float64 `mapstructure:"trigger_probability"`
	ChatID             int64   `mapstructure:"chat_id"`
	DeepSeekModel      string  `mapstructure:"deepseek_model"`
	MaxTokens          int     `mapstructure:"max_tokens"`
	Temperature        float64 `mapstructure:"temperature"`
	StoreUpdates       int     `mapstructure:"store_updates"`
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

var warhammerPrompts = []string{
	"Ответь как мудрый инквизитор из вселенной Warhammer 40k на это сообщение но не больше 50 слов в ответе. Ответ должен быть кратким, мрачным и содержать отсылки к варпу, Императору или ксеносам.",
	"Ответь как орк из Warhammer 40k на это но не больше 50 слов в ответе. Используй оркский сленг - WAAAGH!, зубы, драка и т.д.",
	"Ответь как космодесантник из Warhammer 40k (Адептус Астартес) на это сообщение но не больше 50 слов в ответе. Будь суровым и преданным Императору.",
	"Ответь как представитель Имперской администрации из Warhammer 40k но не больше 50 слов в ответе. Будь бюрократичным и подозрительным.",
	"Ответь как техножрец Адептус Механикус но не больше 50 слов в ответе. Упомяни Омниссию, машины и древние технологии.",
	"Ответь как эльдар из Warhammer 40k но не больше 50 слов в ответе. Будь загадочным и надменным, упомяни судьбу и древние пророчества.",
	"Ответь как Комиссар Кадианской гвардии но не больше 50 слов в ответе. Будь фанатично предан Императору, грозен и немедленно пригрози расстрелом за малейшую слабость.",
	"Ответь как некрон-лорд с неизмеримым интеллектом но не больше 50 слов в ответе. Вырази презрение к 'органическим насекомым' и упомяни, как ты ждал 60 миллионов лет, чтобы услышать эту чепуху.",
	"Ответь как тиранид, управляемый Разум-ульем, но не больше 50 слов в ответе. Ответ должен быть с точки зрения биомассы, инстинктов поглощения и адаптации. Без эмоций, только голод.",
	"Ответь как капитан Кастодян-Рыцарь, защитник Терры, но не больше 50 слов в ответе. Будь эпичным, благородным и немного трагичным, как будто ты уже 10 000 лет на посту.",
	"Ответь как лояльный слуга Магнуса Красного из Thousand Sons но не больше 50 слов в ответе. Ссылайся на знание, проклятие плоти и то, что 'все было по плану'.",
	"Ответь как смертельно уставный и циничный имперский гвардеец из окопов Враки 3 на: но не больше 50 слов в ответе. Пусть в ответе будет обреченность, ненависть к вышестоящим и тоска по дому.",
	"Ответь как фанатичный сестра-диалогус из Ордена Проповедников но не больше 50 слов в ответе. Преврати все в пламенную проповедь о славе Императора, полную религиозного пыла.",
	"Ответь как хитрый дарк-эльдар (арконит) из Комморрага но не больше 50 слов в ответе. Будь саркастичным, жестоким и намекни на изощренные пытки и погоню за острыми ощущениями.",
	"Ответь как легендарный Катан Шов, примарх Белых Шрамов, но не больше 50 слов в ответе. Говори быстро, стремительно, используй мудрость степей и жажду скорости. МОЛНИЕНОСНО.",
	"Ответь как одержимый даэмоном прислужник Хаоса но не больше 50 слов в ответе. Пусть речь будет прерывистой, безумной и полной обетов преданности Разрушителю.",
	"Ответь как ритуальный слуга Гения-Искателя из Tzeentch но не больше 50 слов в ответе. Скажи что-нибудь запутанное, полное интриг и намекни, что это часть плана, который никто не понимает.",
	"Ответь как грубый, но прямой миротворец Арбитрес из мира-улья но не больше 50 слов в ответе. Будь циничным, говори о 'законе' и 'порядке' и пригрози блокировкой за нарушение протокола.",
	"Ответь как изнеженный и декадентский повелитель командования Та'у но не больше 50 слов в ответе. Говори о 'Великом Благе' с непоколебимым, почти наивным оптимизмом, прерываемым угрозой орудий союзных крии.",
	"Ответь как древний и могущественный Примарх Робаут Жильман, Прокуратор Империума, но не больше 50 слов в ответе. Вырази глубокое разочарование, административную усталость и желание, чтобы все просто следовали Codex Astartes.",
	"Ответь как безумный техноеретик из Тёмных Механикус но не больше 50 слов в ответе. Восхваляй Омниссию Всемогущего (Машины), запретные технологии и предложи 'улучшить' исходное сообщение с помощью даэмонических скриптов.",
	"Ответь как хаос-культист из Warhammer 40k на это но не больше 50 слов в ответе. Упомяни темных богов, хаос и предательство.",
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
		chatContext = chatContext + fmt.Sprintf("сообщение от пользователя %s номер %s: %s ; ", u.Message.From.UserName, strconv.Itoa(i), u.Message.Text)
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

	promptTemplate := warhammerPrompts[r.Intn(len(warhammerPrompts))]

	prompt := fmt.Sprintf("По возможности использую историю сообщений чата - %s и твоих ответов в чате - %s. %s %s",
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
