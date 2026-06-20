package dotsql

import (
	"bufio"
	"regexp"
	"strings"
)

// tagRe matches lines like "-- name: my_query" or "-- name: my_query | display title"
var tagRe = regexp.MustCompile(`^\s*--\s*name:\s*(.*)$`)

// QueryInfo holds a parsed SQL query with its display metadata.
type QueryInfo struct {
	Content     string // raw SQL
	DisplayName string // name part before |
	Description string // text after |, if any (display title)
	Args        string // params after --, if any
	Disabled    bool   // true if all content lines are SQL comments (starts with --)
}

// Scanner parses SQL files containing named queries.
type Scanner struct {
	line    string
	queries map[string]*QueryInfo
	current string
	keys    []string // preserve insertion order
}

type stateFn func(*Scanner) stateFn

func getTag(line string) string {
	matches := tagRe.FindStringSubmatch(line)
	if matches == nil {
		return ""
	}
	return strings.TrimSpace(matches[1])
}

func initialState(s *Scanner) stateFn {
	if tag := getTag(s.line); len(tag) > 0 {
		s.current = tag
		s.keys = append(s.keys, tag)
		return queryState
	}
	return initialState
}

func queryState(s *Scanner) stateFn {
	if tag := getTag(s.line); len(tag) > 0 {
		s.current = tag
		s.keys = append(s.keys, tag)
	} else {
		s.appendQueryLine()
	}
	return queryState
}

func (s *Scanner) appendQueryLine() {
	info := s.queries[s.current]
	if info == nil {
		info = &QueryInfo{}
	}
	line := strings.Trim(s.line, " \t")
	if len(line) == 0 {
		return
	}

	// Skip SQL comment lines (-- ...)
	if strings.HasPrefix(line, "--") {
		return
	}

	if len(info.Content) > 0 {
		info.Content = info.Content + "\n"
	}
	info.Content = info.Content + line
	s.queries[s.current] = info
}

// Run parses the input and returns a map of query name -> QueryInfo, preserving insertion order.
func (s *Scanner) Run(io *bufio.Scanner) map[string]*QueryInfo {
	s.queries = make(map[string]*QueryInfo)
	s.keys = nil

	for state := initialState; io.Scan(); {
		s.line = io.Text()
		state = state(s)
	}

	// Ensure every key has an entry in the queries map,
	// and mark queries with empty content as disabled.
	for _, key := range s.keys {
		info := s.queries[key]
		if info == nil {
			// query had no content lines at all (e.g. all lines were -- comments)
			s.queries[key] = &QueryInfo{Disabled: true}
			continue
		}
		if strings.TrimSpace(info.Content) == "" {
			info.Disabled = true
		}
	}

	// Parse name annotations into DisplayName / Description / Args
	for _, key := range s.keys {
		info := s.queries[key]
		if info == nil {
			continue
		}
		info.DisplayName = key
		// split by "|" for display title
		if idx := strings.Index(key, "|"); idx >= 0 {
			info.DisplayName = strings.TrimSpace(key[:idx])
			info.Description = strings.TrimSpace(key[idx+1:])
		}
		// split DisplayName by "--" for parameter annotations
		if idx := strings.Index(info.DisplayName, "--"); idx >= 0 {
			info.Args = strings.TrimSpace(info.DisplayName[idx+2:])
			info.DisplayName = strings.TrimSpace(info.DisplayName[:idx])
		}
	}

	return s.queries
}

// Keys returns the query names in the order they were discovered.
func (s *Scanner) Keys() []string {
	return s.keys
}
