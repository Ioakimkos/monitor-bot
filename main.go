package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	GROUP_ID       = int64(-1003968418152)
	RUSSIAN_SERVER = "http://33.130.151.210"
)

type Metrics struct {
	Hostname   string `json:"hostname"`
	CPU        string `json:"cpu"`
	RAM        string `json:"ram"`
	Disk       string `json:"disk"`
	Nginx      string `json:"nginx"`
	Containers string `json:"containers"`
}

type LogsResponse struct {
	Logs string `json:"logs"`
}

var lastMetrics Metrics
var prevContainers = map[string]bool{}
var bot *tgbotapi.BotAPI

// допустимые имена docker-контейнеров
var containerNameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,127}$`)

func checkAlerts(metrics Metrics) {
	lines := strings.Split(metrics.Containers, "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		isUp := strings.HasPrefix(line, "✅")
		name := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "✅ "), "🔴 "))
		wasUp, known := prevContainers[name]

		if known && wasUp && !isUp {
			msg := tgbotapi.NewMessage(GROUP_ID, fmt.Sprintf("🔴 %s: контейнер *%s* упал!", metrics.Hostname, name))
			msg.ParseMode = "Markdown"
			bot.Send(msg)
		} else if known && !wasUp && isUp {
			msg := tgbotapi.NewMessage(GROUP_ID, fmt.Sprintf("✅ %s: контейнер *%s* восстановился!", metrics.Hostname, name))
			msg.ParseMode = "Markdown"
			bot.Send(msg)
		}
		prevContainers[name] = isUp
	}
}

func getLogs(containerName string) string {
	if !containerNameRe.MatchString(containerName) {
		return "Некорректное имя контейнера"
	}
	out, err := exec.Command("docker", "logs", "--tail=50", containerName).CombinedOutput()
	if err != nil {
		return fmt.Sprintf("Ошибка: %s", err.Error())
	}
	logs := strings.TrimSpace(string(out))
	if logs == "" {
		return "Логи пусты"
	}
	// обрезаем до 4000 символов (лимит Telegram)
	if len(logs) > 4000 {
		logs = "..." + logs[len(logs)-4000:]
	}
	return logs
}

func main() {
	var err error
	token := os.Getenv("TELEGRAM_TOKEN")
	if token == "" {
		log.Fatal("TELEGRAM_TOKEN не задан")
	}
	bot, err = tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Бот запущен: %s", bot.Self.UserName)

	// endpoint для приёма метрик
	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", 405)
			return
		}
		json.NewDecoder(r.Body).Decode(&lastMetrics)
		w.WriteHeader(200)
		log.Printf("Получены метрики от %s", lastMetrics.Hostname)
		checkAlerts(lastMetrics)
	})

	// endpoint для получения логов от российского сервера
	http.HandleFunc("/logs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", 405)
			return
		}
		var req struct {
			ChatID    int64  `json:"chat_id"`
			Container string `json:"container"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ChatID != GROUP_ID {
			http.Error(w, "Forbidden", 403)
			return
		}
		logs := getLogs(req.Container)
		text := fmt.Sprintf("📋 Логи *%s*:\n```\n%s\n```", req.Container, logs)
		msg := tgbotapi.NewMessage(req.ChatID, text)
		msg.ParseMode = "Markdown"
		bot.Send(msg)
		w.WriteHeader(200)
	})

	go http.ListenAndServe(":8080", nil)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		switch update.Message.Command() {
		case "start":
			text := "Привет! Доступные команды:\n" +
				"/status — статус сервера\n" +
				"/logs имя\\_контейнера — последние 50 строк логов"
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, text)
			msg.ParseMode = "Markdown"
			bot.Send(msg)

		case "status":
			if lastMetrics.Hostname == "" {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID,
					"⏳ Данные ещё не получены от сервера")
				bot.Send(msg)
				continue
			}
			text := fmt.Sprintf(
				"🖥 *%s*\n\nCPU: %s%%\nRAM: %s%%\nDISK: %s\n\nNginx: %s\n\n🐳 Контейнеры:\n%s",
				lastMetrics.Hostname,
				lastMetrics.CPU,
				lastMetrics.RAM,
				lastMetrics.Disk,
				lastMetrics.Nginx,
				lastMetrics.Containers,
			)
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, text)
			msg.ParseMode = "Markdown"
			bot.Send(msg)

		case "logs":
			if update.Message.Chat.ID != GROUP_ID {
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Команда доступна только в группе мониторинга"))
				continue
			}
			args := update.Message.CommandArguments()
			if args == "" {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID,
					"Укажи имя контейнера: /logs имя_контейнера")
				bot.Send(msg)
				continue
			}
			waiting := tgbotapi.NewMessage(update.Message.Chat.ID,
				fmt.Sprintf("⏳ Получаю логи *%s*...", args))
			waiting.ParseMode = "Markdown"
			bot.Send(waiting)

			logs := getLogs(args)
			text := fmt.Sprintf("📋 Логи *%s*:\n```\n%s\n```", args, logs)
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, text)
			msg.ParseMode = "Markdown"
			bot.Send(msg)
		}
	}
}
