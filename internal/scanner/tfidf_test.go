package scanner

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPathSegments(t *testing.T) {
	assert.Equal(t, []string{"apps", "big-app", "pages"}, pathSegments("apps/big-app/pages"))
	assert.Equal(t, []string{"src"}, pathSegments("src"))
	assert.Equal(t, []string{"."}, pathSegments("."))
}

func TestDirNameIDF_CommonVsUnique(t *testing.T) {
	dirs := []string{
		"services/auth/utils",
		"services/users/utils",
		"services/orders/utils",
		"services/payments/utils",
		"services/auth/controllers",
		"shared/activation",
	}
	idf := DirNameIDF(dirs)

	// "utils" appears in 4 dirs → low IDF
	// "activation" appears in 1 dir → high IDF
	assert.Greater(t, idf["activation"], idf["utils"],
		"unique segment should have higher IDF than common one")

	// "services" appears in 5 dirs → lower than "shared" (1 dir)
	assert.Greater(t, idf["shared"], idf["services"],
		"rare top-level segment should have higher IDF")
}

func TestDirTFIDF_MultipleSegments(t *testing.T) {
	dirs := []string{
		"services/auth/utils",
		"services/users/utils",
		"shared/activation",
	}
	idf := DirNameIDF(dirs)

	// "shared/activation" has 2 unique segments → higher average IDF
	// "services/auth/utils" has "services" and "utils" (both common) + "auth" (unique)
	scoreUnique := DirTFIDF("shared/activation", idf)
	scoreCommon := DirTFIDF("services/auth/utils", idf)

	assert.Greater(t, scoreUnique, 0.0, "TF-IDF should be positive")
	assert.Greater(t, scoreCommon, 0.0, "TF-IDF should be positive")
	assert.Greater(t, scoreUnique, scoreCommon,
		"path with more unique segments should score higher")
}

func TestCombinedWeight(t *testing.T) {
	// sourceCount=100, tfidfScore=2.0, alpha=0.3
	// 100 * (1 + 0.3 * 2.0) = 100 * 1.6 = 160.0
	w := combinedWeight(100, 2.0)
	assert.InDelta(t, 160.0, w, 0.01)

	// sourceCount=0 → always 0 regardless of TF-IDF
	assert.Equal(t, 0.0, combinedWeight(0, 5.0))
}

func TestDirNameIDF_EmptyInput(t *testing.T) {
	idf := DirNameIDF(nil)
	assert.Empty(t, idf)
}

func TestDirTFIDF_SingleSegment(t *testing.T) {
	idf := map[string]float64{"src": 2.5}
	score := DirTFIDF("src", idf)
	assert.InDelta(t, 2.5, score, 0.01, "single segment = its IDF directly")
}
