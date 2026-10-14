package sink

import (
	"fmt"
	"os"

	"github.com/xuri/excelize/v2"
)

type excelSink struct {
	file    string
	headers []string
	f       *excelize.File
	sheet   string
	row     int
}

func newExcelSink(file string, headers []string) *excelSink {
	if file == "" {
		file = "result.xlsx"
	}
	return &excelSink{file: file, headers: headers, sheet: "Result", row: 1}
}

func (s *excelSink) Open() error {
	s.f = excelize.NewFile()
	if err := s.f.SetSheetName(s.f.GetSheetName(0), s.sheet); err != nil {
		return err
	}

	headerStyle, err := s.f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"D9E1F2"}, Pattern: 1},
	})
	if err != nil {
		return err
	}

	for j, h := range s.headers {
		cell, _ := excelize.CoordinatesToCellName(j+1, s.row)
		if err := s.f.SetCellValue(s.sheet, cell, h); err != nil {
			return err
		}
	}
	if len(s.headers) > 0 {
		start, _ := excelize.CoordinatesToCellName(1, s.row)
		end, _ := excelize.CoordinatesToCellName(len(s.headers), s.row)
		s.f.SetCellStyle(s.sheet, start, end, headerStyle)
	}
	s.row++
	return nil
}

func (s *excelSink) Write(row []string) error {
	for j, v := range row {
		cell, _ := excelize.CoordinatesToCellName(j+1, s.row)
		if err := s.f.SetCellValue(s.sheet, cell, v); err != nil {
			return err
		}
	}
	s.row++
	return nil
}

func (s *excelSink) Close() error {
	if err := s.f.SaveAs(s.file); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "已输出 %d 行到 %s\n", s.row-2, s.file)
	return s.f.Close()
}
