package view

import "strings"

func GetLanguageIcon(language string) string {
	// https://icon-sets.iconify.design/mdi/
	langs := map[string]string{
		"c++":        "language-cpp",
		"cpp":        "language-cpp",
		"go":         "language-go",
		"haskell":    "language-haskell",
		"html":       "language-html5",
		"java":       "language-java",
		"javascript": "language-javascript",
		"jsx":        "language-javascript",
		"kotlin":     "language-kotlin",
		"lua":        "language-lua",
		"php":        "language-php",
		"python":     "language-python",
		"r":          "language-r",
		"ruby":       "language-ruby",
		"rust":       "language-rust",
		"swift":      "language-swift",
		"typescript": "language-typescript",
		"tsx":        "language-typescript",
		"markdown":   "language-markdown",
		"vue":        "vuejs",
		"react":      "react",
		"bash":       "bash",
		"json":       "code-json",
		"figma":      "palette",
	}
	if match, ok := langs[strings.ToLower(language)]; ok {
		return "mdi:" + match
	}
	return ""
}
