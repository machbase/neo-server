# TQL Runtime Design

## Purpose

TQL executes a statement pipeline as a stream of `Record` values. The runtime is designed around one channel ownership rule:

> The stage that writes an output channel creates and closes that channel.

No downstream stage, sink, or `Task` closes an upstream output channel.

## Main Types

### Task

`Task` owns compile-time metadata and the public execution API.

- It parses and validates a script into a `compiledPlan`.
- It keeps `Compile()` metadata-only: no reusable runtime node graph, sink encoder, or database sink is retained after compile.
- It instantiates a fresh node graph for each allowed execution and prepares the sink after an execution runtime exists.
- It starts the first source, connects map stages, starts the sink, and waits for completion.
- It rejects concurrent calls to `Execute()` on the same task.
- Preemptive cache refreshes use a cloned task and an independent runtime graph.

`Task` must not close a node input or output channel.

`Task` is a single-use execution object. A second `Execute()` call is rejected because a TQL pipeline can contain non-idempotent side effects such as database writes, HTTP requests, scripts, or shell commands. Use an explicit clone to execute the same immutable plan again with a fresh runtime graph.

### compiledPlan

`compiledPlan` is immutable execution metadata.

- Each `compiledNode` stores statement text, source location, pragmas, and statement kind.
- `instantiatePlan()` creates source/map runtime nodes and a sink output shell from the plan.
- The sink expression is evaluated later by `output.prepare(runtime)`, after the fresh execution runtime is available.
- Normal execution and preemptive cache execution use the same factory.
- Each stage has a resolved `StageRole`: `StageSource`, `StageMap`, or `StageSink`.
- Plans are cached by source hash in a bounded in-memory FIFO cache, but every execution instantiates a new runtime graph.

### execution and executionRuntime

`execution` owns mutable lifecycle state for one run:

- child context and cancel function
- directional stop state
- stop listeners
- first terminal error

`executionRuntime` binds that state to an immutable configuration snapshot:

- params, input reader, output writer, and argument values
- console, logging, HTTP client, and volatile asset configuration
- result columns

Nodes and sinks use their bound runtime. They must not re-read `Task.currentExecution()` while running.

## Pipeline Lifecycle

The runtime creates the pipeline in this order:

```text
source.RunSource() -> map.Run(input) -> ... -> sink.Consume(input)
```

1. `Execute()` creates a fresh execution runtime.
2. The sink output shell prepares its encoder or database sink against that runtime.
3. The first node runs as a source with `RunSource()`.
4. Every remaining node runs with `Run(input <-chan *Record)`.
5. The terminal output runs with `Consume(input <-chan *Record)`.
6. `Task` waits for the sink and every node to finish.

`Task.ExplainPlan()` returns immutable copies of compiled stage role, source line, statement text, pragma metadata, and configured output buffer capacity.

### Source Nodes

The first statement has `node.role = StageSource` and runs without a synthetic input record. This distinguishes a first-stage `SQL()` query from `SQL()` used as a map or sink statement.

Source nodes evaluate their statement once, emit zero or more records, run their finalize callback, and close their own output channel.

### Map Nodes

Map nodes read until one of these conditions occurs:

- upstream output closes: normal completion
- runtime context is canceled: task cancellation or terminal failure
- directional stop applies to this node: upstream early stop

On normal completion, a node runs `finalizeCallback` before closing its output. Stateful operations such as `GROUP`, `TIMEWINDOW`, filters, and `SCRIPT` use this hook to flush final records.

### Sink

The sink consumes the final read-only stream until it closes. Its expression is evaluated at execution time, after the runtime configuration snapshot exists. It opens encoders or database sinks lazily when the first record arrives, finalizes them once, and never closes its input channel.

`DatabaseSink.Open()` receives `*executionRuntime`, not `*Task`.

HTTP callers must not rely on output metadata immediately after `Compile()`. Content type, content encoding, chart type, and custom headers are execution-time metadata. Streaming HTTP handlers should set headers immediately before the first response write, after sink preparation has completed.

## Channel Rules

- `Node.RunSource()` and `Node.Run()` own `node.output` and close it exactly once.
- `output.Consume()` only reads its input.
- A consumer never closes the channel it reads.
- `Node.emit()` is the only normal record emission path. It observes runtime cancellation and directional stop before blocking on output.
- `Record` is data. It does not carry EOF or cancellation sentinels.

## Cancellation and Early Stop

### External Cancellation

`Task.Cancel()` cancels the run context and fires a global stop signal. Context-aware operations, including database queries, HTTP requests, JavaScript runtimes, and channel sends, must stop promptly.

### Directional Stop

`TAKE` uses directional stop rather than canceling the whole run.

- The `TAKE` node records itself as the stop origin.
- Source and upstream map nodes stop producing.
- Downstream map nodes and the sink keep draining until upstream output closes.

This preserves records emitted before the limit was reached while avoiding unnecessary upstream work.

## Records and Errors

`Record` represents data tuples, arrays, bytes, images, or stream-local errors.

- Normal completion is represented by channel close, not a record.
- `ErrorRecord` remains a stream value for formatter compatibility.
- Encoder open/write failures and node or sink panics are terminal execution errors.
- `execution` keeps the first terminal error; `Task.Execute()` returns it when present.

Runtime code should use explicit error-policy helpers:

- `EmitError(stageIndex, err)` for stream-local error records.
- `ReportError(stageIndex, err)` for stage telemetry plus terminal error reporting.
- `Fail(err)` for terminal failure with cancellation and global stop propagation.
- `Warn(...)` for log-only warnings.

## Result Columns

`executionRuntime` owns the current result-column snapshot. Nodes that change shape, such as sources, map functions, grouping, or transpose operations, update runtime columns. The sink uses runtime columns to configure encoders.

After a foreground execution completes, its final runtime columns are published as read-only Task metadata. Runtime code must use `node.ensureRuntime().SetResultColumns()` and `ResultColumns()`; `Task` does not own mutable execution columns.

Schema transitions through `SchemaUnknown`, `SchemaKnown`, and `SchemaFinal`. The sink finalizes the schema when it opens its header. Later schema mutations are strict terminal failures.

## Output Metadata

Output metadata is an execution result, not compile-time state.

- `Result.OutputMetadata` contains the completed output content type, content encoding, chart type, and custom headers.
- `Task.LastOutputMetadata()` returns the latest foreground execution metadata snapshot.
- `Task.OutputContentType()`, `OutputContentEncoding()`, `OutputChartType()`, and `OutputHttpHeaders()` use the active prepared sink when present, then fall back to the last completed metadata snapshot.

## Backpressure and Metrics

Output channels are unbuffered by default. A statement pragma can set a bounded output buffer for the following stage:

```text
#pragma pipeline-buffer=32
```

Bounded buffers do not change ownership or cancellation rules. `Task.LastStageStats()` exposes completed foreground stage index, role, input count, output count, queue capacity, queue high-water mark, blocked send duration, error count, last error, stop-origin marker, and elapsed time.

## Resources

Nodes may register `Closer` values for context-bound resources.

- `closeResources()` snapshots registered closers under lock.
- Resources close in reverse registration order.
- Each closer closes exactly once.
- A closer registered after node shutdown is closed immediately.

## Developer Rules

When adding or modifying a TQL statement implementation:

1. Emit records with `node.emit(record)`; do not create a new downstream channel or close a channel you do not own.
2. Use `node.ensureRuntime()` for context, params, result columns, logging, HTTP clients, console identity, and cancellation-sensitive configuration.
3. Put first-stage behavior in source-compatible logic. Do not rely on a synthetic initial `Record`.
4. Use `SetFinalize()` for end-of-stream flushing.
5. Register resource handles with `AddCloser()` when they must be released on cancellation or completion.
6. Preserve the distinction between stream `ErrorRecord` values and terminal execution errors.

## Verification

The TQL test suite includes full, race, and lifecycle stress coverage. Before changing lifecycle behavior, run:

```sh
cd neo-server
go test ./mods/tql/...
go test -race ./mods/tql/...
go test -count=100 -run 'TestPipelineLifecycleRepeated|TestTakeStopsUpstreamAndDrainsDownstream|TestSCRIPT_interrupt' ./mods/tql/...
```