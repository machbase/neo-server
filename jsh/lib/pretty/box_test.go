package pretty_test

import (
	"testing"

	"github.com/machbase/neo-server/v8/jsh/test_engine"
)

func TestTable(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "Table_basic",
			Script: `
				const pretty = require('pretty');
				const tw = pretty.Table({style: 'light'});
				tw.appendHeader(['Name', 'Age']);
				tw.appendRow(tw.row('Alice', 30));
				tw.appendRow(tw.row('Bob', 25));
				console.println(tw.render());
			`,
			Output: []string{
				"┌────────┬───────┬─────┐",
				"│ ROWNUM │ NAME  │ AGE │",
				"├────────┼───────┼─────┤",
				"│      1 │ Alice │  30 │",
				"│      2 │ Bob   │  25 │",
				"└────────┴───────┴─────┘",
			},
		},
		{
			Name: "Table_with_floats",
			Script: `
				const pretty = require('pretty');
				const tw = pretty.Table({style: 'light', precision: 2});
				tw.appendHeader(['Item', 'Price']);
				tw.appendRow(tw.row('Apple', 1.234));
				tw.appendRow(tw.row('Orange', 2.567));
				console.println(tw.render());
			`,
			Output: []string{
				"┌────────┬────────┬───────┐",
				"│ ROWNUM │ ITEM   │ PRICE │",
				"├────────┼────────┼───────┤",
				"│      1 │ Apple  │  1.23 │",
				"│      2 │ Orange │  2.57 │",
				"└────────┴────────┴───────┘",
			},
		},
		{
			Name: "Table_styles",
			Script: `
				const pretty = require('pretty');
				const styles = ['light', 'double', 'bold', 'rounded', 'compact'];
				for (const style of styles) {
					const tw = pretty.Table({boxStyle: style, rownum: false});
					tw.appendHeader(['Col']);
					tw.appendRow(tw.row('Val'));
					console.println(style + ':');
					console.println(tw.render());
				}
			`,
			Output: []string{
				"light:",
				"┌─────┐",
				"│ COL │",
				"├─────┤",
				"│ Val │",
				"└─────┘",
				"double:",
				"╔═════╗",
				"║ COL ║",
				"╠═════╣",
				"║ Val ║",
				"╚═════╝",
				"bold:",
				"┏━━━━━┓",
				"┃ COL ┃",
				"┣━━━━━┫",
				"┃ Val ┃",
				"┗━━━━━┛",
				"rounded:",
				"╭─────╮",
				"│ COL │",
				"├─────┤",
				"│ Val │",
				"╰─────╯",
				"compact:",
				" COL ",
				"─────",
				" Val ",
			},
		},
		{
			Name: "Table_timeformat_datetime",
			Script: `
				const pretty = require('pretty');
				const tw = pretty.Table({style: 'light', timeformat: 'DATETIME', tz: 'UTC'});
				tw.appendHeader(['Event', 'Time']);
				tw.append(['Start', new Date('2024-03-15T14:30:45.000Z')]);
				tw.append(['End', new Date('2024-03-15T18:20:30.000Z')]);
				console.println(tw.render());
			`,
			Output: []string{
				"┌────────┬───────┬─────────────────────┐",
				"│ ROWNUM │ EVENT │ TIME                │",
				"├────────┼───────┼─────────────────────┤",
				"│      1 │ Start │ 2024-03-15 14:30:45 │",
				"│      2 │ End   │ 2024-03-15 18:20:30 │",
				"└────────┴───────┴─────────────────────┘",
			},
		},
		{
			Name: "Table_timeformat_date",
			Script: `
				const pretty = require('pretty');
				const tw = pretty.Table({style: 'light', rownum:true, timeformat: 'DATE', tz: 'UTC'});
				tw.appendHeader(['Event', 'Date']);
				tw.append(['Meeting', new Date('2024-03-15T00:00:00Z')]);
				tw.append(['Deadline', new Date('2024-12-31T00:00:00Z')]);
				console.println(tw.render());
			`,
			Output: []string{
				"┌────────┬──────────┬────────────┐",
				"│ ROWNUM │ EVENT    │ DATE       │",
				"├────────┼──────────┼────────────┤",
				"│      1 │ Meeting  │ 2024-03-15 │",
				"│      2 │ Deadline │ 2024-12-31 │",
				"└────────┴──────────┴────────────┘",
			},
		},
		{
			Name: "Table_timeformat_time",
			Script: `
				const pretty = require('pretty');
				const tw = pretty.Table({style: 'light', timeformat: 'TIME', tz: 'UTC'});
				tw.appendHeader(['Event', 'Time']);
				tw.append(['Start', new Date('2024-03-15T14:30:45Z')]);
				tw.append(['End', new Date('2024-03-15T18:20:30Z')]);
				console.println(tw.render());
			`,
			Output: []string{
				"┌────────┬───────┬──────────┐",
				"│ ROWNUM │ EVENT │ TIME     │",
				"├────────┼───────┼──────────┤",
				"│      1 │ Start │ 14:30:45 │",
				"│      2 │ End   │ 18:20:30 │",
				"└────────┴───────┴──────────┘",
			},
		},
		{
			Name: "Table_timeformat_rfc3339",
			Script: `
				const pretty = require('pretty');
				const tw = pretty.Table({style: 'light', timeformat: 'RFC3339', tz: 'UTC'});
				tw.appendHeader(['Event', 'Timestamp']);
				tw.append(['Created', new Date('2024-03-15T14:30:45.123Z')]);
				console.println(tw.render());
			`,
			Output: []string{
				"┌────────┬─────────┬──────────────────────────┐",
				"│ ROWNUM │ EVENT   │ TIMESTAMP                │",
				"├────────┼─────────┼──────────────────────────┤",
				"│      1 │ Created │ 2024-03-15T14:30:45.123Z │",
				"└────────┴─────────┴──────────────────────────┘",
			},
		},
		{
			Name: "Table_timeformat_rfc1123",
			Script: `
				const pretty = require('pretty');
				const tw = pretty.Table({style: 'light', timeformat: 'RFC1123', tz: 'UTC'});
				tw.appendHeader(['Event', 'Timestamp']);
				tw.append(['Notification', new Date('2024-03-15T14:30:45Z')]);
				console.println(tw.render());
			`,
			Output: []string{
				"┌────────┬──────────────┬───────────────────────────────┐",
				"│ ROWNUM │ EVENT        │ TIMESTAMP                     │",
				"├────────┼──────────────┼───────────────────────────────┤",
				"│      1 │ Notification │ Fri, 15 Mar 2024 14:30:45 UTC │",
				"└────────┴──────────────┴───────────────────────────────┘",
			},
		},
		{
			Name: "Table_timeformat_ansic",
			Script: `
				const pretty = require('pretty');
				const tw = pretty.Table({style: 'light', timeformat: 'ANSIC', tz: 'UTC'});
				tw.appendHeader(['Event', 'Timestamp']);
				tw.append(['Log', new Date('2024-03-15T14:30:45Z')]);
				console.println(tw.render());
			`,
			Output: []string{
				"┌────────┬───────┬──────────────────────────┐",
				"│ ROWNUM │ EVENT │ TIMESTAMP                │",
				"├────────┼───────┼──────────────────────────┤",
				"│      1 │ Log   │ Fri Mar 15 14:30:45 2024 │",
				"└────────┴───────┴──────────────────────────┘",
			},
		},
		{
			Name: "Table_timeformat_kitchen",
			Script: `
				const pretty = require('pretty');
				const tw = pretty.Table({style: 'light', timeformat: 'KITCHEN', tz: 'UTC'});
				tw.appendHeader(['Event', 'Time']);
				tw.append(['Lunch', new Date('2024-03-15T14:30:00Z')]);
				tw.append(['Dinner', new Date('2024-03-15T18:45:00Z')]);
				console.println(tw.render());
			`,
			Output: []string{
				"┌────────┬────────┬────────┐",
				"│ ROWNUM │ EVENT  │ TIME   │",
				"├────────┼────────┼────────┤",
				"│      1 │ Lunch  │ 2:30PM │",
				"│      2 │ Dinner │ 6:45PM │",
				"└────────┴────────┴────────┘",
			},
		},
		{
			Name: "Table_timeformat_stamp",
			Script: `
				const pretty = require('pretty');
				const tw = pretty.Table({style: 'light', timeformat: 'STAMP', tz: 'UTC'});
				tw.appendHeader(['Event', 'Timestamp']);
				tw.append(['Alert', new Date('2024-03-15T14:30:45Z')]);
				console.println(tw.render());
			`,
			Output: []string{
				"┌────────┬───────┬─────────────────┐",
				"│ ROWNUM │ EVENT │ TIMESTAMP       │",
				"├────────┼───────┼─────────────────┤",
				"│      1 │ Alert │ Mar 15 14:30:45 │",
				"└────────┴───────┴─────────────────┘",
			},
		},
		{
			Name: "Table_timeformat_stampmilli",
			Script: `
				const pretty = require('pretty');
				const tw = pretty.Table({style: 'light', timeformat: 'STAMPMILLI', tz: 'UTC'});
				tw.appendHeader(['Event', 'Timestamp']);
				tw.append(['Debug', new Date('2024-03-15T14:30:45.123Z')]);
				console.println(tw.render());
			`,
			Output: []string{
				"┌────────┬───────┬─────────────────────┐",
				"│ ROWNUM │ EVENT │ TIMESTAMP           │",
				"├────────┼───────┼─────────────────────┤",
				"│      1 │ Debug │ Mar 15 14:30:45.123 │",
				"└────────┴───────┴─────────────────────┘",
			},
		},
		{
			Name: "Table_timeformat_stampmicro",
			Script: `
				const pretty = require('pretty');
				const tw = pretty.Table({style: 'light', timeformat: 'STAMPMICRO', tz: 'UTC'});
				tw.appendHeader(['Event', 'Timestamp']);
				tw.append(['Trace', new Date('2024-03-15T14:30:45.123Z')]);
				console.println(tw.render());
			`,
			Output: []string{
				"┌────────┬───────┬────────────────────────┐",
				"│ ROWNUM │ EVENT │ TIMESTAMP              │",
				"├────────┼───────┼────────────────────────┤",
				"│      1 │ Trace │ Mar 15 14:30:45.123000 │",
				"└────────┴───────┴────────────────────────┘",
			},
		},
		{
			Name: "Table_timeformat_stampnano",
			Script: `
				const pretty = require('pretty');
				const tw = pretty.Table({style: 'light', timeformat: 'STAMPNANO', tz: 'UTC'});
				tw.appendHeader(['Event', 'Timestamp']);
				tw.append(['Precise', new Date('2024-03-15T14:30:45.123Z')]);
				console.println(tw.render());
			`,
			Output: []string{
				"┌────────┬─────────┬───────────────────────────┐",
				"│ ROWNUM │ EVENT   │ TIMESTAMP                 │",
				"├────────┼─────────┼───────────────────────────┤",
				"│      1 │ Precise │ Mar 15 14:30:45.123000000 │",
				"└────────┴─────────┴───────────────────────────┘",
			},
		},
	}
	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}

func TestMakeRow(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "MakeRow_basic",
			Script: `
				const pretty = require('pretty');
				const rows = pretty.MakeRow(3);
				console.println('length:', rows.length);
				console.println('is array:', Array.isArray(rows));
			`,
			Output: []string{
				"length: 3",
				"is array: true",
			},
		},
	}
	for _, tc := range tests {
		test_engine.RunTest(t, tc)
	}
}
