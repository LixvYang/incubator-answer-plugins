package mixin

import (
	"encoding/json"

	"github.com/lixvyang/incubator-answer-plugins/notification-mixin/i18n"
	"github.com/apache/incubator-answer/plugin"
)

type NotificationConfig struct {
	Notification      bool   `json:"notification"`
	ClientID          string `json:"client_id" mapstructure:"client_id" yaml:"client_id"`
	SessionID         string `json:"session_id" mapstructure:"session_id" yaml:"session_id"`
	ServerPublicKey   string `json:"server_public_key" mapstructure:"server_public_key" yaml:"server_public_key"`
	SessionPrivateKey string `json:"session_private_key" mapstructure:"session_private_key" yaml:"session_private_key"`
}

func (n *Notification) ConfigFields() []plugin.ConfigField {
	return []plugin.ConfigField{
		{
			Name:        "notification",
			Type:        plugin.ConfigTypeSwitch,
			Title:       plugin.MakeTranslator(i18n.ConfigNotificationTitle),
			Description: plugin.MakeTranslator(i18n.ConfigNotificationDescription),
			UIOptions: plugin.ConfigFieldUIOptions{
				Label: plugin.MakeTranslator(i18n.ConfigNotificationLabel),
			},
			Value: n.Config.Notification,
		},
		{
			Name:        "client_id",
			Type:        plugin.ConfigTypeInput,
			Title:       plugin.MakeTranslator(i18n.ConfigClientIDTitle),
			Description: plugin.MakeTranslator(i18n.ConfigClientIDDescription),
			UIOptions: plugin.ConfigFieldUIOptions{
				Label: plugin.MakeTranslator(i18n.ConfigClientIDLabel),
			},
			Value: n.Config.ClientID,
		},
		{
			Name:        "session_id",
			Type:        plugin.ConfigTypeInput,
			Title:       plugin.MakeTranslator(i18n.ConfigSessionIDTitle),
			Description: plugin.MakeTranslator(i18n.ConfigSessionIDDescription),
			UIOptions: plugin.ConfigFieldUIOptions{
				Label: plugin.MakeTranslator(i18n.ConfigSessionIDLabel),
			},
			Value: n.Config.SessionID,
		},
		{
			Name:        "server_public_key",
			Type:        plugin.ConfigTypeInput,
			Title:       plugin.MakeTranslator(i18n.ConfigServerPublicKeyTitle),
			Description: plugin.MakeTranslator(i18n.ConfigServerPublicKeyDescription),
			UIOptions: plugin.ConfigFieldUIOptions{
				Label: plugin.MakeTranslator(i18n.ConfigServerPublicKeyLabel),
			},
			Value: n.Config.ServerPublicKey,
		},
		{
			Name:        "session_private_key",
			Type:        plugin.ConfigTypeInput,
			Title:       plugin.MakeTranslator(i18n.ConfigSessionPrivateKeyTitle),
			Description: plugin.MakeTranslator(i18n.ConfigSessionPrivateKeyDescription),
			UIOptions: plugin.ConfigFieldUIOptions{
				Label: plugin.MakeTranslator(i18n.ConfigSessionPrivateKeyLabel),
			},
			Value: n.Config.SessionPrivateKey,
		},
	}
}

func (n *Notification) ConfigReceiver(config []byte) error {
	c := &NotificationConfig{}
	_ = json.Unmarshal(config, c)
	n.Config = c
	return nil
}
