package bot

import (
	"github.com/rguedesdev/TG-FileStreamBot/config"
	"github.com/rguedesdev/TG-FileStreamBot/internal/commands"
	"context"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/celestix/gotgproto"
	"github.com/celestix/gotgproto/sessionMaker"
	"github.com/glebarez/sqlite"
)

var Bot *gotgproto.Client

const mainClientStartupTimeout = 120 * time.Second

var telegramDiagnosticMessages = map[string]struct{}{
	"Starting":                           {},
	"Connection restored from state":     {},
	"Run: start":                         {},
	"Run: end":                           {},
	"Generating new auth key":            {},
	"Initializing new key exchange":      {},
	"Sending ReqPqMultiRequest":          {},
	"Received server ResPQ":              {},
	"PQ decomposing complete":            {},
	"Sending ReqDHParamsRequest":         {},
	"Received server ServerDHParams":     {},
	"Sending SetClientDHParamsRequest":   {},
	"Received server DhGen":              {},
	"Auth key generated":                 {},
	"Key already exists":                 {},
	"Initializing":                       {},
	"SessionInit":                        {},
	"Ready":                              {},
	"Connection dead":                    {},
	"Restarting connection":              {},
	"Retrying connection initialization": {},
	"Failed to cleanup connection":       {},
	"Re-generating keys (server not found key that we provided)": {},
	"Re-created auth keys": {},
}

var telegramDiagnosticFields = map[string]struct{}{
	"addr":             {},
	"backoff":          {},
	"conn_id":          {},
	"context_deadline": {},
	"dc_id":            {},
	"duration":         {},
	"error":            {},
	"reason":           {},
	"timeout":          {},
	"took":             {},
	"v":                {},
}

type telegramDiagnosticCore struct {
	base zapcore.Core
}

func (c *telegramDiagnosticCore) Enabled(level zapcore.Level) bool {
	return c.base.Enabled(level)
}

func (c *telegramDiagnosticCore) With(fields []zapcore.Field) zapcore.Core {
	return &telegramDiagnosticCore{base: c.base.With(safeTelegramDiagnosticFields(fields))}
}

func (c *telegramDiagnosticCore) Check(entry zapcore.Entry, checked *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if _, allowed := telegramDiagnosticMessages[entry.Message]; allowed && c.Enabled(entry.Level) {
		return checked.AddCore(entry, c)
	}
	return checked
}

func (c *telegramDiagnosticCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	return c.base.Write(entry, safeTelegramDiagnosticFields(fields))
}

func (c *telegramDiagnosticCore) Sync() error {
	return c.base.Sync()
}

func safeTelegramDiagnosticFields(fields []zapcore.Field) []zapcore.Field {
	safe := make([]zapcore.Field, 0, len(fields))
	for _, field := range fields {
		if _, allowed := telegramDiagnosticFields[field.Key]; !allowed {
			continue
		}
		if field.Key == "error" || field.Key == "reason" || field.Key == "addr" {
			safe = append(safe, zap.String(field.Key, safeTelegramDiagnosticValue(field)))
			continue
		}
		safe = append(safe, field)
	}
	return safe
}

func safeTelegramDiagnosticValue(field zapcore.Field) string {
	value := field.String
	if err, ok := field.Interface.(error); ok {
		value = err.Error()
	}
	for _, secret := range []string{config.ValueOf.BotToken, config.ValueOf.ApiHash} {
		if secret != "" {
			value = strings.ReplaceAll(value, secret, "[REDACTED]")
		}
	}
	return value
}

func telegramConnectionLogger(log *zap.Logger) *zap.Logger {
	if !config.ValueOf.Dev {
		return nil
	}
	return zap.New(&telegramDiagnosticCore{base: log.Core()}).Named("TelegramTransport")
}

func StartClient(log *zap.Logger) (*gotgproto.Client, error) {
	if err := os.MkdirAll("sessions", 0o700); err != nil {
		return nil, err
	}
	startupCtx, cancel := context.WithTimeout(context.Background(), mainClientStartupTimeout)
	defer cancel()

	var startupStage atomic.Value
	startupStage.Store("opening the SQLite session")
	log.Info("Starting main Telegram client", zap.Duration("startup_timeout", mainClientStartupTimeout))

	resultChan := make(chan struct {
		client *gotgproto.Client
		err    error
	}, 1)
	go func() {
		client, err := gotgproto.NewClient(
			int(config.ValueOf.ApiID),
			config.ValueOf.ApiHash,
			gotgproto.ClientTypeBot(config.ValueOf.BotToken),
			&gotgproto.ClientOpts{
				Logger: telegramConnectionLogger(log),
				Session: sessionMaker.SqlSession(
					sqlite.Open("sessions/fsb.session"),
				),
				DisableCopyright: true,
				RunMiddleware: func(
					run func(context.Context, func(context.Context) error) error,
					runtimeCtx context.Context,
					initialize func(context.Context) error,
				) error {
					startupStage.Store("waiting for MTProto transport readiness")
					log.Info("Main Telegram session opened; waiting for MTProto transport readiness")
					return run(runtimeCtx, func(ctx context.Context) error {
						startupStage.Store("authenticating the bot or loading its Telegram identity")
						log.Info("Main Telegram transport ready; authenticating bot and loading identity")
						return initialize(ctx)
					})
				},
			},
		)
		resultChan <- struct {
			client *gotgproto.Client
			err    error
		}{client, err}
	}()

	select {
	case <-startupCtx.Done():
		return nil, fmt.Errorf(
			"main Telegram client did not become ready within %s while %s: %w",
			mainClientStartupTimeout,
			startupStage.Load().(string),
			startupCtx.Err(),
		)
	case result := <-resultChan:
		if result.err != nil {
			return nil, result.err
		}
		commands.Load(log, result.client.Dispatcher)
		log.Info("Client started", zap.String("username", result.client.Self.Username))
		Bot = result.client
		return result.client, nil
	}
}
