package charset

import (
	"strings"

	enc "golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/unicode"
)

var encodingMap = map[string]enc.Encoding{
	"UTF-8":             unicode.UTF8,
	"ISO-2022-JP":       japanese.ISO2022JP,
	"EUC-KR":            korean.EUCKR,
	"SJIS":              japanese.ShiftJIS,
	"CP932":             japanese.ShiftJIS,
	"SHIFT_JIS":         japanese.ShiftJIS,
	"EUC-JP":            japanese.EUCJP,
	"UTF-16":            unicode.UTF16(unicode.BigEndian, unicode.UseBOM),
	"UTF-16BE":          unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM),
	"UTF-16LE":          unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM),
	"CP437":             charmap.CodePage437,
	"CP850":             charmap.CodePage850,
	"CP852":             charmap.CodePage852,
	"CP855":             charmap.CodePage855,
	"CP858":             charmap.CodePage858,
	"CP860":             charmap.CodePage860,
	"CP862":             charmap.CodePage862,
	"CP863":             charmap.CodePage863,
	"CP865":             charmap.CodePage865,
	"CP866":             charmap.CodePage866,
	"LATIN-1":           charmap.ISO8859_1,
	"ISO-8859-1":        charmap.ISO8859_1,
	"ISO-8859-2":        charmap.ISO8859_2,
	"ISO-8859-3":        charmap.ISO8859_3,
	"ISO-8859-4":        charmap.ISO8859_4,
	"ISO-8859-5":        charmap.ISO8859_5,
	"ISO-8859-6":        charmap.ISO8859_6,
	"ISO-8859-7":        charmap.ISO8859_7,
	"ISO-8859-8":        charmap.ISO8859_8,
	"ISO-8859-10":       charmap.ISO8859_10,
	"ISO-8859-13":       charmap.ISO8859_13,
	"ISO-8859-14":       charmap.ISO8859_14,
	"ISO-8859-15":       charmap.ISO8859_15,
	"ISO-8859-16":       charmap.ISO8859_16,
	"KOI8R":             charmap.KOI8R,
	"KOI8U":             charmap.KOI8U,
	"MACINTOSH":         charmap.Macintosh,
	"MACINTOSHCYRILLIC": charmap.MacintoshCyrillic,
	"WINDOWS1250":       charmap.Windows1250,
	"WINDOWS1251":       charmap.Windows1251,
	"WINDOWS1252":       charmap.Windows1252,
	"WINDOWS1253":       charmap.Windows1253,
	"WINDOWS1254":       charmap.Windows1254,
	"WINDOWS1255":       charmap.Windows1255,
	"WINDOWS1256":       charmap.Windows1256,
	"WINDOWS1257":       charmap.Windows1257,
	"WINDOWS1258":       charmap.Windows1258,
	"WINDOWS874":        charmap.Windows874,
	"XUSERDEFINED":      charmap.XUserDefined,
	"HZ-GB2312":         simplifiedchinese.HZGB2312,
	// "Big5":              traditionalchinese.Big5,
}

func Encoding(charset string) (enc.Encoding, bool) {
	enc, ok := encodingMap[strings.ToUpper(charset)]
	return enc, ok
}
