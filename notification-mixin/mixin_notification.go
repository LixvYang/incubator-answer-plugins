/*
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package mixin

import (
	"context"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"net/url"
	"sync"

	"github.com/apache/incubator-answer-plugins/util"
	"github.com/apache/incubator-answer/plugin"
	"github.com/fox-one/mixin-sdk-go/v2"
	mixinI18n "github.com/lixvyang/incubator-answer-plugins/notification-mixin/i18n"
	"github.com/segmentfault/pacman/i18n"
	"github.com/segmentfault/pacman/log"
)

//go:embed info.yaml
var Info embed.FS

type Notification struct {
	MixinBot     *mixin.Client
	MixinBotUser *mixin.User

	userMixinConfigMapping map[string]*mixin.User
	userMixinConfigLock    sync.Mutex

	Config          *NotificationConfig
	UserConfigCache *UserConfigCache
}

func init() {
	uc := &Notification{
		Config:                 &NotificationConfig{},
		UserConfigCache:        NewUserConfigCache(),
		userMixinConfigMapping: make(map[string]*mixin.User),
	}
	plugin.Register(uc)
}

func (n *Notification) Info() plugin.Info {
	info := &util.Info{}
	info.GetInfo(Info)

	return plugin.Info{
		Name:        plugin.MakeTranslator(mixinI18n.InfoName),
		SlugName:    info.SlugName,
		Description: plugin.MakeTranslator(mixinI18n.InfoDescription),
		Author:      info.Author,
		Version:     info.Version,
		Link:        info.Link,
	}
}

func (n *Notification) GetNewQuestionSubscribers() (userIDs []string) {
	for userID, conf := range n.UserConfigCache.userConfigMapping {
		if conf.AllNewQuestions {
			userIDs = append(userIDs, userID)
		}
	}
	return userIDs
}

func (n *Notification) Notify(msg plugin.NotificationMessage) {
	log.Debugf("try to send notification %+v", msg)

	if !n.Config.Notification {
		return
	}

	if n.MixinBot == nil {
		if err := n.initMixinBot(); err != nil {
			return
		}
	}

	// if the client id is not the same, we need to re-init the mixin bot
	if n.Config.ClientID != n.MixinBot.ClientID {
		if err := n.initMixinBot(); err != nil {
			return
		}
	}

	userConfig, err := n.getUserConfig(msg.ReceiverUserID)
	if err != nil || userConfig == nil || len(userConfig.MixinID) == 0 {
		log.Debugf("user %s has no config", msg.ReceiverUserID)
		return
	}

	if !n.isNotificationEnabled(msg.Type, userConfig) {
		log.Debugf("user %s not config the notification type %s", msg.ReceiverUserID, msg.Type)
		return
	}

	userMixinConfig, err := n.getUserMixinConfig(msg.ReceiverUserID, userConfig.MixinID)
	if err != nil {
		log.Errorf("get user info failed, userID: %s, error: %v", msg.ReceiverUserID, err)
		return
	}

	notificationTitle := renderNotificationTitle(msg)
	notificationDescription := renderNotificationDescription(msg)
	if len(notificationTitle) == 0 || len(notificationDescription) == 0 {
		log.Debugf("this type of notification will be drop, the type is %s", msg.Type)
		return
	}

	if err := n.sendNotification(notificationTitle, notificationDescription, msg, userMixinConfig); err != nil {
		log.Errorf("send notification failed: %v", err)
	}
}

func (n *Notification) isNotificationEnabled(msgType plugin.NotificationType, userConfig *UserConfig) bool {
	switch msgType {
	case plugin.NotificationNewQuestion:
		return userConfig.AllNewQuestions
	case plugin.NotificationNewQuestionFollowedTag:
		return userConfig.NewQuestionsForFollowingTags
	default:
		return userConfig.InboxNotifications
	}
}

func (n *Notification) getUserMixinConfig(userID, mixinID string) (*mixin.User, error) {
	n.userMixinConfigLock.Lock()
	defer n.userMixinConfigLock.Unlock()

	if userMixinConfig, ok := n.userMixinConfigMapping[userID]; ok {
		return userMixinConfig, nil
	}

	userMixinConfig, err := n.MixinBot.ReadUser(context.Background(), mixinID)
	if err != nil {
		return nil, err
	}

	n.userMixinConfigMapping[userID] = userMixinConfig
	return userMixinConfig, nil
}

func (n *Notification) sendNotification(notificationMsgTitle, notificationMsgDescription string, msg plugin.NotificationMessage, userMixinConfig *mixin.User) error {
	if userMixinConfig == nil || len(userMixinConfig.UserID) == 0 {
		return errors.New("invalid user mixin config")
	}

	if len(notificationMsgTitle) > 36 {
		notificationMsgTitle = notificationMsgTitle[:33] + "..."
	}
	if len(notificationMsgDescription) > 1024 {
		notificationMsgDescription = notificationMsgDescription[:1021] + "..."
	}

	card := &mixin.AppCardMessage{
		AppID:       n.MixinBot.ClientID,
		Title:       notificationMsgTitle,
		Description: notificationMsgDescription,
		IconURL:     n.MixinBotUser.App.IconURL,
		Shareable:   true,
	}
	n.fillCardAction(card, msg)

	if len(card.Action) == 0 {
		return errors.New("the card action is empty")
	}

	parsedAction, err := url.Parse(card.Action)
	if err != nil {
		return err
	}

	if parsedAction.Scheme != "https" || net.ParseIP(parsedAction.Host).IsPrivate() {
		card.Action = "https://mixin.one"
	}

	cardBytes, err := json.Marshal(card)
	if err != nil {
		return err
	}

	cardBase64code := base64.StdEncoding.EncodeToString(cardBytes)
	return n.MixinBot.SendMessage(context.Background(), &mixin.MessageRequest{
		ConversationID: mixin.UniqueConversationID(n.MixinBot.ClientID, userMixinConfig.UserID),
		RecipientID:    userMixinConfig.UserID,
		MessageID:      mixin.RandomTraceID(),
		Category:       mixin.MessageCategoryAppCard,
		Data:           cardBase64code,
	})
}

func (n *Notification) initMixinBot() error {
	client, err := mixin.NewFromKeystore(&mixin.Keystore{
		ClientID:          n.Config.ClientID,
		SessionID:         n.Config.SessionID,
		ServerPublicKey:   n.Config.ServerPublicKey,
		SessionPrivateKey: n.Config.SessionPrivateKey,
	})
	if err != nil {
		log.Errorf("init mixin bot failed: %v", err)
		return err
	}

	me, err := client.UserMe(context.Background())
	if err != nil {
		log.Errorf("get mixin bot info failed: %v", err)
		return err
	}

	if me.App == nil {
		log.Error("use a bot keystore instead")
		return errors.New("invalid bot keystore")
	}

	n.MixinBot = client
	n.MixinBotUser = me
	return nil
}

func renderNotificationDescription(msg plugin.NotificationMessage) string {
	lang := i18n.Language(msg.ReceiverLang)
	return plugin.TranslateWithData(lang, getDescriptionTemplate(msg.Type), msg)
}

func renderNotificationTitle(msg plugin.NotificationMessage) string {
	lang := i18n.Language(msg.ReceiverLang)
	return plugin.TranslateWithData(lang, getTitleTemplate(msg.Type), msg)
}

func getDescriptionTemplate(msgType plugin.NotificationType) string {
	switch msgType {
	case plugin.NotificationUpdateQuestion:
		return mixinI18n.TplUpdateQuestionDescription
	case plugin.NotificationAnswerTheQuestion:
		return mixinI18n.TplAnswerTheQuestionDescription
	case plugin.NotificationUpdateAnswer:
		return mixinI18n.TplUpdateAnswerDescription
	case plugin.NotificationAcceptAnswer:
		return mixinI18n.TplAcceptAnswerDescription
	case plugin.NotificationCommentQuestion:
		return mixinI18n.TplCommentQuestionDescription
	case plugin.NotificationCommentAnswer:
		return mixinI18n.TplCommentAnswerDescription
	case plugin.NotificationReplyToYou:
		return mixinI18n.TplReplyToYouDescription
	case plugin.NotificationMentionYou:
		return mixinI18n.TplMentionYouDescription
	case plugin.NotificationInvitedYouToAnswer:
		return mixinI18n.TplInvitedYouToAnswerDescription
	case plugin.NotificationNewQuestion, plugin.NotificationNewQuestionFollowedTag:
		return mixinI18n.TplNewQuestionDescription
	default:
		return ""
	}
}

func getTitleTemplate(msgType plugin.NotificationType) string {
	switch msgType {
	case plugin.NotificationUpdateQuestion:
		return mixinI18n.TplUpdateQuestionTitle
	case plugin.NotificationAnswerTheQuestion:
		return mixinI18n.TplAnswerTheQuestionTitle
	case plugin.NotificationUpdateAnswer:
		return mixinI18n.TplUpdateAnswerTitle
	case plugin.NotificationAcceptAnswer:
		return mixinI18n.TplAcceptAnswerTitle
	case plugin.NotificationCommentQuestion:
		return mixinI18n.TplCommentQuestionTitle
	case plugin.NotificationCommentAnswer:
		return mixinI18n.TplCommentAnswerTitle
	case plugin.NotificationReplyToYou:
		return mixinI18n.TplReplyToYouTitle
	case plugin.NotificationMentionYou:
		return mixinI18n.TplMentionYouTitle
	case plugin.NotificationInvitedYouToAnswer:
		return mixinI18n.TplInvitedYouToAnswerTitle
	case plugin.NotificationNewQuestion, plugin.NotificationNewQuestionFollowedTag:
		return mixinI18n.TplNewQuestionTitle
	default:
		return ""
	}
}

func (n *Notification) fillCardAction(card *mixin.AppCardMessage, msg plugin.NotificationMessage) {
	switch msg.Type {
	case plugin.NotificationUpdateQuestion, plugin.NotificationInvitedYouToAnswer, plugin.NotificationNewQuestion, plugin.NotificationNewQuestionFollowedTag:
		card.Action = msg.QuestionUrl
	case plugin.NotificationAnswerTheQuestion, plugin.NotificationUpdateAnswer, plugin.NotificationAcceptAnswer:
		card.Action = msg.AnswerUrl
	case plugin.NotificationCommentQuestion, plugin.NotificationCommentAnswer, plugin.NotificationReplyToYou, plugin.NotificationMentionYou:
		card.Action = msg.CommentUrl
	default:
		log.Debugf("this type of notification will be drop, the type is %s", msg.Type)
	}
}
