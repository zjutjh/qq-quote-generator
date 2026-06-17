package main

// quoteHTML 是内嵌的 HTML 模板，直接用 Go template 语法渲染，
// 无需任何本地 HTTP 服务器。样式与原项目保持一致。
const quoteHTML = `<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }

  body {
    background: transparent;
    font-family: "PingFang SC", "Microsoft YaHei", "Noto Sans CJK SC", sans-serif;
  }

  .theme-dark {
    --card-bg: #1e1e2e;
    --avatar-bg: #333;
    --name-color: #7c7f93;
    --bubble-bg: #313244;
    --text-color: #cdd6f4;
  }

  .theme-light {
    --card-bg: #f7f8fb;
    --avatar-bg: #d9dee8;
    --name-color: #667085;
    --bubble-bg: #ffffff;
    --text-color: #242937;
  }

  #app {
    background: var(--card-bg);
    padding: 16px 12px;
    min-width: 0;
    max-width: 600px;
    display: inline-block;
    border-radius: 12px;
  }

  .message-row {
    display: flex;
    align-items: flex-start;
    margin-bottom: 14px;
    gap: 10px;
  }
  .message-row:last-child {
    margin-bottom: 0;
  }

  .avatar {
    width: 42px;
    height: 42px;
    border-radius: 50%;
    flex-shrink: 0;
    object-fit: cover;
    background: var(--avatar-bg);
  }

  .content {
    flex: 1;
    min-width: 0;
  }

  .nickname {
    font-size: 12px;
    color: var(--name-color);
    margin-bottom: 4px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .bubble {
    background: var(--bubble-bg);
    border-radius: 4px 12px 12px 12px;
    padding: 8px 12px;
    display: inline-block;
    max-width: 100%;
    word-break: break-word;
  }

  .bubble-text {
    color: var(--text-color);
    font-size: 15px;
    line-height: 1.6;
    white-space: pre-wrap;
  }

  .bubble-img {
    max-width: 200px;
    max-height: 200px;
    border-radius: 6px;
    display: block;
  }

  /* 图文混排：图片和文字间加间距 */
  .seg + .seg {
    margin-top: 4px;
  }
</style>
</head>
<body>
<div id="app" class="{{.Theme}}">
{{- range .Messages}}
  <div class="message-row">
    {{- if .Avatar}}
    <img class="avatar" src="{{.Avatar}}" onerror="this.style.background='#45475a'">
    {{- else}}
    <div class="avatar"></div>
    {{- end}}
    <div class="content">
      <div class="nickname">{{.Nickname}}</div>
      <div class="bubble">
        {{- range .Segments}}
          {{- if eq .Type "text"}}
          <div class="seg bubble-text">{{.Text}}</div>
          {{- else if eq .Type "image"}}
          <div class="seg"><img class="bubble-img" src="{{.URL}}"></div>
          {{- end}}
        {{- end}}
      </div>
    </div>
  </div>
{{- end}}
</div>
</body>
</html>`
