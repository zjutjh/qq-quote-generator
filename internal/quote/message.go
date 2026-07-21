package quote

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	cardBackground   = "#f7f8fb"
	avatarBackground = "#d9dee8"
	nicknameColor    = "#667085"
	bubbleBackground = "#ffffff"
	messageColor     = "#242937"
)

func resolveAvatar(msg Message) string {
	if msg.Avatar != "" {
		return msg.Avatar
	}
	if msg.UserID > 0 {
		return fmt.Sprintf("https://q1.qlogo.cn/g?b=qq&nk=%d&s=100", msg.UserID)
	}
	return ""
}

func faceID(value string) string {
	id := strings.TrimSpace(value)
	if len(id) == 0 || len(id) > 10 {
		return ""
	}
	parsed, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return ""
	}
	return strconv.FormatUint(parsed, 10)
}
