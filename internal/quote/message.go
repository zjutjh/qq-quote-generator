package quote

import (
	"fmt"
	"strconv"
	"strings"
)

type Theme struct {
	CardBG    string
	AvatarBG  string
	NameColor string
	BubbleBG  string
	TextColor string
}

var lightTheme = Theme{
	CardBG: "#f7f8fb", AvatarBG: "#d9dee8", NameColor: "#667085",
	BubbleBG: "#ffffff", TextColor: "#242937",
}

func safeImageURL(raw string) string {
	raw = strings.TrimSpace(raw)
	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "data:image/") {
		return raw
	}
	return ""
}

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
