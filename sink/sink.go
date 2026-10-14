package sink

import "errors"

type Sink interface {
	Open() error
	Write(row []string) error
	Close() error
}

var ErrUnknownType = errors.New("unknown sink type")

func New(typ, outFile string, headers []string) (Sink, error) {
	switch typ {
	case "excel", "xlsx", "":
		return newExcelSink(outFile, headers), nil
	default:
		return nil, ErrUnknownType
	}
}
