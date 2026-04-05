package vizspec_test

import (
	"testing"

	"github.com/machbase/neo-server/v8/jsh/test_engine"
)

func TestParseAndValidate(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "advn-parse-and-validate",
			Script: `
				const advn = require('vizspec');
				const spec = advn.parse(JSON.stringify({
					version: 1,
					series: [{
						id: 'cpu-overview',
						representation: {
							kind: 'time-bucket-band',
							fields: ['time', 'min', 'max', 'avg', 'count']
						}
					}]
				}));
				console.println(spec.version);
				console.println(spec.series[0].representation.kind);
				console.println(advn.validate(spec));
			`,
			Output: []string{"1", "time-bucket-band", "true"},
		},
		{
			Name: "advn-validate-invalid-style",
			Script: `
				const advn = require('vizspec');
				try {
					advn.validate({
						version: 1,
						series: [{
							id: 'cpu-overview',
							representation: { kind: 'time-bucket-band', fields: ['time', 'min', 'max', 'avg'] },
							style: { opacity: 3 }
						}]
					});
				} catch (err) {
					console.println(String(err).indexOf('opacity must be between 0 and 1') >= 0);
				}
			`,
			Output: []string{"true"},
		},
		{
			Name: "advn-parse-epoch-nanoseconds",
			Script: `
				const advn = require('vizspec');
				const spec = advn.parse(JSON.stringify({
					version: 1,
					domain: {
						kind: 'time',
						timeformat: 'ns',
						from: 1775174400000000000,
						to: 1775217600000000000
					},
					series: [{
						id: 'maintenance-window',
						representation: { kind: 'event-range', fields: ['from', 'to', 'label'] },
						data: [[1775210400000000000, 1775214000000000000, 'maintenance']]
					}]
				}));
				const option = advn.toEChartsOption(spec);
				console.println(typeof spec.domain.from);
				console.println(spec.domain.from);
				console.println(spec.series[0].data[0][0]);
				console.println(option.xAxis.type);
				console.println(String(option.series[0].markArea.data[0][0].xAxis).indexOf('T') >= 0);
			`,
			Output: []string{"string", "1775174400000000000", "1775210400000000000", "time", "true"},
		},
		{
			Name: "advn-echarts-output-time-options",
			Script: `
				const advn = require('vizspec');
				const option = advn.toEChartsOption(advn.createSpec({
					domain: {
						kind: 'time',
						timeformat: 'ns'
					},
					series: [{
						id: 'maintenance-window',
						representation: { kind: 'event-range', fields: ['from', 'to', 'label'] },
						data: [['1775210400000000000', '1775214000000000000', 'maintenance']]
					}]
				}), { timeformat: advn.Timeformat.rfc3339, tz: 'Asia/Seoul' });
				console.println(option.xAxis.type);
				console.println(option.series[0].markArea.data[0][0].xAxis);
			`,
			Output: []string{"time", "2026-04-03T19:00:00+09:00"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			test_engine.RunTest(t, tc)
		})
	}
}

func TestNormalizeAndStringify(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "advn-normalize-and-stringify",
			Script: `
				const advn = require('vizspec');
				const spec = advn.createSpec({
					series: [{
						id: 'sensor-1',
						representation: { kind: 'raw-point' }
					}]
				});
				console.println(spec.version);
				console.println(spec.series.length);
				console.println(advn.stringify(spec));
			`,
			Output: []string{
				"1",
				"1",
				`{"version":1,"series":[{"id":"sensor-1","representation":{"kind":"raw-point"}}]}`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			test_engine.RunTest(t, tc)
		})
	}
}

func TestBuilderAPI(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "advn-builder-api",
			Script: `
				const advn = require('vizspec');
				const spec = new advn.Builder()
					.domain({ kind: 'time', tz: 'UTC' })
					.xAxis({ type: 'time' })
					.addYAxis({ id: 'value', type: 'linear', unit: '°C' })
					.addSeries(advn.timeBucketBandSeries({
						id: 'sensor-1',
						axis: 'value',
						representation: { fields: ['time', 'min', 'max', 'avg', 'count'] }
					}))
					.addAnnotation(advn.rangeAnnotation({
						axis: 'x',
						from: '2026-04-03T10:00:00Z',
						to: '2026-04-03T11:00:00Z',
						label: 'maintenance'
					}))
					.view({ preferredRenderer: 'interactive-time-series' })
					.meta({ producer: 'jsh-analysis' })
					.build();

				console.println(spec.version);
				console.println(spec.domain.kind);
				console.println(spec.axes.y[0].id);
				console.println(spec.series[0].representation.kind);
				console.println(spec.annotations[0].kind);
				console.println(advn.validate(spec));
			`,
			Output: []string{"1", "time", "value", "time-bucket-band", "range", "true"},
		},
		{
			Name: "advn-helper-builders",
			Script: `
				const advn = require('vizspec');
				const raw = advn.rawPointSeries({
					id: 'raw-1',
					representation: { fields: ['time', 'value'] }
				});
				const hist = advn.distributionHistogramSeries({ id: 'hist-1' });
				const box = advn.distributionBoxplotSeries({ id: 'box-1' });
				const ann = advn.lineAnnotation({ axis: 'y', value: 42, label: 'threshold' });
				console.println(raw.representation.kind);
				console.println(raw.representation.fields[1]);
				console.println(hist.representation.fields[2]);
				console.println(box.representation.outlierFields[1]);
				console.println(ann.kind);
				console.println(ann.label);
			`,
			Output: []string{"raw-point", "value", "count", "value", "line", "threshold"},
		},
		{
			Name: "advn-builder-alias-and-echarts",
			Script: `
				const advn = require('vizspec');
				const option = new advn.Builder()
					.setDomain({ kind: 'time', tz: 'UTC' })
					.setXAxis({ id: 'time', type: 'time' })
					.addAxis({ id: 'value', type: 'linear', label: 'Temperature' })
					.addTimeBucketValueSeries({
						id: 'sensor-1',
						name: 'sensor-1',
						axis: 'value',
						data: [
							['2026-04-03T00:00:00Z', 19.8],
							['2026-04-03T00:01:00Z', 19.7]
						]
					})
					.addLineAnnotation({ axis: 'value', value: 25, label: 'warning' })
					.addRangeAnnotation({ axis: 'x', from: '2026-04-03T10:00:00Z', to: '2026-04-03T11:00:00Z', label: 'maintenance' })
					.setView({ preferredRenderer: 'interactive-time-series', defaultZoom: [0, 100] })
					.setMeta({ producer: 'jsh-analysis' })
					.toEChartsOption();

				console.println(option.xAxis.type);
				console.println(option.series.length);
				console.println(option.series[0].markLine.data[0].name);
				console.println(option.series[0].markArea.data.length);
				console.println(option.dataZoom[0].type);
			`,
			Output: []string{"time", "1", "warning", "1", "slider"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			test_engine.RunTest(t, tc)
		})
	}
}

func TestRenderers(t *testing.T) {
	tests := []test_engine.TestCase{
		{
			Name: "advn-list-series",
			Script: `
				const advn = require('vizspec');
				const spec = advn.createSpec({
					series: [
						advn.timeBucketBandSeries({ id: 'band-1', name: 'Band One' }),
						advn.distributionHistogramSeries({ id: 'hist-1' })
					]
				});
				const listed = advn.listSeries(spec);
				const builderListed = new advn.Builder()
					.addTimeBucketBandSeries({ id: 'band-1', name: 'Band One' })
					.addDistributionHistogramSeries({ id: 'hist-1' })
					.listSeries();
				console.println(listed.length);
				console.println(listed[0].id);
				console.println(listed[0].title);
				console.println(listed[0].tuiLinesCompatible);
				console.println(listed[1].tuiLinesCompatible);
				console.println(builderListed[0].kind);
			`,
			Output: []string{"2", "band-1", "Band One", "true", "false", "time-bucket-band"},
		},
		{
			Name: "advn-echarts-histogram-and-boxplot",
			Script: `
				const advn = require('vizspec');
				const histOption = advn.toEChartsOption(advn.createSpec({
					series: [advn.distributionHistogramSeries({
						id: 'hist-1',
						style: { color: '#ff8800', opacity: 0.6 },
						representation: { fields: ['binStart', 'binEnd', 'count'] },
						data: [
							[0, 10, 3],
							[10, 20, 8]
						]
					})]
				}));
				const boxOption = advn.toEChartsOption(advn.createSpec({
					series: [advn.distributionBoxplotSeries({
						id: 'box-1',
						representation: { fields: ['category', 'low', 'q1', 'median', 'q3', 'high'] },
						data: [
							['A', 1, 2, 3, 4, 5],
							['B', 2, 3, 4, 5, 6]
						],
						extra: { outliers: [['A', 7]] }
					})]
				}));
				console.println(histOption.xAxis.type);
				console.println(histOption.series[0].type);
				console.println(histOption.series[0].itemStyle.color);
				console.println(boxOption.xAxis.type);
				console.println(boxOption.series[0].type);
				console.println(boxOption.series[1].type);
			`,
			Output: []string{"category", "bar", "#ff8800", "category", "boxplot", "scatter"},
		},
		{
			Name: "advn-echarts-event-series",
			Script: `
				const advn = require('vizspec');
				const option = advn.toEChartsOption(advn.createSpec({
					domain: { kind: 'time' },
					series: [
						advn.eventPointSeries({
							id: 'event-point-1',
							name: 'alerts',
							style: { color: '#ff3300', opacity: 0.9 },
							data: [['2026-04-03T10:15:00Z', 93, 'threshold exceeded', 'warn']]
						}),
						advn.eventRangeSeries({
							id: 'event-range-1',
							name: 'maintenance',
							style: { color: '#ffcc00', opacity: 0.3 },
							data: [['2026-04-03T10:00:00Z', '2026-04-03T11:00:00Z', 'maintenance']]
						})
					]
				}));
				console.println(option.series[0].type);
				console.println(option.series[0].itemStyle.color);
				console.println(option.series[1].type);
				console.println(option.series[1].markArea.data.length);
			`,
			Output: []string{"scatter", "#ff3300", "line", "1"},
		},
		{
			Name: "advn-band-style-echarts",
			Script: `
				const advn = require('vizspec');
				const option = advn.toEChartsOption(advn.createSpec({
					series: [advn.timeBucketBandSeries({
						id: 'band-1',
						name: 'band-1',
						style: { color: '#3366cc', bandColor: '#99bbff', lineWidth: 2, opacity: 0.25 },
						representation: { fields: ['time', 'min', 'max', 'avg'] },
						data: [
							['2026-04-03T00:00:00Z', 1, 5, 3],
							['2026-04-03T00:01:00Z', 2, 6, 4]
						]
					})]
				}));
				console.println(option.series.length);
				console.println(option.series[1].areaStyle.color);
				console.println(option.series[1].areaStyle.opacity);
				console.println(option.series[2].lineStyle.color);
				console.println(option.series[2].lineStyle.width);
			`,
			Output: []string{"3", "#99bbff", "0.25", "#3366cc", "2"},
		},
		{
			Name: "advn-tui-blocks",
			Script: `
				const advn = require('vizspec');
				const blocks = new advn.Builder()
					.setDomain({ kind: 'time', from: '2026-04-03T00:00:00Z', to: '2026-04-03T01:00:00Z' })
					.addTimeBucketBandSeries({
						id: 'sensor-1',
						name: 'sensor-1',
						data: [
							['2026-04-03T00:00:00Z', 1, 5, 3],
							['2026-04-03T00:10:00Z', 2, 6, 4],
							['2026-04-03T00:20:00Z', 1, 4, 2]
						]
					})
					.addAnnotation(advn.lineAnnotation({ axis: 'x', value: '2026-04-03T00:30:00Z', label: 'checkpoint' }))
					.toTUIBlocks();
				console.println(blocks[0].type);
				console.println(blocks[1].type);
				console.println(blocks[2].type);
				console.println(blocks[2].lines.length);
				console.println(blocks[3].type);
				console.println(blocks[4].type);
			`,
			Output: []string{"summary", "series-summary", "bandline", "3", "table", "annotations"},
		},
		{
			Name: "advn-tui-histogram-and-event-range",
			Script: `
				const advn = require('vizspec');
				const blocks = advn.toTUIBlocks(advn.createSpec({
					domain: { kind: 'time', from: '2026-04-03T00:00:00Z', to: '2026-04-03T12:00:00Z' },
					series: [
						advn.distributionHistogramSeries({
							id: 'hist-1',
							data: [[0, 10, 3], [10, 20, 8]]
						}),
						advn.eventRangeSeries({
							id: 'maintenance-1',
							data: [['2026-04-03T10:00:00Z', '2026-04-03T11:00:00Z', 'maintenance']]
						})
					]
				}));
				console.println(blocks[2].type);
				console.println(blocks[2].lines[0].indexOf('0-10') >= 0);
				console.println(blocks[5].type);
				console.println(blocks[5].lines[0].indexOf('=') >= 0);
			`,
			Output: []string{"bars", "true", "timeline", "true"},
		},
		{
			Name: "advn-tui-options",
			Script: `
				const advn = require('vizspec');
				const data = [];
				for (let i = 0; i < 12; i++) {
					data.push([i, i]);
				}
				const spec = advn.createSpec({
					series: [advn.rawPointSeries({
						id: 'raw-1',
						representation: { fields: ['x', 'y'] },
						data,
					})]
				});
				const blocks = advn.toTUIBlocks(spec, { width: 5, rows: 2 });
				const compactBlocks = advn.toTUIBlocks(spec, { width: 5, rows: 2, compact: true });
				console.println(blocks[2].type);
				console.println(blocks[2].lines.length);
				console.println(blocks[2].lines[0].indexOf('┤') >= 0);
				console.println(blocks[3].rows.length);
				console.println(compactBlocks.length);
			`,
			Output: []string{"sparkline", "1", "false", "2", "2"},
		},
		{
			Name: "advn-tui-lines-output",
			Script: `
				const advn = require('vizspec');
				const spec = advn.createSpec({
					domain: { kind: 'time', from: '2026-04-03T00:00:00Z', to: '2026-04-03T00:04:00Z' },
					series: [
						advn.timeBucketValueSeries({
							id: 'value-1',
							data: [
								['2026-04-03T00:00:00Z', 1],
								['2026-04-03T00:02:00Z', 4],
								['2026-04-03T00:04:00Z', 2]
							]
						}),
						advn.timeBucketValueSeries({
							id: 'value-2',
							data: [
								['2026-04-03T00:00:00Z', 8],
								['2026-04-03T00:02:00Z', 2],
								['2026-04-03T00:04:00Z', 7]
							]
						})
					]
				});
				const lines = advn.toTUILines(spec, { width: 12 });
				const tallLines = advn.toTUILines(spec, { width: 12, height: 5 });
				const selectedLines = advn.toTUILines(spec, { width: 12, seriesId: 'value-2' });
				const builderLines = new advn.Builder()
					.setDomain({ kind: 'time', from: '2026-04-03T00:00:00Z', to: '2026-04-03T00:04:00Z' })
					.addTimeBucketValueSeries({
						id: 'value-1',
						data: [
							['2026-04-03T00:00:00Z', 1],
							['2026-04-03T00:02:00Z', 4],
							['2026-04-03T00:04:00Z', 2]
						]
					})
					.addTimeBucketValueSeries({
						id: 'value-2',
						data: [
							['2026-04-03T00:00:00Z', 8],
							['2026-04-03T00:02:00Z', 2],
							['2026-04-03T00:04:00Z', 7]
						]
					})
					.toTUILines({ width: 12, seriesId: 'value-2' });
				console.println(lines.length);
				console.println(lines[1].indexOf('┤') >= 0);
				console.println(lines[0].indexOf(':') >= 0);
				console.println(tallLines.length);
				console.println(lines.join('\n') !== selectedLines.join('\n'));
				console.println(selectedLines.join('\n') === builderLines.join('\n'));
				console.println(builderLines.length);
			`,
			Output: []string{"4", "true", "true", "6", "true", "true", "4"},
		},
		{
			Name: "advn-svg-output",
			Script: `
				const advn = require('vizspec');
				const svg = new advn.Builder()
					.setDomain({
						kind: 'time',
						timeformat: advn.Timeformat.ns,
						from: '1712102400000000000',
						to: '1712102520000000000'
					})
					.setXAxis({ id: 'time', type: 'time', label: 'Time' })
					.addYAxis({ id: 'value', type: 'linear', label: 'Value' })
					.addTimeBucketBandSeries({
						id: 'sensor-band',
						name: 'sensor-band',
						axis: 'value',
						style: { color: '#3366cc', bandColor: '#99bbff', opacity: 0.22, lineWidth: 2 },
						data: [
							['1712102400000000000', 10, 14, 12],
							['1712102460000000000', 11, 15, 13],
							['1712102520000000000', 9, 16, 12.5]
						]
					})
					.addLineAnnotation({ axis: 'value', value: 13.5, label: 'threshold' })
					.toSVG({ title: 'ADVN SVG', showLegend: true, width: 640, height: 320, timeformat: advn.Timeformat.rfc3339, tz: 'UTC' });
				console.println(svg.indexOf('<svg ') >= 0);
				console.println(svg.indexOf('data-advn-role="series"') >= 0);
				console.println(svg.indexOf('ADVN SVG') >= 0);
				console.println(svg.indexOf('00:00') >= 0 || svg.indexOf('00:01') >= 0);
				console.println(svg.indexOf('1712102400000000000') < 0);
			`,
			Output: []string{"true", "true", "true", "true", "true"},
		},
		{
			Name: "advn-png-output",
			Script: `
				const advn = require('vizspec');
				const png = new advn.Builder()
					.setDomain({ kind: 'time', tz: 'UTC' })
					.setXAxis({ id: 'time', type: 'time', label: 'Time' })
					.addYAxis({ id: 'value', type: 'linear', label: 'Value' })
					.addTimeBucketValueSeries({
						id: 'series-1',
						name: 'series-1',
						axis: 'value',
						data: [
							['2026-04-03T00:00:00Z', 10],
							['2026-04-03T00:01:00Z', 12]
						]
					})
					.toPNG({ title: 'ADVN PNG', showLegend: true, width: 480, height: 220 });
				const view = new Uint8Array(png);
				const header = Array.prototype.map.call(view.slice(0, 8), value => value.toString(16).padStart(2, '0')).join('');
				console.println(header);
				console.println(png.byteLength > 64);
			`,
			Output: []string{"89504e470d0a1a0a", "true"},
		},
		{
			Name: "advn-png-combined-options-and-legacy",
			Script: `
				const advn = require('vizspec');
				const spec = new advn.Builder()
					.setDomain({ kind: 'time', tz: 'UTC' })
					.setXAxis({ id: 'time', type: 'time', label: 'Time' })
					.addYAxis({ id: 'value', type: 'linear', label: 'Value' })
					.addTimeBucketValueSeries({
						id: 'series-1',
						name: 'series-1',
						axis: 'value',
						data: [
							['2026-04-03T00:00:00Z', 10],
							['2026-04-03T00:01:00Z', 12]
						]
					})
					.build();
				const combined = advn.toPNG(spec, {
					title: 'ADVN PNG',
					width: 480,
					height: 220,
					background: '#ffffff',
					scale: 2,
					theme: 'mrtg'
				});
				const legacy = advn.toPNG(spec,
					{ title: 'ADVN PNG', width: 480, height: 220, background: '#ffffff' },
					{ scale: 2, theme: 'mrtg', background: '#ffffff' }
				);
				console.println(new Uint8Array(combined)[0].toString(16).padStart(2, '0'));
				console.println(combined.byteLength > 64);
				console.println(new Uint8Array(legacy)[0].toString(16).padStart(2, '0'));
				console.println(legacy.byteLength > 64);
			`,
			Output: []string{"89", "true", "89", "true"},
		},
		{
			Name: "advn-tui-output-time-options",
			Script: `
				const advn = require('vizspec');
				const blocks = advn.toTUIBlocks(advn.createSpec({
					domain: { kind: 'time', timeformat: 'ns', from: '1775174400000000000', to: '1775174460000000000' },
					series: [{
						id: 'value-1',
						representation: { kind: 'time-bucket-value', fields: ['time', 'value'] },
						data: [['1775174400000000000', 1]]
					}]
				}), { timeformat: advn.Timeformat.rfc3339, tz: 'Asia/Seoul' });
				console.println(blocks[0].lines[0].indexOf('+09:00') >= 0);
				console.println(blocks[3].rows[0][0]);
			`,
			Output: []string{"true", "2026-04-03T09:00:00+09:00"},
		},
		{
			Name: "advn-svg-options-validation",
			Script: `
				const advn = require('vizspec');
				try {
					advn.toSVG(advn.createSpec({ version: 1 }), { width: -1 });
				} catch (err) {
					console.println(String(err).indexOf('width must be greater than 0') >= 0);
				}
			`,
			Output: []string{"true"},
		},
		{
			Name: "advn-invalid-representation-fields",
			Script: `
				const advn = require('vizspec');
				try {
					advn.validate({
						version: 1,
						series: [{
							id: 'hist-1',
							representation: { kind: 'distribution-histogram', fields: ['binStart', 'count'] }
						}]
					});
				} catch (err) {
					console.println(String(err).indexOf('requires field "binEnd"') >= 0);
				}
			`,
			Output: []string{"true"},
		},
		{
			Name: "advn-invalid-axis-reference",
			Script: `
				const advn = require('vizspec');
				try {
					advn.validate({
						version: 1,
						axes: { y: [{ id: 'value', type: 'linear' }] },
						series: [{
							id: 'series-1',
							axis: 'missing',
							representation: { kind: 'time-bucket-value', fields: ['time', 'value'] }
						}]
					});
				} catch (err) {
					console.println(String(err).indexOf('axis "missing" is not defined') >= 0);
				}
			`,
			Output: []string{"true"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			test_engine.RunTest(t, tc)
		})
	}
}
