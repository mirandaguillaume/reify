package scanner

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSplitWords(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"UserController.ts", []string{"user", "controller"}},
		{"test_helper.py", []string{"test", "helper"}},
		{"my-component.tsx", []string{"my", "component"}},
		{"index.ts", []string{"index"}},
		{"Makefile", []string{"makefile"}},
		{"README", []string{"readme"}},
		{"HTMLParser.go", []string{"htmlparser"}}, // all-caps prefix stays together
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, splitWords(tt.input), "splitWords(%q)", tt.input)
	}
}

func TestFilePattern(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"UserController.ts", "controller"},
		{"OrderService.php", "service"},
		{"test_helper.spec.ts", "spec"},
		{"auth.test.ts", "test"},
		{"index.ts", "index"},
		{"Makefile", "makefile"},
		{"types.d.ts", "d"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, filePattern(tt.input), "filePattern(%q)", tt.input)
	}
}

func TestFileEntropy_AllSame(t *testing.T) {
	files := []string{"auth.spec.ts", "users.spec.ts", "orders.spec.ts"}
	h, norm := fileEntropy(files)
	assert.Equal(t, 0.0, h, "all same label → H=0")
	assert.Equal(t, 0.0, norm, "all same label → norm=0")
}

func TestFileEntropy_AllDifferent(t *testing.T) {
	files := []string{"UserController.ts", "UserService.ts", "UserRepository.ts"}
	h, norm := fileEntropy(files)
	assert.Greater(t, h, 0.0, "all different labels → H>0")
	assert.InDelta(t, 1.0, norm, 0.01, "perfectly uniform → norm≈1.0")
}

func TestFileEntropy_Mixed(t *testing.T) {
	files := []string{
		"auth.spec.ts", "users.spec.ts", "orders.spec.ts", "payments.spec.ts",
		"UserController.ts",
	}
	_, norm := fileEntropy(files)
	assert.Greater(t, norm, 0.0, "mixed → norm > 0")
	assert.Less(t, norm, 1.0, "mixed → norm < 1")
}

func TestFileEntropy_SingleFile(t *testing.T) {
	h, norm := fileEntropy([]string{"index.ts"})
	assert.Equal(t, 0.0, h)
	assert.Equal(t, 0.0, norm)
}

func TestAdaptiveFileCap_LowEntropy(t *testing.T) {
	// All specs → entropy ≈ 0 → cap = minEntropyFiles = 2
	files := []string{"a.spec.ts", "b.spec.ts", "c.spec.ts", "d.spec.ts"}
	cap := adaptiveFileCap(files)
	assert.Equal(t, minEntropyFiles, cap)
}

func TestAdaptiveFileCap_HighEntropy(t *testing.T) {
	// All different labels → normalized entropy ≈ 1.0 → cap = maxCollapsedFilesPerDir
	files := []string{
		"UserController.ts", "UserService.ts", "UserRepository.ts",
		"UserModel.ts", "UserValidator.ts", "UserFactory.ts",
	}
	cap := adaptiveFileCap(files)
	assert.Equal(t, maxCollapsedFilesPerDir, cap)
}
