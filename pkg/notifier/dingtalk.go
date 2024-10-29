package notifier

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/blinkbean/dingtalk"
	mylog "github.com/langchou/informer/pkg/log"
)

type DingTalkNotifier struct {
	client       *dingtalk.DingTalk
	logger       *mylog.Logger
	messageQueue []Message
	mu           sync.Mutex
	ticker       *time.Ticker
}

type Message struct {
	Title        string
	Content      string
	PhoneNumbers []string
	Time         time.Time
}

func NewDingTalkNotifier(token, secret string, logger *mylog.Logger) *DingTalkNotifier {
	client := dingtalk.InitDingTalkWithSecret(token, secret)
	n := &DingTalkNotifier{
		client:       client,
		logger:       logger,
		messageQueue: make([]Message, 0),
		ticker:       time.NewTicker(3 * time.Second), // 每3秒检查一次消息队列
	}

	// 启动消息处理协程
	go n.processMessages()

	return n
}

func (n *DingTalkNotifier) processMessages() {
	for range n.ticker.C {
		n.mu.Lock()
		if len(n.messageQueue) == 0 {
			n.mu.Unlock()
			continue
		}

		// 获取所有待发送的消息
		messages := n.messageQueue
		n.messageQueue = make([]Message, 0)
		n.mu.Unlock()

		// 合并消息
		var combinedContent strings.Builder
		var allPhoneNumbers []string
		phoneNumbersMap := make(map[string]bool)

		for i, msg := range messages {
			// 添加分隔线
			if i > 0 {
				combinedContent.WriteString("\n----------------------------------------\n")
			}
			combinedContent.WriteString(fmt.Sprintf("%s\n%s", msg.Title, msg.Content))

			// 收集所有需要@的手机号，去重
			for _, phone := range msg.PhoneNumbers {
				if !phoneNumbersMap[phone] {
					phoneNumbersMap[phone] = true
					allPhoneNumbers = append(allPhoneNumbers, phone)
				}
			}
		}

		// 发送合并后的消息
		var err error
		if len(allPhoneNumbers) > 0 {
			err = n.client.SendTextMessage(combinedContent.String(), dingtalk.WithAtMobiles(allPhoneNumbers))
		} else {
			err = n.client.SendTextMessage(combinedContent.String())
		}

		if err != nil {
			mylog.Error(fmt.Sprintf("发送钉钉通知失败: %v", err))
		} else {
			mylog.Info("钉钉通知发送成功")
		}
	}
}

func (n *DingTalkNotifier) SendNotification(title, content string, phoneNumbers []string) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	// 将消息添加到队列
	n.messageQueue = append(n.messageQueue, Message{
		Title:        title,
		Content:      content,
		PhoneNumbers: phoneNumbers,
		Time:         time.Now(),
	})

	return nil
}

func (n *DingTalkNotifier) ReportError(title, content string) {
	mylog.Error(fmt.Sprintf("错误: %s - %s", title, content))
	n.SendNotification("监控程序错误: "+title, content, nil)
}
