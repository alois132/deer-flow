package safe

import (
	"encoding/json"
	"regexp"
)

var markdownJSONRegex = regexp.MustCompile("```(?:json)?\\s*([\\s\\S]*?)\\s*```")

func ParseJSON[T any](input string, output *T) error {
	b := []byte(input)
	// 尝试提取代码块内容
	matches := markdownJSONRegex.FindSubmatch(b)
	if len(matches) > 1 {
		b = matches[1] // 取第一个捕获组（JSON 内容）
	}

	err := json.Unmarshal(b, &output)
	return err
}

func Value[T any](data *T) T {
	if data == nil {
		var t T
		return t
	}
	return *data
}
