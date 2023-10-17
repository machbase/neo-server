package util_test

import (
	"testing"
	"time"

	"github.com/machbase/neo-server/mods/util"
	"github.com/stretchr/testify/require"
)

func TestTimeFormatter(t *testing.T) {
	ts := time.Unix(0, 1692907084548634123)

	// Format
	tf := util.NewTimeFormatter(util.Timeformat("ns"), util.TimeLocation(time.UTC))
	result := tf.Format(ts)
	require.Equal(t, "1692907084548634123", result)

	tf = util.NewTimeFormatter(util.Timeformat("us"), util.TimeLocation(time.UTC))
	result = tf.Format(ts)
	require.Equal(t, "1692907084548634", result)

	tf = util.NewTimeFormatter(util.Timeformat("ms"), util.TimeLocation(time.UTC))
	result = tf.Format(ts)
	require.Equal(t, "1692907084548", result)

	tf = util.NewTimeFormatter(util.Timeformat("s"), util.TimeLocation(time.UTC))
	result = tf.Format(ts)
	require.Equal(t, "1692907084", result)

	tf = util.NewTimeFormatter(util.Timeformat("DEFAULT"), util.TimeLocation(nil))
	result = tf.Format(ts)
	require.Equal(t, "2023-08-24 19:58:04.548", result)

	tf = util.NewTimeFormatter(util.Timeformat("DEFAULT"), util.TimeZoneFallback("Wrong TZ", time.UTC))
	result = tf.Format(ts)
	require.Equal(t, "2023-08-24 19:58:04.548", result)

	tf = util.NewTimeFormatter(util.Timeformat("DEFAULT"), util.TimeZoneFallback("KST", time.UTC))
	result = tf.Format(ts)
	require.Equal(t, "2023-08-25 04:58:04.548", result)

	tf = util.NewTimeFormatter(util.Timeformat("RFC822"), util.TimeZoneFallback("KST", time.UTC))
	result = tf.Format(ts)
	require.Equal(t, "25 Aug 23 04:58 KST", result)

	tf = util.NewTimeFormatter(util.Timeformat("RFC3339"), util.TimeZoneFallback("KST", time.UTC))
	result = tf.Format(ts)
	require.Equal(t, "2023-08-25T04:58:04+09:00", result)

	tf = util.NewTimeFormatter(util.Timeformat("RFC3339NANO"), util.TimeZoneFallback("KST", time.UTC))
	result = tf.Format(ts)
	require.Equal(t, "2023-08-25T04:58:04.548634123+09:00", result)

	// EpochOrFormat
	tf = util.NewTimeFormatter(util.Timeformat("ns"), util.TimeLocation(time.UTC))
	obj := tf.FormatEpoch(ts)
	require.Equal(t, int64(1692907084548634123), obj)

	tf = util.NewTimeFormatter(util.Timeformat("us"), util.TimeLocation(time.UTC))
	obj = tf.FormatEpoch(ts)
	require.Equal(t, int64(1692907084548634), obj)

	tf = util.NewTimeFormatter(util.Timeformat("ms"), util.TimeLocation(time.UTC))
	obj = tf.FormatEpoch(ts)
	require.Equal(t, int64(1692907084548), obj)

	tf = util.NewTimeFormatter(util.Timeformat("s"), util.TimeLocation(time.UTC))
	obj = tf.FormatEpoch(ts)
	require.Equal(t, int64(1692907084), obj)

	tf = util.NewTimeFormatter(util.Timeformat("DEFAULT"), util.TimeLocation(nil))
	obj = tf.FormatEpoch(ts)
	require.Equal(t, "2023-08-24 19:58:04.548", obj)

	tf = util.NewTimeFormatter(util.Timeformat("DEFAULT"), util.TimeZoneFallback("Wrong TZ", time.UTC))
	obj = tf.FormatEpoch(ts)
	require.Equal(t, "2023-08-24 19:58:04.548", obj)

	// Sql Timeformat
	sqltf := util.ToTimeformatSql("YYYY-MM-DD HH24:MI:SS.nnnnnnnnn")
	require.Equal(t, "2006-01-02 15:04:05.999999999", sqltf)

	sqltf = util.ToTimeformatSql("YYYY-MM-DD HH24:MI:SS.mmmuuunnn")
	require.Equal(t, "2006-01-02 15:04:05.999999999", sqltf)

	tf = util.NewTimeFormatter(util.Timeformat(sqltf), util.TimeLocation(time.UTC))
	obj = tf.FormatEpoch(ts)
	require.Equal(t, "2023-08-24 19:58:04.548634123", obj)

	// Ansi Timeformat
	ansitf := util.ToTimeformatAnsi("yyyy-mm-dd hh:nn:ss.fffffffff")
	tf = util.NewTimeFormatter(util.Timeformat(ansitf), util.TimeLocation(time.UTC))
	obj = tf.FormatEpoch(ts)
	require.Equal(t, "2023-08-24 19:58:04.548634123", obj)
}

func TestToFloat32(t *testing.T) {
	testOne := func(o any, expect float32, expectErr string) {
		ret, err := util.ToFloat32(o)
		if expectErr == "" {
			require.Nil(t, err)
			require.Equal(t, expect, ret)
		} else {
			require.NotNil(t, err)
			require.Equal(t, expectErr, err.Error())
		}
	}
	testErr := func(o any, expectErr string) {
		testOne(o, -1, expectErr)
	}
	testIt := func(o any, expect float32) {
		testOne(o, expect, "")
	}

	str := "2s"
	testErr(true, "incompatible conv 'true' (bool) to float32")
	testErr(str, "incompatible conv '0' (float64) to float32, strconv.ParseFloat: parsing \"2s\": invalid syntax")
	testErr(&str, "incompatible conv '0' (float64) to float32, strconv.ParseFloat: parsing \"2s\": invalid syntax")

	str = "1.23"
	testIt(str, float32(1.23))
	testIt(&str, float32(1.23))

	f32 := float32(3.1415)
	testIt(f32, float32(3.1415))
	testIt(&f32, float32(3.1415))

	f64 := 3.1415
	testIt(f64, float32(3.1415))
	testIt(&f64, float32(3.1415))

	ival := 123
	testIt(ival, float32(123))
	testIt(&ival, float32(123))
}

func TestToFloat64(t *testing.T) {
	testOne := func(o any, expect float64, expectErr string) {
		ret, err := util.ToFloat64(o)
		if expectErr == "" {
			require.Nil(t, err)
			require.Equal(t, expect, ret)
		} else {
			require.NotNil(t, err)
			require.Equal(t, expectErr, err.Error())
		}
	}
	testErr := func(o any, expectErr string) {
		testOne(o, -1, expectErr)
	}
	testIt := func(o any, expect float64) {
		testOne(o, expect, "")
	}

	str := "2s"
	testErr(true, "incompatible conv 'true' (bool) to float64")
	testErr(str, "incompatible conv '0' (float64) to float64, strconv.ParseFloat: parsing \"2s\": invalid syntax")
	testErr(&str, "incompatible conv '0' (float64) to float64, strconv.ParseFloat: parsing \"2s\": invalid syntax")

	str = "1.23"
	testIt(str, float64(1.23))
	testIt(&str, float64(1.23))

	f32 := float32(3.14150)
	testIt(f32, float64(float32(3.14150)))
	testIt(&f32, float64(float32(3.14150)))

	f64 := 3.1415
	testIt(f64, 3.1415)
	testIt(&f64, 3.1415)

	ival := 123
	testIt(ival, float64(123))
	testIt(&ival, float64(123))
}

func TestParseInt(t *testing.T) {
	_, err := util.ParseInt("12.1")
	require.NotNil(t, err)

	v, err := util.ParseInt("12")
	require.Nil(t, err)
	require.Equal(t, 12, v)

	_, err = util.ParseInt8("12.1")
	require.NotNil(t, err)

	v8, err := util.ParseInt8("12")
	require.Nil(t, err)
	require.Equal(t, int8(12), v8)

	_, err = util.ParseInt16("12.1")
	require.NotNil(t, err)

	v16, err := util.ParseInt16("12")
	require.Nil(t, err)
	require.Equal(t, int16(12), v16)

	_, err = util.ParseInt16("12.1")
	require.NotNil(t, err)

	v32, err := util.ParseInt32("12")
	require.Nil(t, err)
	require.Equal(t, int32(12), v32)

	_, err = util.ParseInt16("12.1")
	require.NotNil(t, err)

	v64, err := util.ParseInt64("12")
	require.Nil(t, err)
	require.Equal(t, int64(12), v64)
}

func TestParseIP(t *testing.T) {
	_, err := util.ParseIP("127.0.0.300")
	require.NotNil(t, err)

	ip, err := util.ParseIP("127.0.0.1")
	require.Nil(t, err)
	require.Equal(t, "127.0.0.1", ip.String())
}

func TestConvTime(t *testing.T) {
	ts := time.Now()
	util.StandardTimeNow = func() time.Time { return ts }

	var ret time.Time
	var err error

	_, err = util.ToTime("wrong")
	require.NotNil(t, err)
	require.Equal(t, "incompatible conv 'wrong' (string) to time.Time", err.Error())

	_, err = util.ToTime(true)
	require.NotNil(t, err)
	require.Equal(t, "incompatible conv 'true' (bool) to time.Time", err.Error())

	_, err = util.ToTime("now * 2s")
	require.NotNil(t, err)
	require.Equal(t, "incompatible conv 'now * 2s' (string) to time.Time", err.Error())

	_, err = util.ToTime("now - 2?")
	require.NotNil(t, err)
	require.Equal(t, "incompatible conv 'now - 2?', time: unknown unit \"?\" in duration \"2?\"", err.Error())

	ret, err = util.ToTime("now")
	require.Nil(t, err)
	require.Equal(t, ts, ret)

	ret, err = util.ToTime(" now ")
	require.Nil(t, err)
	require.Equal(t, ts, ret)

	ret, err = util.ToTime("now + 12.5s")
	require.Nil(t, err)
	require.Equal(t, ts.Add(12500*time.Millisecond), ret)

	ret, err = util.ToTime("now - 12.5s")
	require.Nil(t, err)
	require.Equal(t, ts.Add(-1*12500*time.Millisecond), ret)

	sval := "now - -12.5s"
	ret, err = util.ToTime(&sval)
	require.Nil(t, err)
	require.Equal(t, ts.Add(12500*time.Millisecond), ret)

	fval := float64(ts.UnixNano())
	ret, err = util.ToTime(fval)
	require.Nil(t, err)
	require.Equal(t, ts.UnixMilli(), ret.UnixMilli())

	ret, err = util.ToTime(&fval)
	require.Nil(t, err)
	require.Equal(t, ts.UnixMilli(), ret.UnixMilli())

	ival := ts.UnixNano()
	ret, err = util.ToTime(ival)
	require.Nil(t, err)
	require.Equal(t, ts.UnixNano(), ret.UnixNano())

	ret, err = util.ToTime(&ival)
	require.Nil(t, err)
	require.Equal(t, ts.UnixNano(), ret.UnixNano())

	ret, err = util.ToTime(int32(ival))
	require.Nil(t, err)
	require.Equal(t, int32(ts.UnixNano()), int32(ret.UnixNano()))

	ret, err = util.ToTime(int16(ival))
	require.Nil(t, err)
	require.Equal(t, int16(ts.UnixNano()), int16(ret.UnixNano()))

	ret, err = util.ToTime(int8(ival))
	require.Nil(t, err)
	require.Equal(t, int8(ts.UnixNano()), int8(ret.UnixNano()))

	ret, err = util.ToTime(int(ival))
	require.Nil(t, err)
	require.Equal(t, int(ts.UnixNano()), int(ret.UnixNano()))

	ret, err = util.ToTime(ts)
	require.Nil(t, err)
	require.Equal(t, ts.UnixNano(), ret.UnixNano())

	ret, err = util.ToTime(&ts)
	require.Nil(t, err)
	require.Equal(t, ts.UnixNano(), ret.UnixNano())
}

func TestTimeFormat(t *testing.T) {
	var ret time.Time
	var err error

	ret, err = util.ParseTime("1691800174123456789", "ns", nil)
	require.Nil(t, err)
	ts := time.Unix(1691800174, 123456789)
	require.Equal(t, ts, ret)

	ret, err = util.ParseTime("1691800174123456", "us", nil)
	require.Nil(t, err)
	ts = time.Unix(1691800174, 123456000)
	require.Equal(t, ts, ret)

	ret, err = util.ParseTime("1691800174123", "ms", nil)
	require.Nil(t, err)
	ts = time.Unix(1691800174, 123000000)
	require.Equal(t, ts, ret)

	ret, err = util.ParseTime("1691800174", "s", nil)
	require.Nil(t, err)
	ts = time.Unix(1691800174, 0)
	require.Equal(t, ts, ret)

	require.Nil(t, err)
	ret, err = util.ParseTime("2023-08-12 00:29:34.123", "2006-01-02 15:04:05.999", time.UTC)
	require.Nil(t, err)
	ts = time.Unix(1691800174, 123000000).UTC()
	require.Equal(t, ts, ret)
}

func TestConvDuration(t *testing.T) {
	var ret time.Duration
	var err error

	_, err = util.ToDuration("wrong")
	require.NotNil(t, err)
	require.Equal(t, "time: invalid duration \"wrong\"", err.Error())

	_, err = util.ToDuration(true)
	require.NotNil(t, err)
	require.Equal(t, "incompatible conv 'true' (bool) to time.Duration", err.Error())

	ret, err = util.ToDuration("1d")
	require.Nil(t, err)
	require.Equal(t, 24*time.Hour, ret)

	ret, err = util.ToDuration("-1d2h3m")
	require.Nil(t, err)
	require.Equal(t, -1*(24*time.Hour+2*time.Hour+3*time.Minute), ret)

	ret, err = util.ToDuration("1s")
	require.Nil(t, err)
	require.Equal(t, time.Second, ret)

	dur := time.Duration(123*time.Second + 456*time.Millisecond)

	i64 := int64(dur)
	ret, err = util.ToDuration(i64)
	require.Nil(t, err)
	require.Equal(t, dur, ret)

	ret, err = util.ToDuration(&i64)
	require.Nil(t, err)
	require.Equal(t, dur, ret)

	ret, err = util.ToDuration(int32(i64))
	require.Nil(t, err)
	require.Equal(t, int32(dur), int32(ret))

	ret, err = util.ToDuration(int16(i64))
	require.Nil(t, err)
	require.Equal(t, int16(dur), int16(ret))

	ret, err = util.ToDuration(int8(i64))
	require.Nil(t, err)
	require.Equal(t, int8(dur), int8(ret))

	ret, err = util.ToDuration(int(i64))
	require.Nil(t, err)
	require.Equal(t, int(dur), int(ret))

	f64 := float64(dur)
	ret, err = util.ToDuration(f64)
	require.Nil(t, err)
	require.Equal(t, dur, ret)

	ret, err = util.ToDuration(&f64)
	require.Nil(t, err)
	require.Equal(t, dur, ret)

	f32 := float32(123*time.Second + 456*time.Millisecond)
	ret, err = util.ToDuration(f32)
	require.Nil(t, err)
	require.Equal(t, f32, float32(ret.Nanoseconds()))

	ret, err = util.ToDuration(&f32)
	require.Nil(t, err)
	require.Equal(t, f32, float32(ret.Nanoseconds()))

}

func TestTimeZone(t *testing.T) {
	// for k, v := range util.Timezones {
	// 	fmt.Printf("%-8s: \"%s\",\n", `"`+k+`"`, v[0])
	// }
	for tz, expect := range timezoneTests {
		loc, err := util.GetTimeLocation(tz)
		require.Nil(t, err)
		require.Equal(t, expect, loc.String())
	}
}

var timezoneTests = map[string]string{
	"GHST":      "Africa/Accra",
	"HAT":       "America/Adak",
	"AET":       "Australia/ACT",
	"GMT-1":     "Etc/GMT+1",
	"COST":      "America/Bogota",
	"HKST":      "Asia/Hong_Kong",
	"RET":       "Indian/Reunion",
	"CKT":       "Pacific/Rarotonga",
	"ALMT":      "Asia/Almaty",
	"LHST":      "Australia/LHI",
	"GMT-4":     "Etc/GMT+4",
	"MDT":       "America/Boise",
	"BNT":       "Asia/Brunei",
	"WIT":       "Asia/Jayapura",
	"PKST":      "Asia/Karachi",
	"GMT-11":    "Etc/GMT+11",
	"CAT":       "Africa/Blantyre",
	"BRT":       "America/Araguaina",
	"OMSST":     "Asia/Omsk",
	"GMT+5":     "Etc/GMT-5",
	"CCT":       "Indian/Cocos",
	"SST":       "Pacific/Midway",
	"AZT":       "Asia/Baku",
	"GMT+2":     "Etc/GMT-2",
	"TKT":       "Pacific/Fakaofo",
	"EGT":       "America/Scoresbysund",
	"WITA":      "Asia/Makassar",
	"NOVT":      "Asia/Novosibirsk",
	"SAKT":      "Asia/Sakhalin",
	"FKT":       "Atlantic/Stanley",
	"MVT":       "Indian/Maldives",
	"CLT":       "America/Punta_Arenas",
	"NZT":       "Antarctica/McMurdo",
	"AQTT":      "Asia/Aqtau",
	"PHST":      "Asia/Manila",
	"AEST":      "Australia/ACT",
	"GMT+6":     "Etc/GMT-6",
	"TOST":      "Pacific/Tongatapu",
	"ACST":      "America/Eirunepe",
	"AWST":      "Antarctica/Casey",
	"TLT":       "Asia/Dili",
	"-00":       "Factory",
	"CHAT":      "NZ-CHAT",
	"WAKT":      "Pacific/Wake",
	"DDUT":      "Antarctica/DumontDUrville",
	"KRAT":      "Asia/Barnaul",
	"TMT":       "Asia/Ashgabat",
	"HOVT":      "Asia/Hovd",
	"PHOT":      "Pacific/Enderbury",
	"CAST":      "Africa/Khartoum",
	"BRST":      "America/Araguaina",
	"ECT":       "America/Guayaquil",
	"AZST":      "Asia/Baku",
	"ULAST":     "Asia/Ulaanbaatar",
	"EET":       "Africa/Cairo",
	"AT":        "America/Anguilla",
	"IST":       "Asia/Calcutta",
	"MMT":       "Asia/Rangoon",
	"GMT":       "Africa/Abidjan",
	"WET":       "Africa/Casablanca",
	"ACT":       "America/Eirunepe",
	"VUT":       "Pacific/Efate",
	"PWT":       "Pacific/Palau",
	"NT":        "America/St_Johns",
	"GET":       "Asia/Tbilisi",
	"GMT+9":     "Etc/GMT-9",
	"MUST":      "Indian/Mauritius",
	"PET":       "America/Lima",
	"SRET":      "Asia/Srednekolymsk",
	"ACWT":      "Australia/Eucla",
	"CKHST":     "Pacific/Rarotonga",
	"TOT":       "Pacific/Tongatapu",
	"EDT":       "America/Detroit",
	"TJT":       "Asia/Dushanbe",
	"BTT":       "Asia/Thimbu",
	"LHDT":      "Australia/LHI",
	"VUST":      "Pacific/Efate",
	"NCT":       "Pacific/Noumea",
	"PONT":      "Pacific/Pohnpei",
	"KGT":       "Asia/Bishkek",
	"BORTST":    "Asia/Kuching",
	"MAGT":      "Asia/Magadan",
	"SCT":       "Indian/Mahe",
	"MLAST":     "Asia/Kuala_Lumpur",
	"YEKST":     "Asia/Yekaterinburg",
	"ACWST":     "Australia/Eucla",
	"AWT":       "Antarctica/Casey",
	"KST":       "Asia/Seoul",
	"MALST":     "Asia/Singapore",
	"IRST":      "Asia/Tehran",
	"LHT":       "Australia/LHI",
	"ART":       "America/Argentina/Buenos_Aires",
	"AFT":       "Asia/Kabul",
	"QYZST":     "Asia/Qyzylorda",
	"UYT":       "America/Montevideo",
	"GMT-10":    "Etc/GMT+10",
	"VOLT":      "Europe/Volgograd",
	"ARST":      "America/Argentina/Buenos_Aires",
	"ADT":       "America/Barbados",
	"VLAT":      "Asia/Ust-Nera",
	"MSK":       "Europe/Kirov",
	"WSDT":      "Pacific/Apia",
	"ChST":      "Pacific/Guam",
	"EEST":      "Africa/Cairo",
	"CVT":       "Atlantic/Cape_Verde",
	"GMT+13":    "Etc/GMT-13",
	"GMT+8":     "Etc/GMT-8",
	"SAMT":      "Europe/Astrakhan",
	"GMT+04:00": "Europe/Saratov",
	"ACWDT":     "Australia/Eucla",
	"FNST":      "America/Noronha",
	"CHOT":      "Asia/Choibalsan",
	"HOVST":     "Asia/Hovd",
	"PKT":       "Asia/Karachi",
	"OMST":      "Asia/Omsk",
	"QYZT":      "Asia/Qyzylorda",
	"YEKT":      "Asia/Yekaterinburg",
	"SAST":      "Africa/Johannesburg",
	"PEST":      "America/Lima",
	"GMT-3":     "Etc/GMT+3",
	"AMST":      "America/Boa_Vista",
	"MAWT":      "Antarctica/Mawson",
	"GMT-12":    "Etc/GMT+12",
	"GMT+7":     "Etc/GMT-7",
	"MUT":       "Indian/Mauritius",
	"PYT":       "America/Asuncion",
	"BST":       "America/La_Paz",
	"BDT":       "Asia/Dacca",
	"GMT+4":     "Etc/GMT-4",
	"HADT":      "America/Adak",
	"NZST":      "Antarctica/McMurdo",
	"VOST":      "Antarctica/Vostok",
	"ICT":       "Asia/Bangkok",
	"YAKT":      "Asia/Chita",
	"AWDT":      "Australia/Perth",
	"EASST":     "Chile/EasterIsland",
	"ALMST":     "Asia/Almaty",
	"UZST":      "Asia/Samarkand",
	"WST":       "Pacific/Apia",
	"VET":       "America/Caracas",
	"MSD":       "Europe/Kirov",
	"WAT":       "Africa/Bangui",
	"ET":        "America/Atikokan",
	"EHDT":      "America/Santo_Domingo",
	"EAST":      "Chile/EasterIsland",
	"GMT+11":    "Etc/GMT-11",
	"GMT+12":    "Etc/GMT-12",
	"BOT":       "America/La_Paz",
	"BDST":      "Asia/Dacca",
	"ULAT":      "Asia/Ulaanbaatar",
	"UTC":       "Etc/UCT",
	"GMT+1":     "Etc/GMT-1",
	"AST":       "America/Anguilla",
	"AQTST":     "Asia/Aqtobe",
	"HKT":       "Asia/Hong_Kong",
	"JDT":       "Asia/Tokyo",
	"AZOST":     "Atlantic/Azores",
	"IOT":       "Indian/Chagos",
	"TAHT":      "Pacific/Tahiti",
	"HAST":      "America/Adak",
	"CST":       "America/Bahia_Banderas",
	"NDT":       "America/St_Johns",
	"UZT":       "Asia/Samarkand",
	"CHADT":     "NZ-CHAT",
	"EGST":      "America/Scoresbysund",
	"GST":       "Asia/Dubai",
	"GMT-6":     "Etc/GMT+6",
	"FJST":      "Pacific/Fiji",
	"KOST":      "Pacific/Kosrae",
	"COT":       "America/Bogota",
	"IRT":       "Iran",
	"MET":       "MET",
	"CHUT":      "Pacific/Chuuk",
	"UYST":      "America/Montevideo",
	"CLST":      "America/Santiago",
	"MIST":      "Antarctica/Macquarie",
	"KRAST":     "Asia/Krasnoyarsk",
	"AZOT":      "Atlantic/Azores",
	"GMT-5":     "Etc/GMT+5",
	"TFT":       "Indian/Kerguelen",
	"EAT":       "Africa/Addis_Ababa",
	"CHOST":     "Asia/Choibalsan",
	"NPT":       "Asia/Kathmandu",
	"CT":        "America/Bahia_Banderas",
	"SYOT":      "Antarctica/Syowa",
	"IDT":       "Asia/Jerusalem",
	"GMT+3":     "Etc/GMT-3",
	"EST":       "America/Atikokan",
	"SRT":       "America/Paramaribo",
	"NST":       "America/St_Johns",
	"GMT+10":    "Etc/GMT-10",
	"MEST":      "MET",
	"AEDT":      "Australia/ACT",
	"NFT":       "Pacific/Norfolk",
	"IRKT":      "Asia/Irkutsk",
	"PHT":       "Asia/Manila",
	"KDT":       "Asia/Seoul",
	"NRT":       "Pacific/Nauru",
	"CEST":      "Africa/Ceuta",
	"PT":        "America/Dawson",
	"CHAST":     "NZ-CHAT",
	"MART":      "Pacific/Marquesas",
	"NCST":      "Pacific/Noumea",
	"MT":        "America/Boise",
	"PMDT":      "America/Miquelon",
	"ACDT":      "Australia/Adelaide",
	"LINT":      "Pacific/Kiritimati",
	"IRKST":     "Asia/Irkutsk",
	"GDT":       "Pacific/Guam",
	"CET":       "Africa/Algiers",
	"WGT":       "America/Godthab",
	"TRT":       "Asia/Istanbul",
	"WEST":      "Africa/Casablanca",
	"AKST":      "America/Anchorage",
	"ORAT":      "Asia/Oral",
	"GMT+14":    "Etc/GMT-14",
	"GFT":       "America/Cayenne",
	"PST":       "America/Dawson",
	"PDT":       "America/Ensenada",
	"DAVT":      "Antarctica/Davis",
	"YAKST":     "Asia/Chita",
	"GMT-8":     "Etc/GMT+8",
	"WFT":       "Pacific/Wallis",
	"PYST":      "America/Asuncion",
	"MST":       "America/Boise",
	"NZDT":      "Antarctica/McMurdo",
	"GILT":      "Pacific/Tarawa",
	"AKT":       "America/Anchorage",
	"GYT":       "America/Guyana",
	"GMT-2":     "Etc/GMT+2",
	"CXT":       "Indian/Christmas",
	"SBT":       "Pacific/Guadalcanal",
	"NFDT":      "Pacific/Norfolk",
	"PGT":       "Pacific/Port_Moresby",
	"VLAST":     "Asia/Ust-Nera",
	"MHT":       "Kwajalein",
	"TVT":       "Pacific/Funafuti",
	"AMT":       "America/Boa_Vista",
	"ANAT":      "Asia/Anadyr",
	"TSD":       "Asia/Dushanbe",
	"PETT":      "Asia/Kamchatka",
	"SGT":       "Asia/Singapore",
	"FJT":       "Pacific/Fiji",
	"WIB":       "Asia/Jakarta",
	"KT":        "Asia/Seoul",
	"JST":       "Asia/Tokyo",
	"GAMT":      "Pacific/Gambier",
	"PMST":      "America/Miquelon",
	"MAGST":     "Asia/Magadan",
	"IRDT":      "Asia/Tehran",
	"GMT-7":     "Etc/GMT+7",
	"AKDT":      "America/Anchorage",
	"NUT":       "Pacific/Niue",
	"CDT":       "America/Bahia_Banderas",
	"WGST":      "America/Godthab",
	"FNT":       "America/Noronha",
	"ROTT":      "Antarctica/Palmer",
	"MYT":       "Asia/Kuala_Lumpur",
	"GALT":      "Pacific/Galapagos",
	"GMT-9":     "Etc/GMT+9",
}
