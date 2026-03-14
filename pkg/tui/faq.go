package tui

import (
	"embed"
	"encoding/json"
	"log"
	"strings"
)

//go:embed faq.json
var faqData embed.FS

// FAQ represents a single frequently asked question and its answer.
type FAQ struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

// wordWrap breaks a string into lines that fit within maxWidth characters.
func wordWrap(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return s
	}
	words := strings.Fields(s)
	if len(words) == 0 {
		return ""
	}

	var lines []string
	line := words[0]
	for _, word := range words[1:] {
		if len(line)+1+len(word) > maxWidth {
			lines = append(lines, line)
			line = word
		} else {
			line += " " + word
		}
	}
	lines = append(lines, line)
	return strings.Join(lines, "\n")
}

// LoadFaqs reads and parses the embedded faq.json file.
func LoadFaqs() []FAQ {
	data, err := faqData.ReadFile("faq.json")
	if err != nil {
		log.Fatalf("failed to read embedded faq.json: %s", err)
	}
	var faqs []FAQ
	if err := json.Unmarshal(data, &faqs); err != nil {
		log.Fatalf("failed to unmarshal faq.json: %s", err)
	}
	return faqs
}
