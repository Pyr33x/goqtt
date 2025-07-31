package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
)

// LogLevel represents logging levels
type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
)

// Logger wraps slog.Logger with MQTT-specific functionality
type Logger struct {
	*slog.Logger
	level     LogLevel
	component string
}

// Config holds logger configuration
type Config struct {
	Level       LogLevel
	Format      string // "json" or "text"
	Output      io.Writer
	Component   string
	ShowCaller  bool
	AddSource   bool
	TimeFormat  string
	Environment string
	Service     string
	Version     string
}

var (
	globalLogger *Logger
	mu           sync.RWMutex
)

// New creates a new logger with the given configuration
func New(config Config) *Logger {
	var handler slog.Handler

	opts := &slog.HandlerOptions{
		Level:     convertLevel(config.Level),
		AddSource: config.AddSource,
	}

	if config.Output == nil {
		config.Output = os.Stdout
	}

	switch strings.ToLower(config.Format) {
	case "json":
		handler = slog.NewJSONHandler(config.Output, opts)
	default:
		handler = slog.NewTextHandler(config.Output, opts)
	}

	// Add default fields if configured
	if config.Environment != "" || config.Service != "" || config.Version != "" {
		attrs := make([]slog.Attr, 0, 3)
		if config.Service != "" {
			attrs = append(attrs, slog.String("service", config.Service))
		}
		if config.Version != "" {
			attrs = append(attrs, slog.String("version", config.Version))
		}
		if config.Environment != "" {
			attrs = append(attrs, slog.String("environment", config.Environment))
		}
		handler = handler.WithAttrs(attrs)
	}

	if config.Component != "" {
		handler = handler.WithGroup(config.Component)
	}

	return &Logger{
		Logger:    slog.New(handler),
		level:     config.Level,
		component: config.Component,
	}
}

// InitGlobalLogger initializes the global logger
func InitGlobalLogger(config Config) {
	mu.Lock()
	defer mu.Unlock()
	globalLogger = New(config)
}

// GetGlobalLogger returns the global logger
func GetGlobalLogger() *Logger {
	mu.RLock()
	defer mu.RUnlock()
	if globalLogger == nil {
		// Initialize with default config
		globalLogger = New(DevelopmentConfig())
	}
	return globalLogger
}

// NewMQTTLogger creates a component-specific logger for MQTT operations
func NewMQTTLogger(component string) *Logger {
	global := GetGlobalLogger()

	// Create a new logger with component group
	handler := global.Handler().WithGroup(component)

	return &Logger{
		Logger:    slog.New(handler),
		level:     global.level,
		component: component,
	}
}

// DevelopmentConfig returns a development-friendly configuration
func DevelopmentConfig() Config {
	return Config{
		Level:       LevelDebug,
		Format:      "text",
		Output:      os.Stdout,
		ShowCaller:  true,
		AddSource:   true,
		Service:     "goqtt",
		Version:     "dev",
		Environment: "development",
	}
}

// ProductionConfig returns a production-ready configuration
func ProductionConfig() Config {
	return Config{
		Level:       LevelInfo,
		Format:      "json",
		Output:      os.Stdout,
		ShowCaller:  false,
		AddSource:   false,
		Service:     "goqtt",
		Environment: "production",
	}
}

// Helper methods for structured logging with MQTT context

// LogClientConnection logs client connection events
func (l *Logger) LogClientConnection(clientID, remoteAddr string, action string, attrs ...slog.Attr) {
	baseAttrs := []slog.Attr{
		slog.String("client_id", clientID),
		slog.String("remote_addr", remoteAddr),
		slog.String("action", action),
	}
	baseAttrs = append(baseAttrs, attrs...)

	l.LogAttrs(context.Background(), slog.LevelInfo, "Client connection event", baseAttrs...)
}

// LogMQTTPacket logs MQTT packet information
func (l *Logger) LogMQTTPacket(packetType string, clientID string, direction string, attrs ...slog.Attr) {
	baseAttrs := []slog.Attr{
		slog.String("packet_type", packetType),
		slog.String("client_id", clientID),
		slog.String("direction", direction), // "inbound" or "outbound"
	}
	baseAttrs = append(baseAttrs, attrs...)

	l.LogAttrs(context.Background(), slog.LevelDebug, "MQTT packet", baseAttrs...)
}

// LogPublish logs PUBLISH packet details
func (l *Logger) LogPublish(clientID, topic string, qos int, retain bool, payloadSize int, attrs ...slog.Attr) {
	baseAttrs := []slog.Attr{
		slog.String("client_id", clientID),
		slog.String("topic", topic),
		slog.Int("qos", qos),
		slog.Bool("retain", retain),
		slog.Int("payload_size", payloadSize),
	}
	baseAttrs = append(baseAttrs, attrs...)

	l.LogAttrs(context.Background(), slog.LevelInfo, "Message published", baseAttrs...)
}

// LogSubscription logs subscription events
func (l *Logger) LogSubscription(clientID, topic string, qos int, action string, attrs ...slog.Attr) {
	baseAttrs := []slog.Attr{
		slog.String("client_id", clientID),
		slog.String("topic_filter", topic),
		slog.Int("qos", qos),
		slog.String("action", action), // "subscribe", "unsubscribe"
	}
	baseAttrs = append(baseAttrs, attrs...)

	l.LogAttrs(context.Background(), slog.LevelInfo, "Subscription event", baseAttrs...)
}

// LogError logs error with context
func (l *Logger) LogError(err error, message string, attrs ...slog.Attr) {
	if err != nil {
		attrs = append(attrs, slog.String("error", err.Error()))
	}
	l.LogAttrs(context.Background(), slog.LevelError, message, attrs...)
}

// LogAuth logs authentication events
func (l *Logger) LogAuth(clientID, username string, success bool, reason string, attrs ...slog.Attr) {
	baseAttrs := []slog.Attr{
		slog.String("client_id", clientID),
		slog.String("username", username),
		slog.Bool("success", success),
		slog.String("reason", reason),
	}
	baseAttrs = append(baseAttrs, attrs...)

	level := slog.LevelInfo
	if !success {
		level = slog.LevelWarn
	}

	l.LogAttrs(context.Background(), level, "Authentication attempt", baseAttrs...)
}

// LogQoSFlow logs QoS flow control events
func (l *Logger) LogQoSFlow(clientID string, packetID uint16, qos int, step string, attrs ...slog.Attr) {
	baseAttrs := []slog.Attr{
		slog.String("client_id", clientID),
		slog.Int("packet_id", int(packetID)),
		slog.Int("qos", qos),
		slog.String("step", step), // e.g., "PUBACK_SENT", "PUBREC_RECEIVED"
	}
	baseAttrs = append(baseAttrs, attrs...)

	l.LogAttrs(context.Background(), slog.LevelDebug, "QoS flow control", baseAttrs...)
}

// LogRetainedMessage logs retained message operations
func (l *Logger) LogRetainedMessage(topic string, action string, payloadSize int, attrs ...slog.Attr) {
	baseAttrs := []slog.Attr{
		slog.String("topic", topic),
		slog.String("action", action), // "stored", "removed", "delivered"
		slog.Int("payload_size", payloadSize),
	}
	baseAttrs = append(baseAttrs, attrs...)

	l.LogAttrs(context.Background(), slog.LevelDebug, "Retained message operation", baseAttrs...)
}

// LogPerformance logs performance metrics
func (l *Logger) LogPerformance(metric string, value any, unit string, attrs ...slog.Attr) {
	baseAttrs := []slog.Attr{
		slog.String("metric", metric),
		slog.Any("value", value),
		slog.String("unit", unit),
	}
	baseAttrs = append(baseAttrs, attrs...)

	l.LogAttrs(context.Background(), slog.LevelInfo, "Performance metric", baseAttrs...)
}

// Convenience methods

// Debug logs a debug message
func (l *Logger) Debug(msg string, attrs ...slog.Attr) {
	l.LogAttrs(context.Background(), slog.LevelDebug, msg, attrs...)
}

// Info logs an info message
func (l *Logger) Info(msg string, attrs ...slog.Attr) {
	l.LogAttrs(context.Background(), slog.LevelInfo, msg, attrs...)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, attrs ...slog.Attr) {
	l.LogAttrs(context.Background(), slog.LevelWarn, msg, attrs...)
}

// Error logs an error message
func (l *Logger) Error(msg string, attrs ...slog.Attr) {
	l.LogAttrs(context.Background(), slog.LevelError, msg, attrs...)
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(msg string, attrs ...slog.Attr) {
	l.LogAttrs(context.Background(), slog.LevelError, msg, attrs...)
	os.Exit(1)
}

// With returns a new logger with the given attributes
func (l *Logger) With(attrs ...slog.Attr) *Logger {
	return &Logger{
		Logger:    l.Logger.With(attrsToAny(attrs)...),
		level:     l.level,
		component: l.component,
	}
}

// WithGroup returns a new logger with the given group
func (l *Logger) WithGroup(name string) *Logger {
	return &Logger{
		Logger:    l.Logger.WithGroup(name),
		level:     l.level,
		component: l.component,
	}
}

// Helper functions

func convertLevel(level LogLevel) slog.Level {
	switch level {
	case LevelDebug:
		return slog.LevelDebug
	case LevelInfo:
		return slog.LevelInfo
	case LevelWarn:
		return slog.LevelWarn
	case LevelError:
		return slog.LevelError
	case LevelFatal:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func attrsToAny(attrs []slog.Attr) []any {
	result := make([]any, len(attrs))
	for i, attr := range attrs {
		result[i] = attr
	}
	return result
}

// Global convenience functions for backward compatibility

// Debug logs a debug message using the global logger
func Debug(msg string, attrs ...slog.Attr) {
	GetGlobalLogger().Debug(msg, attrs...)
}

// Info logs an info message using the global logger
func Info(msg string, attrs ...slog.Attr) {
	GetGlobalLogger().Info(msg, attrs...)
}

// Warn logs a warning message using the global logger
func Warn(msg string, attrs ...slog.Attr) {
	GetGlobalLogger().Warn(msg, attrs...)
}

// Error logs an error message using the global logger
func Error(msg string, attrs ...slog.Attr) {
	GetGlobalLogger().Error(msg, attrs...)
}

// Fatal logs a fatal message using the global logger and exits
func Fatal(msg string, attrs ...slog.Attr) {
	GetGlobalLogger().Fatal(msg, attrs...)
}

// Convenience functions for creating common attributes

// ClientID creates a client_id attribute
func ClientID(clientID string) slog.Attr {
	return slog.String("client_id", clientID)
}

// String creates a string attribute
func String(key, value string) slog.Attr {
	return slog.String(key, value)
}

// Int creates an int attribute
func Int(key string, value int) slog.Attr {
	return slog.Int(key, value)
}

// Bool creates a bool attribute
func Bool(key string, value bool) slog.Attr {
	return slog.Bool(key, value)
}

// Any creates an attribute with any value
func Any(key string, value any) slog.Attr {
	return slog.Any(key, value)
}

// ErrorAttr creates an error attribute
func ErrorAttr(err error) slog.Attr {
	return slog.String("error", err.Error())
}
