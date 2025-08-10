package observability

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/cocowh/muxcore/pkg/logger"
)

// ConsoleExporter 控制台导出器
type ConsoleExporter struct {
	name     string
	running  bool
	interval time.Duration
}

// NewConsoleExporter 创建控制台导出器
func NewConsoleExporter(name string, interval time.Duration) *ConsoleExporter {
	return &ConsoleExporter{
		name:     name,
		interval: interval,
	}
}

// Export 导出追踪数据
func (ce *ConsoleExporter) Export(traces []*Trace) error {
	if len(traces) == 0 {
		return nil
	}

	logger.Info(fmt.Sprintf("[%s] Exporting %d traces", ce.name, len(traces)))

	for _, trace := range traces {
		ce.exportTrace(trace)
	}

	return nil
}

// exportTrace 导出单个追踪
func (ce *ConsoleExporter) exportTrace(trace *Trace) {
	trace.mutex.RLock()
	defer trace.mutex.RUnlock()

	logger.Info(fmt.Sprintf("Trace ID: %s, Duration: %v, Spans: %d", 
		trace.TraceID, trace.Duration, len(trace.Spans)))

	for _, span := range trace.Spans {
		ce.exportSpan(span)
	}
}

// exportSpan 导出单个span
func (ce *ConsoleExporter) exportSpan(span *Span) {
	span.mutex.RLock()
	defer span.mutex.RUnlock()

	spanData := map[string]interface{}{
		"trace_id":       span.TraceID,
		"span_id":        span.SpanID,
		"parent_span_id": span.ParentSpanID,
		"operation_name": span.OperationName,
		"start_time":     span.StartTime.Format(time.RFC3339Nano),
		"duration":       span.Duration.String(),
		"status":         span.Status,
		"attributes":     span.Attributes,
		"events_count":   len(span.Events),
	}

	if span.Error != "" {
		spanData["error"] = span.Error
	}

	data, _ := json.MarshalIndent(spanData, "  ", "  ")
	logger.Info(fmt.Sprintf("  Span: %s", string(data)))
}

// Shutdown 关闭导出器
func (ce *ConsoleExporter) Shutdown() error {
	ce.running = false
	logger.Info(fmt.Sprintf("[%s] Console exporter shutdown", ce.name))
	return nil
}

// Start 启动导出器
func (ce *ConsoleExporter) Start() error {
	ce.running = true
	logger.Info(fmt.Sprintf("[%s] Console exporter started", ce.name))
	return nil
}

// IsRunning 检查是否运行中
func (ce *ConsoleExporter) IsRunning() bool {
	return ce.running
}