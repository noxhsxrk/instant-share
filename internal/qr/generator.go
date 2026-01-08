package qr

import (
	"strings"

	"github.com/skip2/go-qrcode"
)

func GenerateASCII(url string) (string, error) {
	qr, err := qrcode.New(url, qrcode.Medium)
	if err != nil {
		return "", err
	}

	bitmap := qr.Bitmap()
	var sb strings.Builder

	for y := 0; y < len(bitmap); y++ {
		for x := 0; x < len(bitmap[y]); x++ {
			if bitmap[y][x] {
				sb.WriteString("██")
			} else {
				sb.WriteString("  ")
			}
		}
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

func GenerateASCIICompact(url string) (string, error) {
	qr, err := qrcode.New(url, qrcode.Medium)
	if err != nil {
		return "", err
	}

	bitmap := qr.Bitmap()
	var sb strings.Builder

	for y := 0; y < len(bitmap); y += 2 {
		for x := 0; x < len(bitmap[y]); x++ {
			top := bitmap[y][x]
			bottom := false
			if y+1 < len(bitmap) {
				bottom = bitmap[y+1][x]
			}

			switch {
			case top && bottom:
				sb.WriteString("██")
			case top && !bottom:
				sb.WriteString("▀▀")
			case !top && bottom:
				sb.WriteString("▄▄")
			default:
				sb.WriteString("  ")
			}
		}
		sb.WriteString("\n")
	}

	return sb.String(), nil
}
