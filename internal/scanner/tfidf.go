package scanner

import (
	"math"
	"path/filepath"
	"strings"
)

// tfidfAlpha blends source count with TF-IDF uniqueness.
// 0.0 = pure source count, 1.0 = equal weight to uniqueness.
const tfidfAlpha = 0.3

// pathSegments splits a directory path into its segments.
// "apps/big-app/pages" → ["apps", "big-app", "pages"]
func pathSegments(dir string) []string {
	return strings.Split(dir, string(filepath.Separator))
}

// DirNameIDF computes inverse document frequency for each path segment
// across the entire directory list. Segments appearing in many dirs get
// lower IDF; unique segments get higher IDF.
//
// IDF(segment) = log(1 + N / DF(segment))
//
// where N = total dirs, DF = how many dirs contain that segment.
func DirNameIDF(dirs []string) map[string]float64 {
	df := map[string]int{} // document frequency per segment
	for _, d := range dirs {
		seen := map[string]bool{}
		for _, seg := range pathSegments(d) {
			if !seen[seg] {
				df[seg]++
				seen[seg] = true
			}
		}
	}

	n := float64(len(dirs))
	idf := make(map[string]float64, len(df))
	for seg, count := range df {
		idf[seg] = math.Log(1.0 + n/float64(count))
	}
	return idf
}

// DirTFIDF computes the TF-IDF score for a directory path.
// It sums the IDF of each segment and normalizes by the number of segments.
func DirTFIDF(dirPath string, idf map[string]float64) float64 {
	segs := pathSegments(dirPath)
	if len(segs) == 0 {
		return 0
	}
	sum := 0.0
	for _, seg := range segs {
		sum += idf[seg]
	}
	return sum / float64(len(segs))
}

// combinedWeight blends source file count with TF-IDF uniqueness score.
// Higher values = more important directory.
func combinedWeight(sourceCount int, tfidfScore float64) float64 {
	return float64(sourceCount) * (1.0 + tfidfAlpha*tfidfScore)
}
