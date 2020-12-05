package main

type (
	TmeConf struct {
		BotId  string `json:"bot_id"`
		ChatId int64  `json:"chat_id"`
	}

	TmeMessage struct {
		ChatId              int64  `json:"chat_id"`
		Text                string `json:"text"`
		DisableNotification bool   `json:"disable_notification"`
	}
)
