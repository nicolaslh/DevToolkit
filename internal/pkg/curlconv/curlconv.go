// Package curlconv parses a cURL command and emits request code in several
// languages (R6).
package curlconv

import (
	"fmt"
	"strings"

	"github.com/nic/devtoolkit/internal/pkg/apperr"
)

const MaxLen = 100000

// tokenize splits a shell-like command honoring single/double quotes and
// backslash line continuations.
func tokenize(cmd string) ([]string, error) {
	// Normalize line continuations.
	cmd = strings.ReplaceAll(cmd, "\\\n", " ")
	cmd = strings.ReplaceAll(cmd, "\n", " ")

	var tokens []string
	var cur strings.Builder
	inToken := false
	var quote byte // 0, '\'' or '"'

	flush := func() {
		if inToken {
			tokens = append(tokens, cur.String())
			cur.Reset()
			inToken = false
		}
	}

	for i := 0; i < len(cmd); i++ {
		c := cmd[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			} else {
				cur.WriteByte(c)
			}
			inToken = true
		case c == '\'' || c == '"':
			quote = c
			inToken = true
		case c == ' ' || c == '\t':
			flush()
		case c == '\\' && i+1 < len(cmd):
			cur.WriteByte(cmd[i+1])
			i++
			inToken = true
		default:
			cur.WriteByte(c)
			inToken = true
		}
	}
	if quote != 0 {
		return nil, apperr.New(apperr.ParseError, "cURL 命令中存在未闭合的引号")
	}
	flush()
	return tokens, nil
}

// Request is the parsed cURL command.
type Request struct {
	Method  string
	URL     string
	Headers []Header
	Body    string
}

// Header preserves order.
type Header struct {
	Key   string
	Value string
}

// Parse tokenizes and interprets a cURL command (R6.1, 6.6, 6.7, 6.8).
func Parse(cmd string) (Request, error) {
	if len(cmd) > MaxLen {
		return Request{}, apperr.Newf(apperr.TooLarge, "cURL 命令超出长度上限（最多 %d 个字符）", MaxLen)
	}
	tokens, err := tokenize(cmd)
	if err != nil {
		return Request{}, err
	}
	if len(tokens) == 0 || tokens[0] != "curl" {
		return Request{}, apperr.New(apperr.ParseError, "不是合法的 cURL 命令：应以 curl 开头")
	}

	req := Request{}
	for i := 1; i < len(tokens); i++ {
		t := tokens[i]
		switch {
		case t == "-X" || t == "--request":
			if i+1 < len(tokens) {
				req.Method = strings.ToUpper(tokens[i+1])
				i++
			}
		case t == "-H" || t == "--header":
			if i+1 < len(tokens) {
				h := tokens[i+1]
				if k, v, ok := splitHeader(h); ok {
					req.Headers = append(req.Headers, Header{Key: k, Value: v})
				}
				i++
			}
		case t == "-d" || t == "--data" || t == "--data-raw" || t == "--data-binary":
			if i+1 < len(tokens) {
				req.Body = tokens[i+1]
				i++
			}
		case t == "-u" || t == "--user":
			if i+1 < len(tokens) {
				req.Headers = append(req.Headers, Header{Key: "Authorization", Value: "Basic <base64(" + tokens[i+1] + ")>"})
				i++
			}
		case t == "--url":
			if i+1 < len(tokens) {
				req.URL = tokens[i+1]
				i++
			}
		case strings.HasPrefix(t, "-"):
			// Unknown flag; skip. A following value (if any) is left in place.
		default:
			if req.URL == "" {
				req.URL = t
			}
		}
	}

	if req.URL == "" {
		return Request{}, apperr.New(apperr.ParseError, "未能从命令中解析出 URL")
	}
	if req.Method == "" {
		if req.Body != "" {
			req.Method = "POST"
		} else {
			req.Method = "GET"
		}
	}
	return req, nil
}

func splitHeader(h string) (string, string, bool) {
	idx := strings.Index(h, ":")
	if idx < 0 {
		return "", "", false
	}
	return strings.TrimSpace(h[:idx]), strings.TrimSpace(h[idx+1:]), true
}

// Convert parses cmd and generates code for the target language (R6.1–6.5).
func Convert(cmd, target string) (string, error) {
	req, err := Parse(cmd)
	if err != nil {
		return "", err
	}
	switch target {
	case "python":
		return toPython(req), nil
	case "javascript":
		return toJavaScript(req), nil
	case "go":
		return toGo(req), nil
	case "java":
		return toJava(req), nil
	default:
		return "", apperr.New(apperr.InvalidInput, "不支持的目标语言")
	}
}

func toPython(req Request) string {
	var b strings.Builder
	b.WriteString("import requests\n\n")
	if len(req.Headers) > 0 {
		b.WriteString("headers = {\n")
		for _, h := range req.Headers {
			b.WriteString(fmt.Sprintf("    %q: %q,\n", h.Key, h.Value))
		}
		b.WriteString("}\n")
	} else {
		b.WriteString("headers = {}\n")
	}
	if req.Body != "" {
		b.WriteString(fmt.Sprintf("data = %q\n", req.Body))
	}
	b.WriteString(fmt.Sprintf("\nresponse = requests.request(%q, %q, headers=headers", req.Method, req.URL))
	if req.Body != "" {
		b.WriteString(", data=data")
	}
	b.WriteString(")\nprint(response.status_code)\nprint(response.text)\n")
	return b.String()
}

func toJavaScript(req Request) string {
	var b strings.Builder
	b.WriteString("const options = {\n")
	b.WriteString(fmt.Sprintf("  method: %q,\n", req.Method))
	if len(req.Headers) > 0 {
		b.WriteString("  headers: {\n")
		for _, h := range req.Headers {
			b.WriteString(fmt.Sprintf("    %q: %q,\n", h.Key, h.Value))
		}
		b.WriteString("  },\n")
	}
	if req.Body != "" {
		b.WriteString(fmt.Sprintf("  body: %q,\n", req.Body))
	}
	b.WriteString("};\n\n")
	b.WriteString(fmt.Sprintf("fetch(%q, options)\n", req.URL))
	b.WriteString("  .then((res) => res.text())\n  .then(console.log)\n  .catch(console.error);\n")
	return b.String()
}

func toGo(req Request) string {
	var b strings.Builder
	b.WriteString("package main\n\nimport (\n\t\"fmt\"\n\t\"io\"\n\t\"net/http\"\n")
	if req.Body != "" {
		b.WriteString("\t\"strings\"\n")
	}
	b.WriteString(")\n\nfunc main() {\n")
	if req.Body != "" {
		b.WriteString(fmt.Sprintf("\tbody := strings.NewReader(%q)\n", req.Body))
		b.WriteString(fmt.Sprintf("\treq, _ := http.NewRequest(%q, %q, body)\n", req.Method, req.URL))
	} else {
		b.WriteString(fmt.Sprintf("\treq, _ := http.NewRequest(%q, %q, nil)\n", req.Method, req.URL))
	}
	for _, h := range req.Headers {
		b.WriteString(fmt.Sprintf("\treq.Header.Set(%q, %q)\n", h.Key, h.Value))
	}
	b.WriteString("\tresp, err := http.DefaultClient.Do(req)\n\tif err != nil {\n\t\tpanic(err)\n\t}\n")
	b.WriteString("\tdefer resp.Body.Close()\n\tdata, _ := io.ReadAll(resp.Body)\n\tfmt.Println(resp.Status)\n\tfmt.Println(string(data))\n}\n")
	return b.String()
}

func toJava(req Request) string {
	var b strings.Builder
	b.WriteString("import java.net.URI;\nimport java.net.http.*;\n\n")
	b.WriteString("HttpClient client = HttpClient.newHttpClient();\n")
	b.WriteString("HttpRequest request = HttpRequest.newBuilder()\n")
	b.WriteString(fmt.Sprintf("    .uri(URI.create(%q))\n", req.URL))
	for _, h := range req.Headers {
		b.WriteString(fmt.Sprintf("    .header(%q, %q)\n", h.Key, h.Value))
	}
	if req.Body != "" {
		b.WriteString(fmt.Sprintf("    .method(%q, HttpRequest.BodyPublishers.ofString(%q))\n", req.Method, req.Body))
	} else {
		b.WriteString(fmt.Sprintf("    .method(%q, HttpRequest.BodyPublishers.noBody())\n", req.Method))
	}
	b.WriteString("    .build();\n")
	b.WriteString("HttpResponse<String> response = client.send(request, HttpResponse.BodyHandlers.ofString());\n")
	b.WriteString("System.out.println(response.statusCode());\nSystem.out.println(response.body());\n")
	return b.String()
}
