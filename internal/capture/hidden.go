package capture

import "fmt"

// Hidden is the first invisible character found in a text, for a message that tells a reviewer
// where to look.
type Hidden struct {
	Line int
	Rune rune
}

// String names the character the way a reviewer can search for it.
func (h Hidden) String() string {
	return fmt.Sprintf("U+%04X %s on line %d", h.Rune, hiddenName(h.Rune), h.Line)
}

// HiddenCharacters counts the characters in text that a model reads and a reviewer cannot see,
// and returns the first one.
//
// The list is deliberately short. It holds the characters with no honest use in a prompt, a tool
// description or an agent instruction file, and leaves out the ones ordinary writing needs: the
// zero-width joiner and non-joiner (emoji sequences, Persian, Indic scripts), the left-to-right
// and right-to-left marks, and the emoji variation selector. A check that fired on a flag emoji
// would be switched off, and then it would catch nothing.
//
//   - U+E0000 to U+E007F, the tag block: invisible copies of ASCII, the usual carrier for an
//     instruction smuggled into a rules file.
//   - U+202A to U+202E and U+2066 to U+2069: bidirectional embeddings, overrides and isolates,
//     which make text read in an order other than the one it is stored in.
//   - U+200B, U+2060 and a U+FEFF that is not the first character: zero-width spaces.
//   - U+2061 to U+2064: invisible mathematical operators.
//   - U+E0100 to U+E01EF: the variation selector supplement, which can encode arbitrary bytes.
//   - U+115F, U+1160, U+3164 and U+FFA0: Hangul fillers, which render as nothing.
//   - U+180E: the Mongolian vowel separator.
func HiddenCharacters(text string) (int, *Hidden) {
	count, line := 0, 1
	var first *Hidden
	for i, r := range text {
		if r == '\n' {
			line++
			continue
		}
		if !isHidden(r) || (r == 0xFEFF && i == 0) {
			continue
		}
		count++
		if first == nil {
			first = &Hidden{Line: line, Rune: r}
		}
	}
	return count, first
}

func isHidden(r rune) bool {
	switch {
	case r >= 0xE0000 && r <= 0xE007F,
		r >= 0x202A && r <= 0x202E,
		r >= 0x2066 && r <= 0x2069,
		r == 0x200B, r == 0x2060, r == 0xFEFF,
		r >= 0x2061 && r <= 0x2064,
		r >= 0xE0100 && r <= 0xE01EF,
		r == 0x115F, r == 0x1160, r == 0x3164, r == 0xFFA0,
		r == 0x180E:
		return true
	}
	return false
}

func hiddenName(r rune) string {
	switch {
	case r >= 0xE0000 && r <= 0xE007F:
		return "(tag character)"
	case r >= 0x202A && r <= 0x202E, r >= 0x2066 && r <= 0x2069:
		return "(bidirectional control)"
	case r == 0x200B, r == 0x2060, r == 0xFEFF:
		return "(zero-width space)"
	case r >= 0x2061 && r <= 0x2064:
		return "(invisible operator)"
	case r >= 0xE0100 && r <= 0xE01EF:
		return "(variation selector)"
	case r == 0x180E:
		return "(Mongolian vowel separator)"
	}
	return "(invisible filler)"
}
