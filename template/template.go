package template

import (
	"bufio"
	"bytes"
	"embed"
	"fmt"
	"log"
	"regexp"
	"strings"
)

var TEMPLATE_PATTERN = regexp.MustCompile(`.*(\{\{[a-zA-Z_]+\}\}).*`)

//go:embed certs
var Certs embed.FS

//go:embed dist
var Dist embed.FS

//go:embed mkboot
var Mkboot embed.FS

//go:embed ipxe
var Ipxe embed.FS

func expandLine(match []string, macros map[string]string) (string, error) {
	if len(match) < 2 {
		return "", fmt.Errorf("unexpected match: %v", match)
	}
	result := match[0]
	macro := match[1]
	log.Printf("expanding: %s in %s\n", macro, result)
	for key, value := range macros {
		if "{{"+key+"}}" == macro {
			result = strings.ReplaceAll(result, macro, value)
			log.Printf("\t expanded: %s\n", result)
			return result, nil
		}
	}
	return "", fmt.Errorf("unexpanded template macro: %s", macro)
}

func ExpandTemplate(data []byte, macros map[string]string) ([]byte, error) {
	var obuf bytes.Buffer
	scanner := bufio.NewScanner(bytes.NewBuffer(data))
	for scanner.Scan() {
		line := scanner.Text()
		var expanded *string
		for expanded == nil {
			match := TEMPLATE_PATTERN.FindStringSubmatch(line)
			log.Printf("match[%d] %v\n", len(match), match)
			if len(match) == 0 {
				expanded = &line
			} else {
				var err error
				line, err = expandLine(match, macros)
				if err != nil {
					return []byte{}, err
				}
			}
		}
		obuf.WriteString(*expanded + "\n")
	}
	err := scanner.Err()
	if err != nil {
		return []byte{}, err
	}
	return obuf.Bytes(), nil
}
