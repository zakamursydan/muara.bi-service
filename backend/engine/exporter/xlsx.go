package exporter

import (
	"fmt"
	"io"
	"strings"

	"github.com/xuri/excelize/v2"
)

const (
	maxSheetNameLen = 31
	minColumnWidth  = 8.0
	maxColumnWidth  = 48.0
	headerFill      = "#0891b2"
)

// WriteXLSX renders every sheet into its own worksheet using the excelize
// streaming writer. Rows are flushed incrementally, so a full-dashboard export
// with many widgets stays within a bounded memory footprint.
func WriteXLSX(w io.Writer, sheets []Sheet) (int64, error) {
	f := excelize.NewFile()
	defer f.Close()

	headerStyle, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "#FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{headerFill}, Pattern: 1},
		Alignment: &excelize.Alignment{Vertical: "center"},
	})
	if err != nil {
		return 0, fmt.Errorf("failed to build xlsx header style: %w", err)
	}

	usedNames := make(map[string]struct{}, len(sheets))
	var total int64

	for i, sheet := range sheets {
		name := uniqueSheetName(sheet.Name, i, usedNames)

		if i == 0 {
			if err := f.SetSheetName(f.GetSheetName(0), name); err != nil {
				return total, fmt.Errorf("failed to rename first worksheet to %q: %w", name, err)
			}
		} else if _, err := f.NewSheet(name); err != nil {
			return total, fmt.Errorf("failed to create worksheet %q: %w", name, err)
		}

		stream, err := f.NewStreamWriter(name)
		if err != nil {
			return total, fmt.Errorf("failed to open stream writer for %q: %w", name, err)
		}

		header := make([]any, len(sheet.Columns))
		for c, col := range sheet.Columns {
			header[c] = excelize.Cell{StyleID: headerStyle, Value: col.Name}
		}
		if err := stream.SetRow("A1", header); err != nil {
			return total, fmt.Errorf("failed to write header for %q: %w", name, err)
		}

		for r, row := range sheet.Rows {
			cells := make([]any, len(sheet.Columns))
			for c, col := range sheet.Columns {
				cells[c] = cellValue(row[col.Name])
			}
			ref, err := excelize.CoordinatesToCellName(1, r+2)
			if err != nil {
				return total, fmt.Errorf("failed to resolve cell for %q row %d: %w", name, r+2, err)
			}
			if err := stream.SetRow(ref, cells); err != nil {
				return total, fmt.Errorf("failed to write row for %q: %w", name, err)
			}
			total++
		}

		applyColumnWidths(stream, sheet)

		if err := stream.Flush(); err != nil {
			return total, fmt.Errorf("failed to flush worksheet %q: %w", name, err)
		}
	}

	f.SetActiveSheet(0)
	if err := f.Write(w); err != nil {
		return total, fmt.Errorf("failed to write xlsx workbook: %w", err)
	}
	return total, nil
}

func applyColumnWidths(stream *excelize.StreamWriter, sheet Sheet) {
	for c, col := range sheet.Columns {
		width := float64(len(col.Name))
		for _, row := range sheet.Rows {
			if l := len(FormatValue(row[col.Name])); float64(l) > width {
				width = float64(l)
			}
		}
		width += 2
		if width < minColumnWidth {
			width = minColumnWidth
		}
		if width > maxColumnWidth {
			width = maxColumnWidth
		}
		_ = stream.SetColWidth(c+1, c+1, width)
	}
}

// cellValue keeps unsupported Go types out of the excelize writer.
func cellValue(v any) any {
	switch val := v.(type) {
	case nil:
		return ""
	case []byte:
		return string(val)
	default:
		return val
	}
}

var sheetNameForbidden = strings.NewReplacer(
	"[", "", "]", "", ":", "", "*", "", "?", "", "/", "", "\\", "",
)

// uniqueSheetName produces an Excel-legal worksheet name that does not collide
// with an already used name.
func uniqueSheetName(raw string, index int, used map[string]struct{}) string {
	name := strings.TrimSpace(sheetNameForbidden.Replace(raw))
	if name == "" {
		name = fmt.Sprintf("Sheet%d", index+1)
	}
	if len(name) > maxSheetNameLen {
		name = name[:maxSheetNameLen]
	}

	base := name
	for suffix := 2; ; suffix++ {
		if _, exists := used[name]; !exists {
			used[name] = struct{}{}
			return name
		}
		tail := fmt.Sprintf("_%d", suffix)
		trimmed := base
		if len(trimmed)+len(tail) > maxSheetNameLen {
			trimmed = trimmed[:maxSheetNameLen-len(tail)]
		}
		name = trimmed + tail
	}
}
