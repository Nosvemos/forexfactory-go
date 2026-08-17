package forexfactory

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/xuri/excelize/v2"
)

// WriteExcel exports a list of events to an Excel (.xlsx) file with professional formatting and styling.
func WriteExcel(events []Event, filePath string) error {
	dir := filepath.Dir(filePath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory for excel file: %w", err)
		}
	}

	f := excelize.NewFile()
	defer f.Close()

	sheetName := "Economic Calendar"
	index, err := f.NewSheet(sheetName)
	if err != nil {
		return fmt.Errorf("failed to create sheet: %w", err)
	}
	f.SetActiveSheet(index)
	_ = f.DeleteSheet("Sheet1") // Remove default sheet

	// Define styles
	headerStyle, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold:  true,
			Color: "#FFFFFF",
			Size:  11,
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#1F497D"},
			Pattern: 1,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create header style: %w", err)
	}

	highImpactStyle, _ := f.NewStyle(&excelize.Style{
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#FFC7CE"}, Pattern: 1},
		Font: &excelize.Font{Color: "#9C0006", Bold: true},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	mediumImpactStyle, _ := f.NewStyle(&excelize.Style{
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#FFEB9C"}, Pattern: 1},
		Font: &excelize.Font{Color: "#9C6500", Bold: true},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	lowImpactStyle, _ := f.NewStyle(&excelize.Style{
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#C6EFCE"}, Pattern: 1},
		Font: &excelize.Font{Color: "#006100", Bold: true},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})

	centerStyle, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})

	headers := []string{
		"ID", "Title", "Currency", "Date (UTC)", "Impact",
		"Forecast", "Previous", "Actual", "Deviation", "Market Bias",
		"All Day", "Tentative",
	}

	for colIdx, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(colIdx+1, 1)
		_ = f.SetCellValue(sheetName, cell, h)
		_ = f.SetCellStyle(sheetName, cell, cell, headerStyle)
	}

	for rowIdx, e := range events {
		row := rowIdx + 2

		var devStr string
		dev, errDev := e.Deviation()
		if errDev == nil {
			devStr = fmt.Sprintf("%.2f", dev)
		} else {
			devStr = "-"
		}

		bias := e.MarketBias()

		_ = f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), e.ID)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), e.Title)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("C%d", row), e.Currency)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("D%d", row), e.Date.UTC().Format(time.RFC3339))
		_ = f.SetCellValue(sheetName, fmt.Sprintf("E%d", row), string(e.Impact))
		_ = f.SetCellValue(sheetName, fmt.Sprintf("F%d", row), e.Forecast)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("G%d", row), e.Previous)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("H%d", row), e.Actual)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("I%d", row), devStr)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("J%d", row), bias)
		_ = f.SetCellValue(sheetName, fmt.Sprintf("K%d", row), strconv.FormatBool(e.IsAllDay))
		_ = f.SetCellValue(sheetName, fmt.Sprintf("L%d", row), strconv.FormatBool(e.IsTentative))

		// Center align some columns
		_ = f.SetCellStyle(sheetName, fmt.Sprintf("C%d", row), fmt.Sprintf("C%d", row), centerStyle)
		_ = f.SetCellStyle(sheetName, fmt.Sprintf("I%d", row), fmt.Sprintf("J%d", row), centerStyle)

		// Apply Impact color styling
		switch e.Impact {
		case ImpactHigh:
			_ = f.SetCellStyle(sheetName, fmt.Sprintf("E%d", row), fmt.Sprintf("E%d", row), highImpactStyle)
		case ImpactMedium:
			_ = f.SetCellStyle(sheetName, fmt.Sprintf("E%d", row), fmt.Sprintf("E%d", row), mediumImpactStyle)
		case ImpactLow:
			_ = f.SetCellStyle(sheetName, fmt.Sprintf("E%d", row), fmt.Sprintf("E%d", row), lowImpactStyle)
		default:
			_ = f.SetCellStyle(sheetName, fmt.Sprintf("E%d", row), fmt.Sprintf("E%d", row), centerStyle)
		}
	}

	// Auto-fit column widths roughly
	colWidths := map[string]float64{
		"A": 12, "B": 38, "C": 10, "D": 22, "E": 12,
		"F": 12, "G": 12, "H": 12, "I": 12, "J": 14,
		"K": 10, "L": 12,
	}
	for col, width := range colWidths {
		_ = f.SetColWidth(sheetName, col, col, width)
	}

	if err := f.SaveAs(filePath); err != nil {
		return fmt.Errorf("failed to save excel workbook to %q: %w", filePath, err)
	}

	return nil
}
