package scanner

import (
	"math"
	"path/filepath"
	"strings"
	"unicode"
)

// minEntropyFiles is the floor for low-entropy (repetitive) directories.
const minEntropyFiles = 2

// splitWords breaks a filename into words using camelCase, snake_case, and kebab-case boundaries.
// "UserController" → ["User", "Controller"]
// "test_helper" → ["test", "helper"]
// "my-component" → ["my", "component"]
func splitWords(name string) []string {
	// Remove extension first.
	ext := filepath.Ext(name)
	if ext != "" {
		name = strings.TrimSuffix(name, ext)
	}

	var words []string
	var current []rune

	flush := func() {
		if len(current) > 0 {
			words = append(words, string(current))
			current = current[:0]
		}
	}

	runes := []rune(name)
	for i, r := range runes {
		switch {
		case r == '_' || r == '-' || r == '.':
			flush()
		case unicode.IsUpper(r):
			// CamelCase boundary: flush if previous was lowercase.
			if i > 0 && unicode.IsLower(runes[i-1]) {
				flush()
			}
			current = append(current, unicode.ToLower(r))
		default:
			current = append(current, r)
		}
	}
	flush()

	return words
}

// filePattern extracts the "type label" of a filename — the last meaningful
// word before the extension. This captures the role of the file.
// "UserController.ts" → "controller"
// "test_helper.spec.ts" → "spec"
// "index.ts" → "index"
// "Makefile" → "makefile"
func filePattern(filename string) string {
	// Handle double extensions like .spec.ts, .test.ts.
	base := filename
	for {
		ext := filepath.Ext(base)
		if ext == "" {
			break
		}
		stem := strings.TrimSuffix(base, ext)
		inner := filepath.Ext(stem)
		if inner == ".spec" || inner == ".test" || inner == ".d" {
			return strings.TrimPrefix(inner, ".")
		}
		base = stem
	}

	words := splitWords(filename)
	if len(words) == 0 {
		return strings.ToLower(filename)
	}
	return words[len(words)-1]
}

// fileEntropy computes the Shannon entropy of file type labels in a directory.
// Returns (entropy, normalized) where normalized ∈ [0, 1].
// H = -Σ p(label) * log2(p(label))
// normalized = H / log2(k), where k = distinct labels.
func fileEntropy(files []string) (entropy float64, normalized float64) {
	if len(files) <= 1 {
		return 0, 0
	}

	// Count label frequencies.
	freq := map[string]int{}
	for _, f := range files {
		label := filePattern(f)
		freq[label]++
	}

	k := len(freq)
	if k <= 1 {
		return 0, 0
	}

	// Compute Shannon entropy.
	n := float64(len(files))
	h := 0.0
	for _, count := range freq {
		p := float64(count) / n
		h -= p * math.Log2(p)
	}

	maxH := math.Log2(float64(k))
	norm := h / maxH

	return h, norm
}

// adaptiveFileCap returns how many files to show for a directory based on
// the diversity of its file name patterns. Low diversity → fewer files (min 2),
// high diversity → more files (up to maxCollapsedFilesPerDir).
func adaptiveFileCap(files []string) int {
	_, norm := fileEntropy(files)
	cap := minEntropyFiles + int(math.Round(norm*float64(maxCollapsedFilesPerDir-minEntropyFiles)))
	if cap < minEntropyFiles {
		cap = minEntropyFiles
	}
	if cap > maxCollapsedFilesPerDir {
		cap = maxCollapsedFilesPerDir
	}
	return cap
}
