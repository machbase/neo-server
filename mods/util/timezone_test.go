package util_test

import (
	"testing"

	"github.com/machbase/neo-server/mods/util"
	"github.com/stretchr/testify/require"
)

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
