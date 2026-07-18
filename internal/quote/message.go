package quote

import (
	"encoding/json"
	"fmt"
	"math"
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

func faceID(value any) string {
	var id string
	switch value := value.(type) {
	case string:
		id = strings.TrimSpace(value)
	case float64:
		if value < 0 || value > math.MaxInt64 || value != math.Trunc(value) {
			return ""
		}
		id = strconv.FormatInt(int64(value), 10)
	case int:
		if value < 0 {
			return ""
		}
		id = strconv.Itoa(value)
	case int64:
		if value < 0 {
			return ""
		}
		id = strconv.FormatInt(value, 10)
	default:
		return ""
	}
	if len(id) == 0 || len(id) > 10 {
		return ""
	}
	parsed, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return ""
	}
	return strconv.FormatUint(parsed, 10)
}

func parseMessageField(raw interface{}) ([]MessageSegment, error) {
	if raw == nil {
		return nil, nil
	}
	switch v := raw.(type) {
	case string:
		return []MessageSegment{{Type: "text", Text: v}}, nil
	case []interface{}:
		data, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		var segments []MessageSegment
		if err := json.Unmarshal(data, &segments); err != nil {
			return nil, err
		}
		return segments, nil
	default:
		return []MessageSegment{{Type: "text", Text: fmt.Sprintf("%v", v)}}, nil
	}
}
