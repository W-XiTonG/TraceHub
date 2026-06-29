package mail

import "fmt"

// AnnouncementEmail 返回公告通知纯文本邮件的主题与正文。
func AnnouncementEmail(title, content, consoleURL string) (subject, body string) {
	subject = fmt.Sprintf("【TraceHub 公告】%s", title)
	body = fmt.Sprintf("%s\n\n%s\n", title, content)
	if consoleURL != "" {
		body += fmt.Sprintf("\n登录控制台查看：%s\n", consoleURL)
	}
	body += "\n---\n您收到此邮件是因为管理员发布了与您相关的系统公告。\n"
	return subject, body
}
