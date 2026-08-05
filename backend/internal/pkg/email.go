package pkg

import (
	"fmt"
	"log/slog"

	"gopkg.in/gomail.v2"
)

type EmailConfig struct {
	Host string
	Port int
	User string
	Pass string
}

// EmailNotifier 邮件通知 — 使用 gomail 库通过 SMTP 发送邮件
// 支持两种邮件模板：低价提醒（HTML表格格式）+ 邮箱验证码
type EmailNotifier struct {
	config EmailConfig
	dialer *gomail.Dialer
}

func NewEmailNotifier(cfg EmailConfig) *EmailNotifier {
	return &EmailNotifier{
		config: cfg,
		dialer: gomail.NewDialer(cfg.Host, cfg.Port, cfg.User, cfg.Pass),
	}
}

// SendPriceAlert 发送低价提醒邮件
func (n *EmailNotifier) SendPriceAlert(to, movieName, cinemaName, hallName,
	showTime string, currentPrice, lowestPrice, targetPrice float64) error {

	subject := fmt.Sprintf("🎬 票价提醒 | %s - %s 现价 ¥%.1f", movieName, cinemaName, currentPrice)
	body := fmt.Sprintf(`
<html>
<body style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto;">
  <h2 style="color: #e74c3c;">🎬 猫眼票价提醒</h2>
  <table style="border-collapse: collapse; width: 100%%;">
    <tr><td style="padding: 8px; border-bottom: 1px solid #eee;"><strong>电影</strong></td><td style="padding: 8px; border-bottom: 1px solid #eee;">%s</td></tr>
    <tr><td style="padding: 8px; border-bottom: 1px solid #eee;"><strong>影院</strong></td><td style="padding: 8px; border-bottom: 1px solid #eee;">%s</td></tr>
    <tr><td style="padding: 8px; border-bottom: 1px solid #eee;"><strong>影厅</strong></td><td style="padding: 8px; border-bottom: 1px solid #eee;">%s</td></tr>
    <tr><td style="padding: 8px; border-bottom: 1px solid #eee;"><strong>场次</strong></td><td style="padding: 8px; border-bottom: 1px solid #eee;">%s</td></tr>
    <tr><td style="padding: 8px; border-bottom: 1px solid #eee; color: #e74c3c;"><strong>当前票价</strong></td><td style="padding: 8px; border-bottom: 1px solid #eee; color: #e74c3c; font-size: 18px;"><strong>¥%.1f</strong></td></tr>
    <tr><td style="padding: 8px; border-bottom: 1px solid #eee;"><strong>历史最低</strong></td><td style="padding: 8px; border-bottom: 1px solid #eee;">¥%.1f</td></tr>
  </table>
  <p style="color: #999; font-size: 12px; margin-top: 20px;">
    此邮件由猫眼服务中台自动发送，您的目标价格：≤ ¥%.1f
  </p>
</body>
</html>`, movieName, cinemaName, hallName, showTime, currentPrice, lowestPrice, targetPrice)

	m := gomail.NewMessage()
	m.SetHeader("From", n.config.User)
	m.SetHeader("To", to)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", body)

	if err := n.dialer.DialAndSend(m); err != nil {
		slog.Error("send email failed", "to", to, "error", err)
		return err
	}
	slog.Info("email sent", "to", to, "subject", subject)
	return nil
}
