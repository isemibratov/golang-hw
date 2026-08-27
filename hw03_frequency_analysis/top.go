package hw03frequencyanalysis

import (
	"sort"
	"strings"
)

const topWordsLimit = 10

func Top10(text string) []string {
	wordCounts := make(map[string]int)
	for _, word := range strings.Fields(text) {
		wordCounts[word]++
	}

	words := make([]string, 0, len(wordCounts))
	for word := range wordCounts {
		words = append(words, word)
	}

	sort.Slice(words, func(i, j int) bool {
		if wordCounts[words[i]] == wordCounts[words[j]] {
			return words[i] < words[j]
		}

		return wordCounts[words[i]] > wordCounts[words[j]]
	})

	if len(words) > topWordsLimit {
		words = words[:topWordsLimit]
	}

	return words
}
