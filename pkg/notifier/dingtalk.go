package notifier

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	mylog "github.com/langchou/informer/pkg/log"
)

type DingTalkNotifier struct {
	token  string
	secret string
}

func NewDingTalkNotifier(token, secret string) *DingTalkNotifier {
	return &DingTalkNotifier{
		token:  token,
		secret: secret,
	}
}

func (n *DingTalkNotifier) sign(timestamp int64) string {
	stringToSign := fmt.Sprintf("%d\n%s", timestamp, n.secret)
	h := hmac.New(sha256.New, []byte(n.secret))
	h.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func (n *DingTalkNotifier) SendNotification(title, message string, atMobiles []string) error {
	timestamp := time.Now().UnixMilli()
	sign := n.sign(timestamp)

	webhook := fmt.Sprintf("https://oapi.dingtalk.com/robot/send?access_token=%s&timestamp=%d&sign=%s",
		n.token, timestamp, url.QueryEscape(sign))

	if len(atMobiles) > 0 {
		message = message + "\n\n"
		for _, mobile := range atMobiles {
			message = message + "@" + mobile + " "
		}
	}

	content := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"title": title,
			"text":  message,
		},
	}

	if len(atMobiles) > 0 {
		content["at"] = map[string]interface{}{
			"atMobiles": atMobiles,
			"isAtAll":   false,
		}
		mylog.Debug(fmt.Sprintf("钉钉通知将@手机号: %v", atMobiles))
	} else {
		mylog.Debug("钉钉通知不包含@")
	}

	jsonData, err := json.Marshal(content)
	if err != nil {
		mylog.Error("序列化消息失败", "error", err)
		return err
	}

	// 添加调试日志，打印完整的JSON请求体
	prettyJSON, _ := json.MarshalIndent(content, "", "  ")
	mylog.Debug(fmt.Sprintf("发送到钉钉的完整消息体:\n%s", string(prettyJSON)))

	resp, err := http.Post(webhook, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		mylog.Error("发送钉钉消息失败", "error", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("钉钉API返回非200状态码: %d", resp.StatusCode)
		mylog.Error("发送钉钉消息失败", "error", err)
		return err
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		mylog.Error("解析钉钉响应失败", "error", err)
		return err
	}

	// 添加调试日志，打印钉钉API的响应
	respJSON, _ := json.MarshalIndent(result, "", "  ")
	mylog.Debug(fmt.Sprintf("钉钉API响应:\n%s", string(respJSON)))

	if errCode, ok := result["errcode"].(float64); !ok || errCode != 0 {
		err = fmt.Errorf("钉钉API返回错误: %v", result["errmsg"])
		mylog.Error("发送钉钉消息失败", "error", err)
		return err
	}

	mylog.Info("成功发送钉钉消息")
	return nil
}

func (n *DingTalkNotifier) ReportError(title, message string) error {
	errorMessage := fmt.Sprintf("❌ **错误报告**\n\n**类型**: %s\n\n**详情**: %s", title, message)
	return n.SendNotification("系统错误", errorMessage, nil)
}

// SendTextNotification 发送text类型消息，更好地支持@功能
func (n *DingTalkNotifier) SendTextNotification(title, message string, atMobiles []string) error {
	timestamp := time.Now().UnixMilli()
	sign := n.sign(timestamp)

	webhook := fmt.Sprintf("https://oapi.dingtalk.com/robot/send?access_token=%s&timestamp=%d&sign=%s",
		n.token, timestamp, url.QueryEscape(sign))

	// 在消息前添加标题
	textMessage := message

	content := map[string]interface{}{
		"msgtype": "text",
		"text": map[string]string{
			"content": textMessage,
		},
	}

	if len(atMobiles) > 0 {
		content["at"] = map[string]interface{}{
			"atMobiles": atMobiles,
			"isAtAll":   false,
		}
		mylog.Debug(fmt.Sprintf("钉钉文本通知将@手机号: %v", atMobiles))
	} else {
		mylog.Debug("钉钉文本通知不包含@")
	}

	jsonData, err := json.Marshal(content)
	if err != nil {
		mylog.Error("序列化消息失败", "error", err)
		return err
	}

	// 添加调试日志，打印完整的JSON请求体
	prettyJSON, _ := json.MarshalIndent(content, "", "  ")
	mylog.Debug(fmt.Sprintf("发送到钉钉的完整文本消息体:\n%s", string(prettyJSON)))

	resp, err := http.Post(webhook, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		mylog.Error("发送钉钉消息失败", "error", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("钉钉API返回非200状态码: %d", resp.StatusCode)
		mylog.Error("发送钉钉消息失败", "error", err)
		return err
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		mylog.Error("解析钉钉响应失败", "error", err)
		return err
	}

	// 添加调试日志，打印钉钉API的响应
	respJSON, _ := json.MarshalIndent(result, "", "  ")
	mylog.Debug(fmt.Sprintf("钉钉API响应:\n%s", string(respJSON)))

	if errCode, ok := result["errcode"].(float64); !ok || errCode != 0 {
		err = fmt.Errorf("钉钉API返回错误: %v", result["errmsg"])
		mylog.Error("发送钉钉消息失败", "error", err)
		return err
	}

	mylog.Debug("成功发送钉钉文本消息")
	return nil
}
