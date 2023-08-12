package ini

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSection(t *testing.T) {
	data := `[section1]
	key1= value1
	[section2]
	key2=value2`
	ini := Load(data)
	if !ini.HasSection("section1") || !ini.HasSection("section2") {
		t.Error("fail to load ini file")
	}
	if ini.DefaultSectionName() != "default" {
		t.Error("fail to default section name")
	}
	sect := NewSection("newsect")
	sect.Add("key1", "value1")
	sect.Add("bool1", "true")
	sect.Add("bool2", "false")
	sect.Add("i64", "123456")

	ini.AddSection(sect)
	if !ini.HasSection("newsect") {
		t.Error("fail to add new section")
	}
	sect, err := ini.Section("newsect")
	require.Nil(t, err)
	require.Equal(t, 4, len(sect.Keys()))

	if sect.GetValueWithDefault("key1", "") != "value1" {
		t.Error("fail to get new section field")
	}

	if ret, err := sect.GetBool("bool1"); true {
		require.Nil(t, err)
		require.Equal(t, true, ret)
	}

	require.Equal(t, true, sect.GetBoolWithDefault("bool0", true))
	require.Equal(t, true, sect.GetBoolWithDefault("bool1", false))
	require.Equal(t, false, sect.GetBoolWithDefault("bool2", true))
	require.Equal(t, int64(123456), sect.GetInt64WithDefault("i64", 0))
	require.Equal(t, int64(99), sect.GetInt64WithDefault("i64_x", 99))
	require.Equal(t, int(123456), sect.GetIntWithDefault("i64", 0))
	require.Equal(t, int(99), sect.GetIntWithDefault("i64_x", 99))
	require.Equal(t, uint(123456), sect.GetUintWithDefault("i64", 0))
	require.Equal(t, uint(99), sect.GetUintWithDefault("i64_x", 99))
	require.Equal(t, float32(123456), sect.GetFloat32WithDefault("i64", 0))
	require.Equal(t, float32(99), sect.GetFloat32WithDefault("i64_x", 99))
	require.Equal(t, float64(123456), sect.GetFloat64WithDefault("i64", 0))

	require.Equal(t, true, ini.GetBoolWithDefault("newsect", "bool0", true))
	require.Equal(t, true, ini.GetBoolWithDefault("newsect", "bool1", false))
	require.Equal(t, false, ini.GetBoolWithDefault("newsect", "bool2", true))
	require.Equal(t, float64(99), ini.GetFloat64WithDefault("newsect", "i64_x", 99))
	require.Equal(t, int64(123456), ini.GetInt64WithDefault("newsect", "i64", 0))
	require.Equal(t, int64(99), ini.GetInt64WithDefault("newsect", "i64_x", 99))
	require.Equal(t, int(123456), ini.GetIntWithDefault("newsect", "i64", 0))
	require.Equal(t, int(99), ini.GetIntWithDefault("newsect", "i64_x", 99))
	require.Equal(t, uint(123456), ini.GetUintWithDefault("newsect", "i64", 0))
	require.Equal(t, uint(99), ini.GetUintWithDefault("newsect", "i64_x", 99))
	require.Equal(t, float32(123456), ini.GetFloat32WithDefault("newsect", "i64", 0))
	require.Equal(t, float32(99), ini.GetFloat32WithDefault("newsect", "i64_x", 99))
	require.Equal(t, float64(123456), ini.GetFloat64WithDefault("newsect", "i64", 0))
	require.Equal(t, float64(99), ini.GetFloat64WithDefault("newsect", "i64_x", 99))

	if ret, err := ini.GetBool("newsect", "bool1"); true {
		require.Nil(t, err)
		require.Equal(t, true, ret)
	}
	if _, err := ini.GetBool("newsect", "bool0"); true {
		require.NotNil(t, err)
	}

	if _, err := ini.GetFloat64("newsect", "i64_x"); true {
		require.NotNil(t, err)
	}
	if ret, err := ini.GetInt64("newsect", "i64"); true {
		require.Nil(t, err)
		require.Equal(t, int64(123456), ret)
	}
	if _, err := ini.GetInt64("newsect", "i64_x"); true {
		require.NotNil(t, err)
	}
	if ret, err := ini.GetInt("newsect", "i64"); true {
		require.Nil(t, err)
		require.Equal(t, int(123456), ret)
	}
	if _, err := ini.GetInt("newsect", "i64_x"); true {
		require.NotNil(t, err)
	}
	if ret, err := ini.GetUint("newsect", "i64"); true {
		require.Nil(t, err)
		require.Equal(t, uint(123456), ret)
	}
	if _, err := ini.GetUint("newsect", "i64_x"); true {
		require.NotNil(t, err)
	}
	if ret, err := ini.GetFloat32("newsect", "i64"); true {
		require.Nil(t, err)
		require.Equal(t, float32(123456), ret)
	}
	if _, err := ini.GetFloat32("newsect", "i64_x"); true {
		require.NotNil(t, err)
	}
	if ret, err := ini.GetFloat64("newsect", "i64"); true {
		require.Nil(t, err)
		require.Equal(t, float64(123456), ret)
	}
	if _, err := ini.GetFloat64("newsect", "i64_x"); true {
		require.NotNil(t, err)

	}
}

func TestNormalKey(t *testing.T) {
	data := `[section1]
    key1 = """this is one line"""
    [section2]
    key2 = value2`
	ini := Load(data)
	if ini.GetValueWithDefault("section1", "key1", "") != "this is one line" || ini.GetValueWithDefault("section2", "key2", "") != "value2" {
		t.Error("Fail to get key")
	}
}

func TestMultiLine(t *testing.T) {
	data := `[section1]
key1 = """this is a
multi line
test"""
[section2]
key2 = value2
`

	ini := Load(data)
	key1_value := `this is a
multi line
test`
	if ini.GetValueWithDefault("section1", "key1", "") != key1_value {
		t.Error("Fail to load ini with multi line keys")
	}
}

func TestContinuationLine(t *testing.T) {
	data := "[section1]\nkey1 = this is a \\\nmulti line \\\ntest\nkey2= this is key2\n[section2]\nkey2=value2"

	ini := Load(data)
	if ini.GetValueWithDefault("section1", "key1", "") != "this is a multi line test" {
		t.Error("Fail to load ini with Continuation char")
	}

	data = "[section1]\nkey1 = this is a line end with \\\\\nkey2= this is key2\n[section2]\nkey2=value2"
	ini = Load(data)
	if ini.GetValueWithDefault("section1", "key1", "") != "this is a line end with \\" {
		t.Error("Fail to load ini without Continuation char")
	}
}

func TestValueWithEscapeChar(t *testing.T) {
	data := "[section1]\nkey1 = this is a \\nmulti line\\ttest\nkey2= this is key2\n[section2]\nkey2=value2"
	ini := Load(data)
	if ini.GetValueWithDefault("section1", "key1", "") != "this is a \nmulti line\ttest" {
		t.Error("Fail to load ini with escape char")
	}
}

func TestToEscape(t *testing.T) {
	data := "string with escape char \r\n\t;# for testing"
	new_data := "string with escape char \\r\\n\\t\\;\\# for testing"
	if toEscape(data) != new_data {
		t.Error("Fail to convert escape string")
	}
}

func TestInlineComments(t *testing.T) {
	//inline comments must be start with ; or # and a space char before it
	data := "[section1]\nkey1 = this is a inline comment test ; comments ; do you know\nkeys=this is key2\n[section2]\nkey3=value3"
	ini := Load(data)
	if ini.GetValueWithDefault("section1", "key1", "") != "this is a inline comment test" {
		t.Error("Fail to load ini with inline comments")
	}

	data = "[section1]\nkey1 = this is a inline comment test;comments\nkeys=this is key2\n[section2]\nkey3=value3"
	ini = Load(data)
	if ini.GetValueWithDefault("section1", "key1", "") != "this is a inline comment test;comments" {
		t.Error("Fail to load ini with inline comments")
	}

	data = "[section1]\nkey1 = this is not a inline comment test \\;comments\nkeys=this is key2\n[section2]\nkey3=value3"

	ini = Load(data)
	if ini.GetValueWithDefault("section1", "key1", "") != "this is not a inline comment test ;comments" {
		t.Error("Fail to load ini without inline comments")
	}
}

func TestRemoveComments(t *testing.T) {
	s := "logfile=/var/log/supervisor/supervisord.log \\; ; (main log file;default $CWD/supervisord.log)"
	if removeComments(s) != "logfile=/var/log/supervisor/supervisord.log \\;" {
		t.Fail()
	}
}

func TestOctInValue(t *testing.T) {
	data := "[section1]\nkey1=this is \\141 oct test"
	ini := Load(data)
	if ini.GetValueWithDefault("section1", "key1", "") != "this is a oct test" {
		t.Error("fail to convert oct to char")
	}
}

func TestUnicodeValue(t *testing.T) {
	data := "[section1]\nkey1=this is \\x0061 unicode test"
	ini := Load(data)
	if ini.GetValueWithDefault("section1", "key1", "") != "this is a unicode test" {
		t.Error("fail to convert unicode to char")
	}
}

func TestIniWriteRead(t *testing.T) {
	buf := bytes.NewBuffer(make([]byte, 0))
	for i := 0; i < 100; i++ {
		sectionName := fmt.Sprintf("section_%d", i)
		fmt.Fprintf(buf, "[%s]\n", sectionName)
		for j := 0; j < 100; j++ {
			key := fmt.Sprintf("key_%d_%d", i, j)
			value := fmt.Sprintf("value_%d_%d", i, j)
			fmt.Fprintf(buf, "%s=%s\n", key, value)
		}
	}

	ini := Load(buf.String())
	ini = Load(ini.String())
	if len(ini.Sections()) != 100 {
		t.Error("fail to write&load ini")
	}

	for i := 0; i < 100; i++ {
		sectionName := fmt.Sprintf("section_%d", i)
		for j := 0; j < 100; j++ {
			key := fmt.Sprintf("key_%d_%d", i, j)
			value := fmt.Sprintf("value_%d_%d", i, j)
			if v, err := ini.GetValue(sectionName, key); err != nil || v != value {
				t.Error("fail to write&load ini")
			}
		}
	}
}

func TestMultiLine2(t *testing.T) {
	data := `[section1]
key1 : this is a ;comment1
 multi line #comment
 test #comment
[section2]
key2 = value2
		`

	ini := Load(data)
	key1_value := "this is a\nmulti line\ntest"
	if ini.GetValueWithDefault("section1", "key1", "") != key1_value {
		t.Error("Fail to load ini with multi line keys")
	}
}

func TestResolve(t *testing.T) {
	data := `[section1]
key1 : this is ${VAR1} ;comment1
 multi line #comment
 test #comment
key2 : this is ${VAR2}
[section2]
key3 = ${VAR3}
key4 = ${section1.key2}
`

	os.Setenv("VAR1", "VALUE1")
	predef := map[string]string{
		"VAR2": "VALUE2",
		"VAR3": "VALUE-INCLUDES-${VAR1}",
	}

	ini := Load(data)
	ini.Resolve(predef, true)

	key1_value := "this is VALUE1\nmulti line\ntest"
	if ini.GetValueWithDefault("section1", "key1", "") != key1_value {
		t.Error("Fail to load ini with multi line keys, key1")
	}

	key2_value := "this is VALUE2"
	if ini.GetValueWithDefault("section1", "key2", "") != key2_value {
		t.Error("Fail to load ini with multi line keys, key2")
	}

	key3_value := "VALUE-INCLUDES-VALUE1"
	if ini.GetValueWithDefault("section2", "key3", "") != key3_value {
		t.Error("Fail to load ini with multi line keys, key3")
	}

	key4_value := key2_value
	if ini.GetValueWithDefault("section2", "key4", "") != key4_value {
		t.Error("Fail to load ini with multi line keys, key4")
	}
}
