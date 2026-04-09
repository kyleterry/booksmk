// Package importer parses browser bookmark export files into a common
// Bookmark type. Supported formats:
//   - Netscape HTML (exported by Firefox, Chrome, Safari, and most browsers)
//   - Pinboard CSV (href,description,extended,...,tags)
//   - Delicious CSV (same column layout as Pinboard)
package importer

import (
	"bufio"
	"encoding/csv"
	"errors"
	"io"
	"regexp"
	"strings"
)

// Bookmark holds the data extracted from a single bookmark entry.
type Bookmark struct {
	URL         string
	Title       string
	Description string
	Tags        []string
}

// ErrUnrecognisedFormat is returned when a CSV file does not contain a
// recognisable header row.
var ErrUnrecognisedFormat = errors.New("bookmarkparser: unrecognised file format")

// Regexes for Netscape HTML bookmark parsing.
var (
	// reDTAnchor matches a <DT><A ...>Title</A> line (case-insensitive).
	reDTAnchor = regexp.MustCompile(`(?i)<DT><A\b([^>]*)>(.*?)</A>`)
	// reHREF extracts the HREF attribute value.
	reHREF = regexp.MustCompile(`(?i)\bHREF="([^"]*)"`)
	// reTAGS extracts the TAGS attribute value (used by Firefox exports).
	reTAGS = regexp.MustCompile(`(?i)\bTAGS="([^"]*)"`)
	// reDD matches a <DD> description line.
	reDD = regexp.MustCompile(`(?i)^[ \t]*<DD>(.*)`)
)

// ParseNetscapeHTML parses a Netscape bookmark HTML file (as exported by
// Firefox, Chrome, Safari, etc.) and returns the flat list of bookmarks
// regardless of folder structure. The reader is expected to contain the full
// file.
func ParseNetscapeHTML(r io.Reader) ([]Bookmark, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	var bookmarks []Bookmark
	var pending *Bookmark

	for scanner.Scan() {
		line := scanner.Text()

		if m := reDTAnchor.FindStringSubmatch(line); m != nil {
			// Save any previously parsed entry before starting a new one.
			if pending != nil {
				bookmarks = append(bookmarks, *pending)
			}

			attrs := m[1]
			title := strings.TrimSpace(m[2])

			hrefM := reHREF.FindStringSubmatch(attrs)
			if hrefM == nil {
				pending = nil
				continue
			}
			rawURL := hrefM[1]

			var tags []string
			if tagsM := reTAGS.FindStringSubmatch(attrs); tagsM != nil {
				tags = splitComma(tagsM[1])
			}

			pending = &Bookmark{
				URL:   rawURL,
				Title: title,
				Tags:  tags,
			}
			continue
		}

		if pending != nil {
			if ddM := reDD.FindStringSubmatch(line); ddM != nil {
				pending.Description = strings.TrimSpace(ddM[1])
				bookmarks = append(bookmarks, *pending)
				pending = nil
				continue
			}

			// Any non-blank, non-DD line after a DT flushes the pending entry.
			if strings.TrimSpace(line) != "" {
				bookmarks = append(bookmarks, *pending)
				pending = nil
			}
		}
	}

	if pending != nil {
		bookmarks = append(bookmarks, *pending)
	}

	return bookmarks, scanner.Err()
}

// ParsePinboardCSV parses a Pinboard (or Delicious) CSV export. The first row
// must be a header matching the expected column names. Tags in Pinboard exports
// are space-separated.
func ParsePinboardCSV(r io.Reader) ([]Bookmark, error) {
	cr := csv.NewReader(r)
	cr.LazyQuotes = true
	cr.TrimLeadingSpace = true

	header, err := cr.Read()
	if errors.Is(err, io.EOF) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	idx, err := pinboardColumnIndex(header)
	if err != nil {
		return nil, err
	}

	var bookmarks []Bookmark

	for {
		row, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if len(row) <= idx.href {
			continue
		}

		rawURL := strings.TrimSpace(row[idx.href])
		if rawURL == "" {
			continue
		}

		bm := Bookmark{URL: rawURL}

		if idx.description >= 0 && idx.description < len(row) {
			bm.Title = strings.TrimSpace(row[idx.description])
		}
		if idx.extended >= 0 && idx.extended < len(row) {
			bm.Description = strings.TrimSpace(row[idx.extended])
		}
		if idx.tags >= 0 && idx.tags < len(row) {
			bm.Tags = splitSpace(row[idx.tags])
		}

		bookmarks = append(bookmarks, bm)
	}

	return bookmarks, nil
}

type pinboardCols struct {
	href        int
	description int
	extended    int
	tags        int
}

func pinboardColumnIndex(header []string) (pinboardCols, error) {
	lower := make([]string, len(header))
	for i, h := range header {
		lower[i] = strings.ToLower(strings.TrimSpace(h))
	}

	findCol := func(name string) int {
		for i, h := range lower {
			if h == name {
				return i
			}
		}
		return -1
	}

	hrefIdx := findCol("href")
	if hrefIdx < 0 {
		return pinboardCols{}, ErrUnrecognisedFormat
	}

	return pinboardCols{
		href:        hrefIdx,
		description: findCol("description"),
		extended:    findCol("extended"),
		tags:        findCol("tags"),
	}, nil
}

// splitComma splits a comma-separated string into trimmed non-empty parts.
func splitComma(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// splitSpace splits a space-separated string into trimmed non-empty parts.
func splitSpace(s string) []string {
	parts := strings.Fields(s)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
