package main

import "html/template"

// MessageSegment 对应消息中的单个内容片段（文本或图片）
type MessageSegment struct {
	Type string `json:"type"` // "text" | "image"
	Kind string `json:"kind,omitempty"`
	Text string `json:"text,omitempty"`
	URL  string `json:"url,omitempty"` // 图片 URL 或 base64 data URI
}

// Message 对应一条聊天消息
type Message struct {
	UserID       int64  `json:"user_id"`
	UserNickname string `json:"user_nickname"`
	// Avatar 可以是头像 URL 或 base64，留空则自动用 QQ 头像接口
	Avatar string `json:"avatar,omitempty"`
	// Message 支持两种格式：
	//   1. 纯字符串（向后兼容原项目）
	//   2. []MessageSegment（支持图文混排）
	Message interface{} `json:"message"`
}

// renderData 是传给 HTML 模板的结构
type renderData struct {
	Messages []processedMessage
	Theme    string
}

type processedMessage struct {
	Nickname string
	Avatar   template.URL // 最终用于 <img src="..."> 的值
	Segments []processedMessageSegment
}

type processedMessageSegment struct {
	Type       string
	Kind       string
	Text       string
	URL        template.URL
	ImageClass string
}
