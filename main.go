package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// TOKEN бота из BotFather
// Рекомендуется хранить в переменных окружения или secrets management
var botToken = os.Getenv("TELEGRAM_BOT_TOKEN")
var tgId, err = strconv.ParseInt(os.Getenv("TELEGRAM_USER_ID"), 10, 64)

// Разрешенные ID пользователей (ваш ID)
// Можно загружать из файла или БД в продакшене
var allowedUserIDs = map[string]bool{
	tgId: true,
}

func main() {
	if botToken == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN environment variable not set.")
	}

	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = true // Включить дебаг для отладки, отключите в продакшене
	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil { // ignore any non-Message Updates
			continue
		}

		// Проверяем, разрешен ли пользователь
		if _, ok := allowedUserIDs[update.Message.From.ID]; !ok {
			log.Printf("Unauthorized access attempt by user ID: %d (%s)", update.Message.From.ID, update.Message.From.UserName)
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Извините, у вас нет прав для использования этого бота.")
			bot.Send(msg)
			continue
		}

		if update.Message.IsCommand() { // Если это команда
			switch update.Message.Command() {
			case "start":
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("Привет, %s! Я бот для выдачи VPN-конфигов.\nИспользуй /get_vpn чтобы получить новый конфиг.\nИспользуй /help для информации.", update.Message.From.FirstName))
				bot.Send(msg)
			case "help":
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Доступные команды:\n/get_vpn - получить новый VPN-конфиг\n/my_id - узнать свой Telegram ID (для настройки доступа)\nЕсли у вас есть вопросы, свяжитесь с администратором.")
				bot.Send(msg)
			case "my_id":
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("Ваш Telegram ID: `%d`", update.Message.From.ID))
				msg.ParseMode = "MarkdownV2"
				bot.Send(msg)
			case "get_vpn":
				sendVPNTypeKeyboard(bot, update.Message.Chat.ID)
			default:
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Неизвестная команда. Используйте /help.")
				bot.Send(msg)
			}
		} else if update.Message.Text != "" { // Если это обычный текст (для выбора VPN)
			handleVPNChoice(bot, update.Message)
		}
	}
}

// sendVPNTypeKeyboard отправляет клавиатуру с выбором VPN
func sendVPNTypeKeyboard(bot *tgbotapi.BotAPI, chatID int64) {
	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("OpenVPN"),
			tgbotapi.NewKeyboardButton("WireGuard"),
		),
	)
	keyboard.OneTimeKeyboard = true
	keyboard.ResizeKeyboard = true

	msg := tgbotapi.NewMessage(chatID, "Какой тип VPN вы хотите получить?")
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}

// handleVPNChoice обрабатывает выбор VPN и генерирует конфиг
func handleVPNChoice(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	choice := message.Text
	userID := message.From.ID
	clientName := fmt.Sprintf("user_%d_%s_%d", userID, strings.ToLower(choice), time.Now().Unix())
	configFilePath := ""
	scriptPath := ""

	switch choice {
	case "OpenVPN":
		scriptPath = "/usr/local/bin/gen_openvpn_config.sh"
		configFilePath = fmt.Sprintf("/tmp/openvpn_configs/%s/%s.ovpn", clientName, clientName)
	case "WireGuard":
		scriptPath = "/usr/local/bin/gen_wireguard_config.sh"
		configFilePath = fmt.Sprintf("/tmp/wireguard_configs/%s.conf", clientName)
	default:
		msg := tgbotapi.NewMessage(message.Chat.ID, "Неизвестный выбор VPN. Пожалуйста, используйте кнопки.")
		bot.Send(msg)
		return
	}

	processingMsg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("Генерирую конфиг %s для вас, пожалуйста подождите...", choice))
	bot.Send(processingMsg)

	// Выполняем скрипт генерации конфига
	// !!! ВНИМАНИЕ: Убедитесь, что пользователь, от которого работает бот, имеет права на выполнение этих скриптов,
	// !!! включая возможность sudo без пароля, если скрипты требуют root-привилегий.
	cmd := exec.Command(scriptPath, clientName)
	output, err := cmd.CombinedOutput() // Объединяем stdout и stderr
	if err != nil {
		log.Printf("Error executing script %s for client %s: %v\nOutput: %s", scriptPath, clientName, err, string(output))
		errMsg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("Ошибка при генерации конфига: %s. Пожалуйста, свяжитесь с администратором.", strings.TrimSpace(string(output))))
		bot.Send(errMsg)
		return
	}

	// Проверяем, существует ли файл и отправляем его
	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		log.Printf("Config file not found after generation: %s", configFilePath)
		errMsg := tgbotapi.NewMessage(message.Chat.ID, "Произошла ошибка при создании файла конфига. Пожалуйста, попробуйте позже или свяжитесь с администратором.")
		bot.Send(errMsg)
		return
	}

	fileBytes, err := os.ReadFile(configFilePath)
	if err != nil {
		log.Printf("Error reading config file %s: %v", configFilePath, err)
		errMsg := tgbotapi.NewMessage(message.Chat.ID, "Не удалось прочитать файл конфига. Пожалуйста, свяжитесь с администратором.")
		bot.Send(errMsg)
		return
	}

	file := tgbotapi.FileBytes{
		Name:  filepath.Base(configFilePath), // Получаем только имя файла
		Bytes: fileBytes,
	}

	docMsg := tgbotapi.NewDocument(message.Chat.ID, file)
	docMsg.Caption = fmt.Sprintf("Ваш %s конфиг готов! Сохраните его и используйте.", choice)
	if _, err := bot.Send(docMsg); err != nil {
		log.Printf("Error sending document: %v", err)
		// Можно отправить сообщение об ошибке пользователю
	}

	// Очистка: удаляем сгенерированный файл и директорию (если OpenVPN)
	if choice == "OpenVPN" {
		os.Remove(configFilePath)
		os.Remove(filepath.Dir(configFilePath)) // Удаляем директорию
	} else if choice == "WireGuard" {
		os.Remove(configFilePath)
	}
	log.Printf("Successfully sent and cleaned up config for %s", clientName)
}
