package main

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// ANSI color escape codes
const (
	ColorReset  = "\033[0m"
	ColorBold   = "\033[1m"
	ColorDim    = "\033[2m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorPurple = "\033[35m"
	ColorCyan   = "\033[36m"
	ColorWhite  = "\033[37m"
)

// Colored returns a string wrapped in the specified ANSI color code.
func Colored(colorCode, text string) string {
	return colorCode + text + ColorReset
}

// DisplayWidth calculates the visual width of a string (handling Unicode runes).
func DisplayWidth(s string) int {
	return utf8.RuneCountInString(s)
}

// PadString pads a string to a specific visual width with spaces.
func PadString(s string, width int, rightAlign bool) string {
	diff := width - DisplayWidth(s)
	if diff <= 0 {
		return s
	}
	padding := strings.Repeat(" ", diff)
	if rightAlign {
		return padding + s
	}
	return s + padding
}

// PrintTable renders a list of rows with a premium Unicode border layout.
func PrintTable(headers []string, rows [][]string) {
	if len(headers) == 0 {
		return
	}

	numCols := len(headers)
	colWidths := make([]int, numCols)

	// Initialize with header widths
	for i, h := range headers {
		colWidths[i] = DisplayWidth(h)
	}

	// Adjust widths based on data cells
	for _, row := range rows {
		for i := 0; i < numCols && i < len(row); i++ {
			width := DisplayWidth(row[i])
			if width > colWidths[i] {
				colWidths[i] = width
			}
		}
	}

	// 1. Draw top border: ┌───┬───┐
	var topSb strings.Builder
	topSb.WriteString("┌")
	for i, w := range colWidths {
		topSb.WriteString(strings.Repeat("─", w+2))
		if i < numCols-1 {
			topSb.WriteString("┬")
		}
	}
	topSb.WriteString("┐")
	fmt.Println(Colored(ColorDim, topSb.String()))

	// 2. Draw header: │ Header 1 │ Header 2 │
	var headerSb strings.Builder
	headerSb.WriteString(Colored(ColorDim, "│"))
	for i, h := range headers {
		padded := PadString(h, colWidths[i], false)
		headerSb.WriteString(" " + Colored(ColorBold+ColorCyan, padded) + " ")
		headerSb.WriteString(Colored(ColorDim, "│"))
	}
	fmt.Println(headerSb.String())

	// 3. Draw middle separator: ├───┼───┤
	var midSb strings.Builder
	midSb.WriteString("├")
	for i, w := range colWidths {
		midSb.WriteString(strings.Repeat("─", w+2))
		if i < numCols-1 {
			midSb.WriteString("┼")
		}
	}
	midSb.WriteString("┤")
	fmt.Println(Colored(ColorDim, midSb.String()))

	// 4. Draw data rows
	for _, row := range rows {
		var rowSb strings.Builder
		rowSb.WriteString(Colored(ColorDim, "│"))
		for i := 0; i < numCols; i++ {
			val := ""
			if i < len(row) {
				val = row[i]
			}
			// Clean any newline characters in table cell to avoid breaking borders
			val = strings.ReplaceAll(val, "\n", " ")
			val = strings.ReplaceAll(val, "\r", "")
			
			// Highlight numeric values or keep them clean
			padded := PadString(val, colWidths[i], false)
			rowSb.WriteString(" " + padded + " ")
			rowSb.WriteString(Colored(ColorDim, "│"))
		}
		fmt.Println(rowSb.String())
	}

	// 5. Draw bottom border: └───┴───┘
	var botSb strings.Builder
	botSb.WriteString("└")
	for i, w := range colWidths {
		botSb.WriteString(strings.Repeat("─", w+2))
		if i < numCols-1 {
			botSb.WriteString("┴")
		}
	}
	botSb.WriteString("┘")
	fmt.Println(Colored(ColorDim, botSb.String()))
}
