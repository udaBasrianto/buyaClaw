package knowledge

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/ledongthuc/pdf"
	"github.com/nguyenthenguyen/docx"
	"github.com/xuri/excelize/v2"
)

// ParseFile extracts plain text from a document based on its file extension.
// Supported: .txt, .md, .pdf, .docx, .xlsx, .xls, .csv
func ParseFile(filename string, data []byte) (string, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".txt", ".md", ".markdown", ".rst", ".log":
		return string(data), nil
	case ".pdf":
		return parsePDF(data)
	case ".docx":
		return parseDOCX(data)
	case ".xlsx", ".xls":
		return parseExcel(data)
	case ".csv":
		return parseCSV(data)
	default:
		// Try as plain text for unknown types
		if isLikelyText(data) {
			return string(data), nil
		}
		return "", fmt.Errorf("unsupported file format: %s", ext)
	}
}

// parsePDF extracts text from a PDF file.
func parsePDF(data []byte) (string, error) {
	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("parse pdf: %w", err)
	}

	var sb strings.Builder
	for i := 1; i <= r.NumPage(); i++ {
		page := r.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}
		sb.WriteString(text)
		sb.WriteString("\n")
	}
	return sb.String(), nil
}

// parseDOCX extracts text from a Word .docx file.
func parseDOCX(data []byte) (string, error) {
	// docx library requires a file path, so write to temp reader
	r, err := docx.ReadDocxFromMemory(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("parse docx: %w", err)
	}
	defer r.Close()

	doc := r.Editable()
	return doc.GetContent(), nil
}

// parseExcel extracts text from an Excel file (.xlsx/.xls).
func parseExcel(data []byte) (string, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("parse excel: %w", err)
	}
	defer f.Close()

	var sb strings.Builder
	for _, sheet := range f.GetSheetList() {
		rows, err := f.GetRows(sheet)
		if err != nil {
			continue
		}
		sb.WriteString("## Sheet: ")
		sb.WriteString(sheet)
		sb.WriteString("\n")
		for _, row := range rows {
			sb.WriteString(strings.Join(row, "\t"))
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}
	return sb.String(), nil
}

// parseCSV extracts text from a CSV file.
func parseCSV(data []byte) (string, error) {
	r := csv.NewReader(bytes.NewReader(data))
	r.LazyQuotes = true
	r.TrimLeadingSpace = true

	var sb strings.Builder
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		sb.WriteString(strings.Join(record, "\t"))
		sb.WriteString("\n")
	}
	return sb.String(), nil
}

// isLikelyText returns true if the data appears to be plain text.
func isLikelyText(data []byte) bool {
	if len(data) == 0 {
		return true
	}
	sample := data
	if len(sample) > 512 {
		sample = sample[:512]
	}
	for _, b := range sample {
		if b == 0 {
			return false
		}
	}
	return true
}

// DetectMimeType returns a simple MIME type based on file extension.
func DetectMimeType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".pdf":
		return "application/pdf"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".xls":
		return "application/vnd.ms-excel"
	case ".csv":
		return "text/csv"
	case ".md", ".markdown":
		return "text/markdown"
	default:
		return "text/plain"
	}
}
