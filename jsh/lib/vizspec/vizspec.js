'use strict';

const _vizspec = require('@jsh/vizspec');

const RepresentationKind = {
    rawPoint: 'raw-point',
    timeBucketValue: 'time-bucket-value',
    timeBucketBand: 'time-bucket-band',
    distributionHistogram: 'distribution-histogram',
    distributionBoxplot: 'distribution-boxplot',
    eventPoint: 'event-point',
    eventRange: 'event-range',
};

const AnnotationKind = {
    point: 'point',
    line: 'line',
    range: 'range',
};

const Timeformat = {
    rfc3339: 'rfc3339',
    s: 's',
    ms: 'ms',
    us: 'us',
    ns: 'ns',
};

function isObject(value) {
    return value !== null && typeof value === 'object' && !Array.isArray(value);
}

function ensureObjectInput(name, value) {
    if (!isObject(value)) {
        throw new Error(`${name}: value must be an object`);
    }
}

function ensureNonEmptyString(name, value) {
    if (typeof value !== 'string' || value.length === 0) {
        throw new Error(`${name}: value must be a non-empty string`);
    }
}

function cloneObject(value) {
    return { ...value };
}

function representation(kind, init = {}) {
    ensureNonEmptyString('vizspec.representation', kind);
    ensureObjectInput('vizspec.representation', init);
    const ret = cloneObject(init);
    ret.kind = kind;
    return ret;
}

function domain(init = {}) {
    ensureObjectInput('vizspec.domain', init);
    return cloneObject(init);
}

function axis(init = {}) {
    ensureObjectInput('vizspec.axis', init);
    return cloneObject(init);
}

function series(init = {}) {
    ensureObjectInput('vizspec.series', init);
    const ret = cloneObject(init);
    if (isObject(ret.representation)) {
        ret.representation = cloneObject(ret.representation);
    }
    if (isObject(ret.source)) {
        ret.source = cloneObject(ret.source);
    }
    if (isObject(ret.quality)) {
        ret.quality = cloneObject(ret.quality);
    }
    if (isObject(ret.style)) {
        ret.style = cloneObject(ret.style);
    }
    if (isObject(ret.extra)) {
        ret.extra = cloneObject(ret.extra);
    }
    return ret;
}

function annotation(init = {}) {
    ensureObjectInput('vizspec.annotation', init);
    const ret = cloneObject(init);
    if (isObject(ret.style)) {
        ret.style = cloneObject(ret.style);
    }
    return ret;
}

function view(init = {}) {
    ensureObjectInput('vizspec.view', init);
    return cloneObject(init);
}

function meta(init = {}) {
    ensureObjectInput('vizspec.meta', init);
    return cloneObject(init);
}

function seriesWithKind(kind, init = {}) {
    const ret = series(init);
    const representationInit = isObject(ret.representation) ? ret.representation : {};
    ret.representation = representation(kind, representationInit);
    if (!Array.isArray(ret.representation.fields) || ret.representation.fields.length === 0) {
        ret.representation.fields = defaultFieldsForKind(kind);
    }
    if (kind === RepresentationKind.distributionBoxplot) {
        if (!Array.isArray(ret.representation.outlierFields) || ret.representation.outlierFields.length === 0) {
            ret.representation.outlierFields = ['category', 'value'];
        }
    }
    return ret;
}

function defaultFieldsForKind(kind) {
    switch (kind) {
        case RepresentationKind.rawPoint:
            return ['x', 'y'];
        case RepresentationKind.timeBucketValue:
            return ['time', 'value'];
        case RepresentationKind.timeBucketBand:
            return ['time', 'min', 'max', 'avg'];
        case RepresentationKind.distributionHistogram:
            return ['binStart', 'binEnd', 'count'];
        case RepresentationKind.distributionBoxplot:
            return ['category', 'low', 'q1', 'median', 'q3', 'high'];
        case RepresentationKind.eventPoint:
            return ['time', 'value', 'label', 'severity'];
        case RepresentationKind.eventRange:
            return ['from', 'to', 'label'];
        default:
            return [];
    }
}

function annotationWithKind(kind, init = {}) {
    const ret = annotation(init);
    ret.kind = kind;
    return ret;
}

function parse(text) {
    if (typeof text !== 'string') {
        throw new Error('vizspec.parse: text must be a string');
    }
    return _vizspec.parse(text);
}

function stringify(spec) {
    ensureObjectInput('vizspec.stringify', spec);
    return _vizspec.stringify(spec);
}

function toEChartsOption(spec, options = undefined) {
    ensureObjectInput('vizspec.toEChartsOption', spec);
    if (options !== undefined) {
        ensureObjectInput('vizspec.toEChartsOption', options);
        return _vizspec.toEChartsOption(spec, options);
    }
    return _vizspec.toEChartsOption(spec);
}

function toTUIBlocks(spec, options = undefined) {
    ensureObjectInput('vizspec.toTUIBlocks', spec);
    if (options !== undefined) {
        ensureObjectInput('vizspec.toTUIBlocks', options);
        return _vizspec.toTUIBlocks(spec, options);
    }
    return _vizspec.toTUIBlocks(spec);
}

function toSparkline(spec, options = undefined) {
    ensureObjectInput('vizspec.toSparkline', spec);
    if (options !== undefined) {
        ensureObjectInput('vizspec.toSparkline', options);
        return _vizspec.toSparkline(spec, options);
    }
    return _vizspec.toSparkline(spec);
}

function toSVG(spec, options = undefined) {
    ensureObjectInput('vizspec.toSVG', spec);
    if (options !== undefined) {
        ensureObjectInput('vizspec.toSVG', options);
        return _vizspec.toSVG(spec, options);
    }
    return _vizspec.toSVG(spec);
}

function toPNG(spec, svgOptions = undefined, pngOptions = undefined) {
    ensureObjectInput('vizspec.toPNG', spec);
    if (svgOptions !== undefined) {
        ensureObjectInput('vizspec.toPNG', svgOptions);
    }
    if (pngOptions !== undefined) {
        ensureObjectInput('vizspec.toPNG', pngOptions);
    }
    if (svgOptions !== undefined && pngOptions !== undefined) {
        return _vizspec.toPNG(spec, svgOptions, pngOptions);
    }
    if (svgOptions !== undefined) {
        return _vizspec.toPNG(spec, svgOptions);
    }
    return _vizspec.toPNG(spec);
}

function validate(spec) {
    ensureObjectInput('vizspec.validate', spec);
    return _vizspec.validate(spec);
}

function normalize(spec = {}) {
    ensureObjectInput('vizspec.normalize', spec);
    return _vizspec.normalize(spec);
}

function createSpec(init = {}) {
    ensureObjectInput('vizspec.createSpec', init);
    return _vizspec.createSpec(init);
}

function rawPointSeries(init = {}) {
    return seriesWithKind(RepresentationKind.rawPoint, init);
}

function timeBucketValueSeries(init = {}) {
    return seriesWithKind(RepresentationKind.timeBucketValue, init);
}

function timeBucketBandSeries(init = {}) {
    return seriesWithKind(RepresentationKind.timeBucketBand, init);
}

function distributionHistogramSeries(init = {}) {
    return seriesWithKind(RepresentationKind.distributionHistogram, init);
}

function distributionBoxplotSeries(init = {}) {
    return seriesWithKind(RepresentationKind.distributionBoxplot, init);
}

function eventPointSeries(init = {}) {
    return seriesWithKind(RepresentationKind.eventPoint, init);
}

function eventRangeSeries(init = {}) {
    return seriesWithKind(RepresentationKind.eventRange, init);
}

function pointAnnotation(init = {}) {
    return annotationWithKind(AnnotationKind.point, init);
}

function lineAnnotation(init = {}) {
    return annotationWithKind(AnnotationKind.line, init);
}

function rangeAnnotation(init = {}) {
    return annotationWithKind(AnnotationKind.range, init);
}

class Builder {
    constructor(init = {}) {
        ensureObjectInput('vizspec.Builder', init);
        this._spec = createSpec(init);
        this._built = undefined;
    }

    _markDirty() {
        this._built = undefined;
        return this;
    }

    _getBuiltSpec() {
        if (this._built === undefined) {
            this._built = normalize(this._spec);
        }
        return this._built;
    }

    domain(definition = {}) {
        this._spec.domain = {
            ...(this._spec.domain || {}),
            ...domain(definition),
        };
        return this._markDirty();
    }

    setDomain(definition = {}) {
        return this.domain(definition);
    }

    xAxis(definition = {}) {
        this._spec.axes = this._spec.axes || {};
        this._spec.axes.x = {
            ...(this._spec.axes.x || {}),
            ...axis(definition),
        };
        return this._markDirty();
    }

    setXAxis(definition = {}) {
        return this.xAxis(definition);
    }

    addYAxis(definition = {}) {
        this._spec.axes = this._spec.axes || {};
        this._spec.axes.y = this._spec.axes.y || [];
        this._spec.axes.y.push(axis(definition));
        return this._markDirty();
    }

    addAxis(definition = {}) {
        return this.addYAxis(definition);
    }

    addSeries(definition = {}) {
        this._spec.series = this._spec.series || [];
        this._spec.series.push(series(definition));
        return this._markDirty();
    }

    addRawPointSeries(definition = {}) {
        return this.addSeries(rawPointSeries(definition));
    }

    addTimeBucketValueSeries(definition = {}) {
        return this.addSeries(timeBucketValueSeries(definition));
    }

    addTimeBucketBandSeries(definition = {}) {
        return this.addSeries(timeBucketBandSeries(definition));
    }

    addDistributionHistogramSeries(definition = {}) {
        return this.addSeries(distributionHistogramSeries(definition));
    }

    addDistributionBoxplotSeries(definition = {}) {
        return this.addSeries(distributionBoxplotSeries(definition));
    }

    addEventPointSeries(definition = {}) {
        return this.addSeries(eventPointSeries(definition));
    }

    addEventRangeSeries(definition = {}) {
        return this.addSeries(eventRangeSeries(definition));
    }

    addAnnotation(definition = {}) {
        this._spec.annotations = this._spec.annotations || [];
        this._spec.annotations.push(annotation(definition));
        return this._markDirty();
    }

    addPointAnnotation(definition = {}) {
        return this.addAnnotation(pointAnnotation(definition));
    }

    addLineAnnotation(definition = {}) {
        return this.addAnnotation(lineAnnotation(definition));
    }

    addRangeAnnotation(definition = {}) {
        return this.addAnnotation(rangeAnnotation(definition));
    }

    view(definition = {}) {
        this._spec.view = {
            ...(this._spec.view || {}),
            ...view(definition),
        };
        return this._markDirty();
    }

    setView(definition = {}) {
        return this.view(definition);
    }

    meta(definition = {}) {
        this._spec.meta = {
            ...(this._spec.meta || {}),
            ...meta(definition),
        };
        return this._markDirty();
    }

    setMeta(definition = {}) {
        return this.meta(definition);
    }

    build() {
        return normalize(this._spec);
    }

    stringify() {
        return stringify(this._getBuiltSpec());
    }

    toEChartsOption(options = undefined) {
        return toEChartsOption(this._getBuiltSpec(), options);
    }

    toTUIBlocks(options = undefined) {
        return toTUIBlocks(this._getBuiltSpec(), options);
    }

    toSparkline(options = undefined) {
        return toSparkline(this._getBuiltSpec(), options);
    }

    toSVG(options = undefined) {
        return toSVG(this._getBuiltSpec(), options);
    }

    toPNG(svgOptions = undefined, pngOptions = undefined) {
        return toPNG(this._getBuiltSpec(), svgOptions, pngOptions);
    }
}

module.exports = {
    AnnotationKind,
    Builder,
    RepresentationKind,
    Timeformat,
    annotation,
    axis,
    createSpec,
    distributionBoxplotSeries,
    distributionHistogramSeries,
    domain,
    eventPointSeries,
    eventRangeSeries,
    lineAnnotation,
    meta,
    normalize,
    parse,
    pointAnnotation,
    rangeAnnotation,
    rawPointSeries,
    representation,
    series,
    stringify,
    timeBucketBandSeries,
    timeBucketValueSeries,
    toEChartsOption,
    toPNG,
    toSparkline,
    toSVG,
    toTUIBlocks,
    validate,
    view,
};
