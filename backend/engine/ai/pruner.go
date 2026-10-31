package ai

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var defaultPIIPatterns = []string{
	"email", "phone", "password", "pwd", "token", "secret",
	"salary", "credit_card", "pin", "ssn", "nik", "cvv", "auth",
}

// ColumnInfo holds name and type of a table column
type ColumnInfo struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// TableSchema holds table name and its columns
type TableSchema struct {
	AliasName string       `json:"alias_name"` // e.g. "db_transaksi"
	TableName string       `json:"table_name"` // e.g. "orders"
	Columns   []ColumnInfo `json:"columns"`
}

// PruneAndMaskSchema filters out PII columns and selects top relevant tables matching the prompt
func PruneAndMaskSchema(
	prompt string,
	tables []TableSchema,
	customMasking []string,
	maxTables int,
) ([]TableSchema, string) {
	if maxTables <= 0 {
		maxTables = 8
	}

	// 1. Build mask lookup
	maskSet := make(map[string]struct{})
	for _, p := range defaultPIIPatterns {
		maskSet[strings.ToLower(p)] = struct{}{}
	}
	for _, m := range customMasking {
		clean := strings.ToLower(strings.TrimSpace(m))
		if clean != "" {
			maskSet[clean] = struct{}{}
		}
	}

	// 2. Tokenize prompt
	promptTokens := tokenize(prompt)

	// 3. Score and sanitize tables
	type scoredTable struct {
		table TableSchema
		score int
	}

	var scoredList []scoredTable

	for _, tbl := range tables {
		// Filter out PII columns
		var safeCols []ColumnInfo
		for _, col := range tbl.Columns {
			if isMasked(col.Name, maskSet) {
				continue
			}
			safeCols = append(safeCols, col)
		}

		if len(safeCols) == 0 {
			continue
		}

		sanitizedTable := TableSchema{
			AliasName: tbl.AliasName,
			TableName: tbl.TableName,
			Columns:   safeCols,
		}

		// Calculate score based on prompt tokens
		score := scoreTable(sanitizedTable, promptTokens)
		scoredList = append(scoredList, scoredTable{
			table: sanitizedTable,
			score: score,
		})
	}

	// Sort by score descending
	sort.Slice(scoredList, func(i, j int) bool {
		return scoredList[i].score > scoredList[j].score
	})

	// Select top N tables
	n := len(scoredList)
	if n > maxTables {
		n = maxTables
	}

	selected := make([]TableSchema, n)
	var ddlBuilder strings.Builder

	for i := 0; i < n; i++ {
		t := scoredList[i].table
		selected[i] = t

		fullName := t.TableName
		if t.AliasName != "" {
			fullName = t.AliasName + "." + t.TableName
		}

		ddlBuilder.WriteString(fmt.Sprintf("-- Table: %s\n", fullName))
		ddlBuilder.WriteString(fmt.Sprintf("CREATE TABLE %s (\n", fullName))
		for cIdx, col := range t.Columns {
			colType := col.Type
			if colType == "" {
				colType = "VARCHAR"
			}
			comma := ","
			if cIdx == len(t.Columns)-1 {
				comma = ""
			}
			ddlBuilder.WriteString(fmt.Sprintf("    %s %s%s\n", col.Name, colType, comma))
		}
		ddlBuilder.WriteString(");\n\n")
	}

	return selected, ddlBuilder.String()
}

func isMasked(colName string, maskSet map[string]struct{}) bool {
	lower := strings.ToLower(colName)
	if _, ok := maskSet[lower]; ok {
		return true
	}
	for pattern := range maskSet {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

var nonAlphaNum = regexp.MustCompile(`[^a-zA-Z0-9]+`)

func tokenize(s string) map[string]struct{} {
	tokens := make(map[string]struct{})
	cleaned := nonAlphaNum.ReplaceAllString(strings.ToLower(s), " ")
	for _, word := range strings.Fields(cleaned) {
		if len(word) > 1 {
			tokens[word] = struct{}{}
		}
	}
	return tokens
}

func scoreTable(t TableSchema, tokens map[string]struct{}) int {
	score := 1 // Base score

	tblLower := strings.ToLower(t.TableName)
	if _, ok := tokens[tblLower]; ok {
		score += 10
	} else {
		tblParts := nonAlphaNum.ReplaceAllString(tblLower, " ")
		for _, part := range strings.Fields(tblParts) {
			if len(part) > 1 {
				if _, ok := tokens[part]; ok {
					score += 5
				} else {
					for tok := range tokens {
						if strings.HasPrefix(tok, part) || strings.HasPrefix(part, tok) {
							score += 3
							break
						}
					}
				}
			}
		}
	}

	aliasLower := strings.ToLower(t.AliasName)
	if _, ok := tokens[aliasLower]; ok {
		score += 5
	}

	for _, col := range t.Columns {
		colLower := strings.ToLower(col.Name)
		if _, ok := tokens[colLower]; ok {
			score += 3
		} else {
			colParts := nonAlphaNum.ReplaceAllString(colLower, " ")
			for _, part := range strings.Fields(colParts) {
				if len(part) > 1 {
					if _, ok := tokens[part]; ok {
						score += 2
					}
				}
			}
		}
	}

	return score
}
