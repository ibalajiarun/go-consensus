package promquery

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

func CsvWriter(w io.Writer, resultSets ...[]Result) error {
	if len(resultSets) == 0 {
		return nil
	}

	// Deduplicate all times from all results by passing them as key into a map.
	timesMap := make(map[string]bool)
	for _, results := range resultSets {
		for _, result := range results {
			for time := range result.Values {
				timesMap[time] = true
			}
		}
	}

	// Create a sorted slice of all times to iterate over later.
	var times []string
	for time := range timesMap {
		times = append(times, time)
	}
	sort.Slice(times, func(i, j int) bool {
		return times[i] < times[j]
	})

	// Iterate over all times and find the belonging values for each result.
	for _, time := range times {
		fmt.Fprint(w, time)
		for _, results := range resultSets {
			for _, result := range results {
				fmt.Fprint(w, ","+result.Values[time])
			}
		}
		fmt.Fprintln(w)
	}

	return nil
}

func CsvHeaderWriter(w io.Writer, results []Result) error {
	if len(results) == 0 {
		return nil
	}

	header := []string{"Time"}
	for _, result := range results {
		header = append(header, result.Metric)
	}

	fmt.Fprintln(w, strings.Join(header, ","))
	return nil
}
