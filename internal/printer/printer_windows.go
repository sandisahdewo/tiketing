//go:build windows

package printer

import (
	"github.com/alexbrainman/printer"
)

func List(_ string) ([]string, error) {
	return printer.ReadNames()
}

func Default() string {
	name, _ := printer.Default()
	return name
}

func Print(name string, data []byte, _ string) (string, error) {
	p, err := printer.Open(name)
	if err != nil {
		return "", err
	}
	defer p.Close()
	if err := p.StartRawDocument("Nota Tiket"); err != nil {
		return "", err
	}
	if err := p.StartPage(); err != nil {
		p.EndDocument()
		return "", err
	}
	if _, err := p.Write(data); err != nil {
		p.EndPage()
		p.EndDocument()
		return "", err
	}
	if err := p.EndPage(); err != nil {
		p.EndDocument()
		return "", err
	}
	return "", p.EndDocument()
}
