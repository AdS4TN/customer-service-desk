package chunk

import (
	"agent-desk/internal/pkg/enums"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	defaultTargetTokens = 300
	defaultMaxTokens    = 400
	defaultParentTokens = 900
	defaultChildTokens  = 200
)

func normalizeOptions(opts ChunkOptions) ChunkOptions {
	if opts.TargetTokens <= 0 {
		opts.TargetTokens = defaultTargetTokens
	}
	if opts.MaxTokens <= 0 {
		opts.MaxTokens = defaultMaxTokens
	}
	if opts.MaxTokens < opts.TargetTokens {
		opts.MaxTokens = opts.TargetTokens
	}
	if opts.OverlapTokens < 0 {
		opts.OverlapTokens = 0
	}
	if opts.OverlapTokens >= opts.MaxTokens {
		opts.OverlapTokens = opts.MaxTokens / 4
	}
	if opts.ParentTokens <= 0 {
		opts.ParentTokens = defaultParentTokens
	}
	if opts.ChildTokens <= 0 {
		opts.ChildTokens = defaultChildTokens
	}
	if opts.ParentTokens < opts.ChildTokens {
		opts.ParentTokens = opts.ChildTokens
	}
	if opts.Provider == "" {
		opts.Provider = string(enums.KnowledgeChunkProviderStructured)
	}
	return opts
}

func NormalizeOptions(opts ChunkOptions) ChunkOptions {
	return normalizeOptions(opts)
}

func normalizeText(text string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
}

func estimateTokenCount(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	count := 0
	inWord := false
	for _, r := range text {
		switch {
		case unicode.IsSpace(r):
			inWord = false
		case unicode.Is(unicode.Han, r):
			count++
			inWord = false
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if !inWord {
				count++
				inWord = true
			}
		default:
			count++
			inWord = false
		}
	}
	if count == 0 {
		return utf8.RuneCountInString(text)
	}
	return count
}

func contentHash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func splitSentences(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	var sentences []string
	var builder strings.Builder
	for _, r := range text {
		builder.WriteRune(r)
		switch r {
		case '\n', '。', '！', '？', '!', '?', ';', '；':
			sentence := normalizeText(builder.String())
			if sentence != "" {
				sentences = append(sentences, sentence)
			}
			builder.Reset()
		}
	}
	if builder.Len() > 0 {
		sentence := normalizeText(builder.String())
		if sentence != "" {
			sentences = append(sentences, sentence)
		}
	}
	if len(sentences) == 0 {
		return []string{normalizeText(text)}
	}
	return sentences
}

func tailTextByTokens(text string, tokenLimit int) string {
	if tokenLimit <= 0 {
		return ""
	}
	sentences := splitSentences(text)
	if len(sentences) == 0 {
		return ""
	}
	var selected []string
	total := 0
	for i := len(sentences) - 1; i >= 0; i-- {
		sentence := sentences[i]
		tokens := estimateTokenCount(sentence)
		if total > 0 && total+tokens > tokenLimit {
			break
		}
		selected = append([]string{sentence}, selected...)
		total += tokens
	}
	return strings.TrimSpace(strings.Join(selected, " "))
}

func splitPlainText(text string, opts ChunkOptions) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	opts = normalizeOptions(opts)
	sentences := splitSentences(text)
	if len(sentences) == 0 {
		return nil
	}

	chunks := make([]string, 0)
	current := make([]string, 0)
	currentTokens := 0
	dirty := false

	flush := func() string {
		if len(current) == 0 || !dirty {
			return ""
		}
		value := strings.TrimSpace(strings.Join(current, " "))
		if value != "" {
			chunks = append(chunks, value)
		}
		return value
	}

	appendPiece := func(piece string) {
		piece = normalizeText(piece)
		if piece == "" {
			return
		}
		pieceTokens := estimateTokenCount(piece)
		if currentTokens > 0 && currentTokens+pieceTokens > opts.MaxTokens {
			flushed := flush()
			overlap := tailTextByTokens(flushed, opts.OverlapTokens)
			current = nil
			currentTokens = 0
			dirty = false
			if overlap != "" {
				current = append(current, overlap)
				currentTokens = estimateTokenCount(overlap)
			}
		}
		current = append(current, piece)
		currentTokens += pieceTokens
		dirty = true
		if currentTokens >= opts.TargetTokens {
			flushed := flush()
			overlap := tailTextByTokens(flushed, opts.OverlapTokens)
			current = nil
			currentTokens = 0
			dirty = false
			if overlap != "" {
				current = append(current, overlap)
				currentTokens = estimateTokenCount(overlap)
			}
		}
	}

	for _, sentence := range sentences {
		if estimateTokenCount(sentence) > opts.MaxTokens {
			for _, piece := range splitLongSentence(sentence, opts.MaxTokens) {
				appendPiece(piece)
			}
			continue
		}
		appendPiece(sentence)
	}

	flush()
	return chunks
}

func splitLongSentence(text string, maxTokens int) []string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) == 0 {
		return nil
	}
	if maxTokens <= 0 {
		return []string{text}
	}
	var result []string
	start := 0
	for start < len(runes) {
		end := start
		count := 0
		inWord := false
		for end < len(runes) {
			increment, nextInWord := tokenIncrement(runes[end], inWord)
			if count > 0 && count+increment > maxTokens {
				break
			}
			count += increment
			inWord = nextInWord
			end++
		}
		part := normalizeText(string(runes[start:end]))
		if part != "" {
			result = append(result, part)
		}
		start = end
	}
	return result
}

func tokenIncrement(r rune, inWord bool) (int, bool) {
	switch {
	case unicode.IsSpace(r):
		return 0, false
	case unicode.Is(unicode.Han, r):
		return 1, false
	case unicode.IsLetter(r) || unicode.IsDigit(r):
		if inWord {
			return 0, true
		}
		return 1, true
	default:
		return 1, false
	}
}
