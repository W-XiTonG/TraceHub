package mail

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"

	"tracehub/internal/pkg/config"
)

// SendPlain 发送纯文本邮件（UTF-8）。From 建议格式为 "Name <addr@domain>"。
func SendPlain(cfg *config.EmailConfig, to []string, subject, body string) error {
	if cfg.SMTPHost == "" || len(to) == 0 || cfg.From == "" {
		return fmt.Errorf("smtp: missing host, recipients or from")
	}
	port := cfg.SMTPPort
	if port <= 0 {
		port = 587
	}
	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, port)

	var auth smtp.Auth
	if cfg.SMTPUser != "" {
		auth = smtp.PlainAuth("", cfg.SMTPUser, cfg.SMTPPassword, cfg.SMTPHost)
	}

	headers := []string{
		"From: " + cfg.From,
		"To: " + strings.Join(to, ", "),
		"Subject: " + encodeSubject(subject),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"Content-Transfer-Encoding: 8bit",
	}
	msg := strings.Join(headers, "\r\n") + "\r\n\r\n" + body + "\r\n"

	if port == 465 {
		tlsConf := &tls.Config{
			ServerName:         cfg.SMTPHost,
			InsecureSkipVerify: cfg.InsecureSkipVerify,
		}
		conn, err := tls.Dial("tcp", addr, tlsConf)
		if err != nil {
			return err
		}
		defer conn.Close()
		cl, err := smtp.NewClient(conn, cfg.SMTPHost)
		if err != nil {
			return err
		}
		defer cl.Close()
		if auth != nil {
			if ok, _ := cl.Extension("AUTH"); ok {
				if err := cl.Auth(auth); err != nil {
					return err
				}
			}
		}
		if err := cl.Mail(parseAddr(cfg.From)); err != nil {
			return err
		}
		for _, rcpt := range to {
			if err := cl.Rcpt(rcpt); err != nil {
				return err
			}
		}
		w, err := cl.Data()
		if err != nil {
			return err
		}
		if _, err := w.Write([]byte(msg)); err != nil {
			return err
		}
		return w.Close()
	}

	d := net.Dialer{Timeout: 15 * time.Second}
	conn, err := d.Dial("tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	cl, err := smtp.NewClient(conn, cfg.SMTPHost)
	if err != nil {
		return err
	}
	defer cl.Close()

	if cfg.StartTLS {
		tlsConf := &tls.Config{
			ServerName:         cfg.SMTPHost,
			InsecureSkipVerify: cfg.InsecureSkipVerify,
		}
		if ok, _ := cl.Extension("STARTTLS"); ok {
			if err := cl.StartTLS(tlsConf); err != nil {
				return err
			}
		}
	}

	if auth != nil {
		if ok, _ := cl.Extension("AUTH"); ok {
			if err := cl.Auth(auth); err != nil {
				return err
			}
		}
	}
	if err := cl.Mail(parseAddr(cfg.From)); err != nil {
		return err
	}
	for _, rcpt := range to {
		if err := cl.Rcpt(rcpt); err != nil {
			return err
		}
	}
	w, err := cl.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write([]byte(msg)); err != nil {
		return err
	}
	return w.Close()
}

func parseAddr(from string) string {
	from = strings.TrimSpace(from)
	if i := strings.LastIndex(from, "<"); i >= 0 {
		j := strings.LastIndex(from, ">")
		if j > i {
			return strings.TrimSpace(from[i+1 : j])
		}
	}
	return from
}

func encodeSubject(s string) string {
	return fmt.Sprintf("=?UTF-8?B?%s?=", base64.StdEncoding.EncodeToString([]byte(s)))
}
