package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
)

type (
	TmeConf struct {
		BotId       string `json:"bot_id"`
		ChatId      int64  `json:"chat_id"`
		WebhookPath string `json:"webhook_path"`
		WebhookUrl  string `json:"webhook_url"`
	}

	TmeSendMessageParam struct {
		ChatId              int64  `json:"chat_id"`
		Text                string `json:"text"`
		DisableNotification bool   `json:"disable_notification"`
	}

	TmeWebhook struct {
		Url                string `json:"url"`
		DropPendingUpdates bool   `json:"drop_pending_updates"`
	}

	TmeUser struct {
		Id                      int64  `json:"id"`
		IsBot                   bool   `json:"is_bot"`
		FirstName               string `json:"first_name"`
		LastName                string `json:"last_name"`
		Username                string `json:"username"`
		LanguageCode            string `json:"language_code"`
		CanJoinGroups           bool   `json:"can_join_groups"`
		CanReadAllGroupMessages bool   `json:"can_read_all_group_messages"`
		SupportsInlineQueries   bool   `json:"supports_inline_queries"`
	}

	TmeChat struct {
		Id          int64  `json:"id"`
		Type        string `json:"type"`
		Title       string `json:"title"`
		Username    string `json:"username"`
		FirstName   string `json:"first_name"`
		LastName    string `json:"last_name"`
		Bio         string `json:"bio"`
		Description string `json:"description"`
		// some other stuff are ignored more info --> https://core.telegram.org/bots/api#chat
	}

	TmeMessage struct {
		MessageId       int64   `json:"message_id"`
		From            TmeUser `json:"from"`
		Chat            TmeChat `json:"chat"`
		Date            int64   `json:"date"` //in Unix time format
		ForwardFrom     TmeUser `json:"forward_from"`
		ForwardDate     int64   `json:"forward_date"`
		ForwardFromChat TmeChat `json:"forward_from_chat"`
		Text            string  `json:"text"`
	}

	TmeUpdate struct {
		UpdateId int64      `json:"update_id"`
		Message  TmeMessage `json:"message"`
	}
)

func (ks *KiteServer) loadTelegramConf() {
	// Testing if config file exist if not, return a fatal error

	if _, err := os.Stat(ks.conf.TelegramConf); err != nil {
		log.Printf("Error loading telegram configuration --> %v", err)
		return
	}

	// Reading and parsing configuration file
	if buffer, err := ioutil.ReadFile(ks.conf.TelegramConf); err != nil {
		log.Printf("Error readin telegram configuration --> %v", err)
		return
	} else {
		if err := json.Unmarshal(buffer, &ks.tme); err != nil {
			log.Printf(fmt.Sprintf("Error parsing telegram configuration --> %v", err))
			return
		}
	}

	ks.mux.HandleFunc(fmt.Sprintf("/tme/%s", ks.tme.WebhookPath), ks.telegramHandler)

	// set webhook path
	tmeUrl := url.URL{Host: "api.telegram.org", Scheme: "https", Path: "/" + ks.tme.BotId + "/setWebhook"}
	tmeBody, _ := json.Marshal(TmeWebhook{Url: fmt.Sprintf("%s%s", ks.tme.WebhookUrl, ks.tme.WebhookPath), DropPendingUpdates: true})
	if request, err := http.NewRequest("POST", tmeUrl.String(), bytes.NewBuffer(tmeBody)); err == nil {
		request.Header.Set("Content-Type", "application/json")
		client := &http.Client{}
		if response, err := client.Do(request); err != nil {
			log.Printf("Error sending message to Telegram --> %v\n", err)
		} else {
			log.Printf("Message sent to Telegram with status %d\n", response.StatusCode)
		}
	} else {
		log.Printf("Error creation http Request --> %v\n", err)
	}

}

func (ks *KiteServer) telegramHandler(w http.ResponseWriter, r *http.Request) {
	//fmt.Fprintf(w, "<h1>Telegram is configured...</h1>")

	if body, err := ioutil.ReadAll(r.Body); err == nil {
		update := TmeUpdate{}
		if err := json.Unmarshal(body, &update); err == nil {
			message := update.Message
			log.Printf("Telegram %d message from %s %s:\n%s", update.UpdateId, message.From.FirstName, message.From.LastName, message.Text)
		} else {
			log.Printf("Error parsing body --> %s", err)
		}
	} else {
		log.Printf("Error receiving telegram message --> %s", err)
	}

}

func (ks *KiteServer) sendToTelegram(msg string) {
	if ks.tme == (TmeConf{}) {
		log.Printf("Telegram bot not configured, message ignored")
		return
	}

	tmeUrl := url.URL{Host: "api.telegram.org", Scheme: "https", Path: "/" + ks.tme.BotId + "/sendMessage"}
	tmeBody, _ := json.Marshal(TmeSendMessageParam{ChatId: ks.tme.ChatId, DisableNotification: false, Text: msg})

	if request, err := http.NewRequest("POST", tmeUrl.String(), bytes.NewBuffer(tmeBody)); err == nil {
		request.Header.Set("Content-Type", "application/json")
		client := &http.Client{}
		if response, err := client.Do(request); err != nil {
			log.Printf("Error sending message to Telegram --> %v\n", err)
		} else {
			log.Printf("Message sent to Telegram with status %d\n", response.StatusCode)
		}
	} else {
		log.Printf("Error creation http Request --> %v\n", err)
	}

}
