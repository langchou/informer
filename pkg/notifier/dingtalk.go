package notifier

import (
	"fmt"

	"github.com/blinkbean/dingtalk"
	mylog "github.com/langchou/informer/pkg/log"
)

type DingTalkNotifier struct {
	client *dingtalk.DingTalk
	logger *mylog.Logger
}

func NewDingTalkNotifier(token, secret string, logger *mylog.Logger) *DingTalkNotifier {
	client := dingtalk.InitDingTalkWithSecret(token, secret)
	return &DingTalkNotifier{
		client: client,
		logger: logger,
	}
}

func (n *DingTalkNotifier) SendNotification(title, content string, phoneNumber ...string) {
	msg := fmt.Sprintf("%s\n%s", title, content)
	var err error

	if len(phoneNumber) > 0 && phoneNumber[0] != "" {
		err = n.client.SendTextMessage(msg, dingtalk.WithAtMobiles([]string{phoneNumber[0]}))
	} else {
		err = n.client.SendTextMessage(msg)
	}

	if err != nil {
		n.logger.Error("发送钉钉通知失败:", err)
	} else {
		n.logger.Info("钉钉通知发送成功")
	}
}

func (n *DingTalkNotifier) ReportError(title, content string) {
	n.logger.Error("错误: %s - %s", title, content)
	n.SendNotification("监控程序错误: "+title, content)
}
